//! Language detection, trust-scored parsing, and comment extraction for
//! source files touched by `Write`/`Edit`. This module only extracts and
//! classifies comments; lint rules that judge them live in a later
//! iteration.
//!
//! Line numbers on [`Comment`] and [`CommentBlock`] are 0-indexed, matching
//! `tree_sitter::Point::row`.

use std::ops::Range;
use std::path::Path;
use std::sync::OnceLock;

use regex::Regex;
use tree_sitter::{Language as TsLanguage, Node, Parser, Tree};

/// A source language this module knows how to parse and extract comments
/// from. `Svelte` is handled by re-parsing its embedded `<script>` content
/// with the TypeScript grammar; the outer Svelte markup is not otherwise
/// mined for comments.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Language {
    Rust,
    TypeScript,
    Tsx,
    JavaScript,
    Go,
    Python,
    C,
    Cpp,
    Java,
    CSharp,
    Bash,
    Css,
    Kotlin,
    Svelte,
    Sql,
}

impl Language {
    /// Detect a language from a file path's extension. An unrecognized or
    /// missing extension yields `None`, meaning callers should skip the
    /// file entirely rather than guess.
    pub fn from_path(path: &Path) -> Option<Self> {
        let ext = path.extension()?.to_str()?;
        Self::from_extension(ext)
    }

    fn from_extension(ext: &str) -> Option<Self> {
        Some(match ext {
            "rs" => Self::Rust,
            "ts" => Self::TypeScript,
            "tsx" => Self::Tsx,
            "js" | "jsx" | "mjs" | "cjs" => Self::JavaScript,
            "go" => Self::Go,
            "py" => Self::Python,
            "c" | "h" => Self::C,
            "cpp" | "cc" | "hpp" => Self::Cpp,
            "java" => Self::Java,
            "cs" => Self::CSharp,
            "sh" | "bash" => Self::Bash,
            "css" => Self::Css,
            "kt" | "kts" => Self::Kotlin,
            "svelte" => Self::Svelte,
            "sql" => Self::Sql,
            _ => return None,
        })
    }

    /// Detect a language from `path`'s extension, content-sniffing `.h`
    /// files: a `.h` extension is ambiguous between C and C++, so it is
    /// only trusted as C when `source` shows no clear C++ marker.
    fn detect(path: &Path, source: &str) -> Option<Self> {
        let ext = path.extension()?.to_str()?;
        let language = Self::from_extension(ext)?;
        if ext == "h" && language == Self::C && looks_like_cpp(source) {
            return Some(Self::Cpp);
        }
        Some(language)
    }

    fn ts_language(self) -> TsLanguage {
        match self {
            Self::Rust => tree_sitter_rust::LANGUAGE.into(),
            Self::TypeScript => tree_sitter_typescript::LANGUAGE_TYPESCRIPT.into(),
            Self::Tsx => tree_sitter_typescript::LANGUAGE_TSX.into(),
            Self::JavaScript => tree_sitter_javascript::LANGUAGE.into(),
            Self::Go => tree_sitter_go::LANGUAGE.into(),
            Self::Python => tree_sitter_python::LANGUAGE.into(),
            Self::C => tree_sitter_c::LANGUAGE.into(),
            Self::Cpp => tree_sitter_cpp::LANGUAGE.into(),
            Self::Java => tree_sitter_java::LANGUAGE.into(),
            Self::CSharp => tree_sitter_c_sharp::LANGUAGE.into(),
            Self::Bash => tree_sitter_bash::LANGUAGE.into(),
            Self::Css => tree_sitter_css::LANGUAGE.into(),
            Self::Kotlin => tree_sitter_kotlin_ng::LANGUAGE.into(),
            Self::Svelte => tree_sitter_svelte_ng::LANGUAGE.into(),
            Self::Sql => tree_sitter_sequel::LANGUAGE.into(),
        }
    }
}

/// Standard headers that only exist in C++, never in C. Their presence in a
/// `.h` file is decisive evidence the file is really C++.
const CPP_ONLY_INCLUDES: &[&str] = &[
    "memory",
    "vector",
    "string",
    "utility",
    "type_traits",
    "optional",
    "variant",
    "functional",
    "algorithm",
    "map",
    "set",
    "unordered_map",
    "unordered_set",
    "tuple",
    "array",
    "sstream",
    "iostream",
];

/// Strip block and line comments so a C++ marker mentioned only in prose
/// does not trigger a false-positive sniff.
fn strip_comments(source: &str) -> String {
    static BLOCK: OnceLock<Regex> = OnceLock::new();
    static LINE: OnceLock<Regex> = OnceLock::new();
    let block =
        BLOCK.get_or_init(|| Regex::new(r"(?s)/\*.*?\*/").expect("valid block-comment pattern"));
    let line =
        LINE.get_or_init(|| Regex::new(r"(?m)//[^\n]*").expect("valid line-comment pattern"));
    line.replace_all(&block.replace_all(source, ""), "")
        .into_owned()
}

fn cpp_marker_patterns() -> &'static [Regex] {
    static PATTERNS: OnceLock<Vec<Regex>> = OnceLock::new();
    PATTERNS.get_or_init(|| {
        let sources = [
            r#"extern\s*"C\+\+""#.to_string(),
            r"\bnamespace\s+\w+".to_string(),
            r"\btemplate\s*<".to_string(),
            r"\bclass\s+\w+".to_string(),
            r"\w::\w".to_string(),
            format!(r"(?m)^\s*#\s*include\s*<({})>", CPP_ONLY_INCLUDES.join("|")),
        ];
        sources
            .iter()
            .map(|src| Regex::new(src).expect("valid c++ marker pattern"))
            .collect()
    })
}

/// Whether `source` (the contents of a `.h` file) shows a clear C++ marker:
/// a C++-only standard header, `namespace`/`template`/`class` declarations,
/// `::` scope resolution, or `extern "C++"`. Comments are stripped first so
/// a marker mentioned only in prose does not count.
fn looks_like_cpp(source: &str) -> bool {
    let code = strip_comments(source);
    cpp_marker_patterns()
        .iter()
        .any(|pattern| pattern.is_match(&code))
}

/// How much a parse can be trusted. Abstaining (`Untrusted`) is a correct
/// outcome for a badly broken file, not a failure - callers must treat it
/// as "did not look", distinct from "looked and found nothing".
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ParseQuality {
    Clean,
    Degraded,
    Untrusted,
}

/// A parse is untrusted once error/missing byte spans exceed this fraction
/// of the file.
const UNTRUSTED_ERROR_BYTE_RATIO: f64 = 0.05;

/// A parse is untrusted once it contains more synthesized (missing) nodes
/// than this, regardless of the error byte ratio.
const UNTRUSTED_MISSING_NODE_LIMIT: usize = 5;

/// Byte-span and count totals backing a [`ParseQuality`] verdict.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct ParseStats {
    pub error_bytes: usize,
    pub missing_nodes: usize,
}

/// Score how trustworthy a parse is. Walks only into subtrees where
/// `has_error()` is true, since it is an O(1) precomputed flag that lets
/// clean subtrees be skipped entirely.
pub fn classify_quality(root: Node, source_len: usize) -> (ParseQuality, ParseStats) {
    if !root.has_error() {
        return (ParseQuality::Clean, ParseStats::default());
    }
    let mut spans = Vec::new();
    let mut missing_nodes = 0usize;
    accumulate_errors(root, &mut spans, &mut missing_nodes);
    let error_bytes = merged_span_bytes(&mut spans);
    let ratio = if source_len == 0 {
        0.0
    } else {
        error_bytes as f64 / source_len as f64
    };
    let quality =
        if ratio > UNTRUSTED_ERROR_BYTE_RATIO || missing_nodes > UNTRUSTED_MISSING_NODE_LIMIT {
            ParseQuality::Untrusted
        } else {
            ParseQuality::Degraded
        };
    (
        quality,
        ParseStats {
            error_bytes,
            missing_nodes,
        },
    )
}

fn accumulate_errors(node: Node, spans: &mut Vec<Range<usize>>, missing_nodes: &mut usize) {
    if !node.has_error() {
        return;
    }
    if node.is_error() {
        spans.push(node.byte_range());
    }
    if node.is_missing() {
        *missing_nodes += 1;
        spans.push(node.byte_range());
    }
    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        accumulate_errors(child, spans, missing_nodes);
    }
}

/// Total bytes covered by `spans` after merging overlapping/nested ranges,
/// so a node inside an already-counted `ERROR` span is not counted twice.
fn merged_span_bytes(spans: &mut [Range<usize>]) -> usize {
    if spans.is_empty() {
        return 0;
    }
    spans.sort_by_key(|r| r.start);
    let mut total = 0usize;
    let mut current = spans[0].clone();
    for span in &spans[1..] {
        if span.start <= current.end {
            current.end = current.end.max(span.end);
        } else {
            total += current.end - current.start;
            current = span.clone();
        }
    }
    total + (current.end - current.start)
}

/// Whether a comment sits doc-adjacent (`Standalone`) or shares a line with
/// preceding code (`Trailing`).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Placement {
    Standalone,
    Trailing,
}

fn placement(source: &str, start_byte: usize) -> Placement {
    let line_start = source[..start_byte].rfind('\n').map_or(0, |i| i + 1);
    let prefix = &source[line_start..start_byte];
    if prefix.chars().all(char::is_whitespace) {
        Placement::Standalone
    } else {
        Placement::Trailing
    }
}

/// Whether a comment reads as documentation (`///`, `/** */`, a Rust `doc`
/// field, or a Go comment adjacent to the declaration it precedes) versus
/// an ordinary comment.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CommentKind {
    Doc,
    Plain,
}

/// A single extracted comment node.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Comment {
    pub byte_range: Range<usize>,
    pub start_line: usize,
    pub end_line: usize,
    pub text: String,
    pub kind: CommentKind,
    pub placement: Placement,
}

/// One or more consecutive standalone single-line comments with no blank
/// line between them, treated as a single logical unit.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CommentBlock {
    pub byte_range: Range<usize>,
    pub start_line: usize,
    pub end_line: usize,
    pub kind: CommentKind,
    pub placement: Placement,
    pub comments: Vec<Comment>,
}

fn is_doc_by_prefix(text: &str) -> bool {
    let trimmed = text.trim_start();
    trimmed.starts_with("/**") || trimmed.starts_with("///")
}

fn is_rust_doc(node: Node) -> bool {
    node.child_by_field_name("doc").is_some()
}

/// Whether `text` cannot participate in adjacent-line merging: either it is
/// a block-style comment (`/* ... */`, possibly one line) or it already
/// spans multiple lines by itself.
fn is_atomic_multiline(text: &str) -> bool {
    text.starts_with("/*") || text.contains('\n')
}

fn make_comment(node: Node, source: &str, kind: CommentKind) -> Comment {
    let byte_range = node.byte_range();
    Comment {
        text: source[byte_range.clone()].to_string(),
        start_line: node.start_position().row,
        end_line: node.end_position().row,
        placement: placement(source, byte_range.start),
        kind,
        byte_range,
    }
}

fn collect_nodes_of_kind<'tree>(root: Node<'tree>, kinds: &[&str], out: &mut Vec<Node<'tree>>) {
    if kinds.contains(&root.kind()) {
        out.push(root);
        return;
    }
    let mut cursor = root.walk();
    for child in root.children(&mut cursor) {
        collect_nodes_of_kind(child, kinds, out);
    }
}

/// Extract comments matching `kinds`, classifying each with `classify`.
fn extract_with(
    tree: &Tree,
    source: &str,
    kinds: &[&str],
    classify: impl Fn(Node, &str) -> CommentKind,
) -> Vec<Comment> {
    let mut nodes = Vec::new();
    collect_nodes_of_kind(tree.root_node(), kinds, &mut nodes);
    nodes
        .into_iter()
        .map(|node| {
            let text = &source[node.byte_range()];
            let kind = classify(node, text);
            make_comment(node, source, kind)
        })
        .collect()
}

fn classify_by_prefix(_node: Node, text: &str) -> CommentKind {
    if is_doc_by_prefix(text) {
        CommentKind::Doc
    } else {
        CommentKind::Plain
    }
}

/// Node kinds carrying comment text for each language. `Svelte` returns an
/// empty slice: its comments come only from re-parsing embedded `<script>`
/// content.
fn comment_node_kinds(language: Language) -> &'static [&'static str] {
    match language {
        Language::Rust | Language::Java | Language::Kotlin => &["line_comment", "block_comment"],
        Language::Go
        | Language::Python
        | Language::C
        | Language::Cpp
        | Language::CSharp
        | Language::Bash
        | Language::Css
        | Language::Sql => &["comment"],
        Language::TypeScript | Language::Tsx | Language::JavaScript => &["comment", "html_comment"],
        Language::Svelte => &[],
    }
}

/// Whether `prev` and `cur` (in source order) merge into one logical block:
/// both standalone, neither an atomic multi-line comment, and line-adjacent
/// with no blank line between them.
fn mergeable(prev: &Comment, cur: &Comment) -> bool {
    prev.placement == Placement::Standalone
        && cur.placement == Placement::Standalone
        && !is_atomic_multiline(&prev.text)
        && !is_atomic_multiline(&cur.text)
        && cur.start_line == prev.end_line + 1
}

/// Index ranges into `comments` for each run of mergeable, consecutive
/// entries. `comments` must already be in source order.
fn group_ranges(comments: &[Comment]) -> Vec<Range<usize>> {
    if comments.is_empty() {
        return Vec::new();
    }
    let mut ranges = Vec::new();
    let mut start = 0;
    for i in 1..comments.len() {
        if !mergeable(&comments[i - 1], &comments[i]) {
            ranges.push(start..i);
            start = i;
        }
    }
    ranges.push(start..comments.len());
    ranges
}

/// Merge adjacent standalone single-line comments (no blank line between
/// them) into logical blocks. Comments that stand alone (block comments,
/// trailing comments, or ones separated by a blank line) become
/// single-member blocks.
pub fn merge_comments(comments: &[Comment]) -> Vec<CommentBlock> {
    group_ranges(comments)
        .into_iter()
        .map(|range| build_block(&comments[range]))
        .collect()
}

fn build_block(members: &[Comment]) -> CommentBlock {
    let first = &members[0];
    let last = &members[members.len() - 1];
    let kind = if members.iter().all(|c| c.kind == CommentKind::Doc) {
        CommentKind::Doc
    } else {
        CommentKind::Plain
    };
    CommentBlock {
        byte_range: first.byte_range.start..last.byte_range.end,
        start_line: first.start_line,
        end_line: last.end_line,
        placement: first.placement,
        kind,
        comments: members.to_vec(),
    }
}

/// Go node kinds that count as a "declaration" for the doc-comment
/// adjacency test: a comment immediately preceding one of these, with no
/// blank line between, is a doc comment.
const GO_DECLARATION_KINDS: &[&str] = &[
    "function_declaration",
    "method_declaration",
    "type_declaration",
    "type_spec",
    "const_declaration",
    "const_spec",
    "var_declaration",
    "var_spec",
    "import_declaration",
    "field_declaration",
];

fn is_go_doc_adjacent(last_node: Node) -> bool {
    let Some(sibling) = last_node.next_named_sibling() else {
        return false;
    };
    GO_DECLARATION_KINDS.contains(&sibling.kind())
        && sibling.start_position().row == last_node.end_position().row + 1
}

/// Go has no comment/doc-comment node distinction. A merged block of
/// standalone comments counts as a doc comment only when it is immediately
/// (no blank line) followed by a declaration.
fn extract_go(tree: &Tree, source: &str) -> Vec<Comment> {
    let mut nodes = Vec::new();
    collect_nodes_of_kind(tree.root_node(), &["comment"], &mut nodes);
    let mut comments: Vec<Comment> = nodes
        .iter()
        .map(|&node| make_comment(node, source, CommentKind::Plain))
        .collect();
    for range in group_ranges(&comments) {
        let last_node = nodes[range.end - 1];
        if is_go_doc_adjacent(last_node) {
            for comment in &mut comments[range] {
                comment.kind = CommentKind::Doc;
            }
        }
    }
    comments
}

fn shift_comment(comment: Comment, byte_offset: usize, line_offset: usize) -> Comment {
    Comment {
        byte_range: (comment.byte_range.start + byte_offset)
            ..(comment.byte_range.end + byte_offset),
        start_line: comment.start_line + line_offset,
        end_line: comment.end_line + line_offset,
        ..comment
    }
}

fn raw_text_child(node: Node) -> Option<Node> {
    let mut cursor = node.walk();
    let found = node.children(&mut cursor).find(|c| c.kind() == "raw_text");
    found
}

/// Re-parse a `<script>` block's raw text as TypeScript and extract its
/// comments, shifted back into the outer file's byte/line coordinates.
fn extract_embedded_script(source: &str, raw: Node, out: &mut Vec<Comment>) {
    let range = raw.byte_range();
    let Some(slice) = source.get(range.clone()) else {
        return;
    };
    let mut parser = Parser::new();
    let ts_language: TsLanguage = tree_sitter_typescript::LANGUAGE_TYPESCRIPT.into();
    if parser.set_language(&ts_language).is_err() {
        return;
    }
    let Some(sub_tree) = parser.parse(slice, None) else {
        return;
    };
    let (quality, _) = classify_quality(sub_tree.root_node(), slice.len());
    if quality == ParseQuality::Untrusted {
        return;
    }
    let line_offset = raw.start_position().row;
    let sub_comments = extract_with(
        &sub_tree,
        slice,
        &["comment", "html_comment"],
        classify_by_prefix,
    );
    out.extend(
        sub_comments
            .into_iter()
            .map(|c| shift_comment(c, range.start, line_offset)),
    );
}

fn extract_svelte(tree: &Tree, source: &str) -> Vec<Comment> {
    let mut out = Vec::new();
    collect_script_elements(tree.root_node(), source, &mut out);
    out
}

fn collect_script_elements(node: Node, source: &str, out: &mut Vec<Comment>) {
    if node.kind() == "script_element" {
        if let Some(raw) = raw_text_child(node) {
            extract_embedded_script(source, raw, out);
        }
        return;
    }
    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        collect_script_elements(child, source, out);
    }
}

/// Extract every comment from `tree`, classified per this language's doc
/// convention.
pub fn extract_comments(language: Language, tree: &Tree, source: &str) -> Vec<Comment> {
    match language {
        Language::Rust => extract_with(
            tree,
            source,
            comment_node_kinds(language),
            |node, _source| {
                if is_rust_doc(node) {
                    CommentKind::Doc
                } else {
                    CommentKind::Plain
                }
            },
        ),
        Language::Go => extract_go(tree, source),
        Language::Svelte => extract_svelte(tree, source),
        _ => extract_with(
            tree,
            source,
            comment_node_kinds(language),
            classify_by_prefix,
        ),
    }
}

/// Everything this module can tell a caller about one source file: the
/// language it was actually parsed as (which for a `.h` file may differ
/// from a naive extension guess), how trustworthy the parse is, and
/// (regardless of trust) the comments it found.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Analysis {
    pub language: Language,
    pub quality: ParseQuality,
    pub comments: Vec<Comment>,
    pub blocks: Vec<CommentBlock>,
}

/// Detect the language from `path` and `source`, parse it, and extract its
/// comments. Returns `None` only when the extension is unrecognized -
/// callers should treat that as "skip this file", distinct from a low-trust
/// `Analysis` for a file that was parsed but is too broken to draw
/// conclusions from.
pub fn analyze(path: &Path, source: &str) -> Option<Analysis> {
    let language = Language::detect(path, source)?;
    let mut parser = Parser::new();
    parser.set_language(&language.ts_language()).ok()?;
    let tree = parser.parse(source, None)?;
    let (quality, _stats) = classify_quality(tree.root_node(), source.len());
    let comments = extract_comments(language, &tree, source);
    let blocks = merge_comments(&comments);
    Some(Analysis {
        language,
        quality,
        comments,
        blocks,
    })
}

#[cfg(test)]
mod tests;
