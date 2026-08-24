//! Comment-lint: two tiers of rules over [`crate::lang::Analysis`].
//!
//! Tier 1 (categorical) is a faithful port of `src/banner-comment-lint.ts`'s
//! banner/divider, bare-label, and history-comment patterns, restricted to
//! real comment nodes instead of raw file content so string/regex literals
//! can't trigger a false positive, and using AST line numbers instead of
//! offset arithmetic over the whole file. Tier 2 (heuristic) is new: nudges
//! for unusually long standalone prose and unusually long trailing remarks,
//! calibrated against a census of the author's own code.

use crate::lang::{Analysis, Comment, CommentBlock, CommentKind, ParseQuality, Placement};
use regex::Regex;
use std::collections::{BTreeMap, HashSet};
use std::path::Path;
use std::sync::OnceLock;

/// Which family of comment-lint rule produced a [`Finding`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Category {
    Banner,
    BareLabel,
    History,
    LongProse,
    VerboseTrailing,
}

impl Category {
    pub fn id(self) -> &'static str {
        match self {
            Self::Banner => "banner",
            Self::BareLabel => "bare-label",
            Self::History => "history",
            Self::LongProse => "long-prose",
            Self::VerboseTrailing => "verbose-trailing",
        }
    }

    fn message(self) -> &'static str {
        match self {
            Self::Banner => {
                "Remove decorative dividers. If an adjacent label adds context, keep it as a \
                 plain comment."
            }
            Self::BareLabel => {
                "Remove bare section-label comments (// Handlers, // Utils). If context is \
                 needed, use a doc comment that explains purpose, not just a category name."
            }
            Self::History => {
                "Remove refactoring/history comments (// Previously used..., // Migrated \
                 from...). Code history belongs in commit messages, not inline comments."
            }
            Self::LongProse => {
                "Standalone comment block is unusually long for this codebase. Consider a doc \
                 comment, a linked reference, or trimming to the essential point."
            }
            Self::VerboseTrailing => {
                "Trailing comment is unusually long. Move the explanation to a standalone \
                 comment above the line, or shorten it."
            }
        }
    }
}

/// One rule hit: a comment's real (0-indexed) line, its text, and the rule
/// category that flagged it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Finding {
    pub line: usize,
    pub text: String,
    pub category: Category,
}

/// Every finding from both rule tiers, split by whether it blocks.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Report {
    pub categorical: Vec<Finding>,
    pub nudges: Vec<Finding>,
}

/// Run both rule tiers over `analysis`, with no awareness of parse trust.
pub fn evaluate(analysis: &Analysis) -> Report {
    let categorical = categorical_findings(&analysis.comments);
    let mut nudges = long_prose_findings(&analysis.blocks);
    nudges.extend(verbose_trailing_findings(&analysis.comments));
    nudges.sort_by_key(|f| f.line);
    Report {
        categorical,
        nudges,
    }
}

/// [`evaluate`], but honoring parse trust: `None` means "abstained" (the
/// parse was too broken to draw conclusions from), distinct from
/// `Some(Report::default())` meaning "looked and found nothing".
pub fn evaluate_trusted(analysis: &Analysis) -> Option<Report> {
    if analysis.quality == ParseQuality::Untrusted {
        None
    } else {
        Some(evaluate(analysis))
    }
}

fn banner_patterns() -> &'static [Regex] {
    static PATTERNS: OnceLock<Vec<Regex>> = OnceLock::new();
    PATTERNS.get_or_init(|| {
        const SOURCES: [&str; 6] = [
            r"(?m)^\s*(?://[/!]?|/\*+|#|--|<!--|@rem|::|\*)\s*[-=*#~+._^─═/]{5,}",
            r"(?m)^\s*[-=*#~+._^─═/]{10,}",
            r"(?m)(?://[/!]?|#|--|/\*+|<!--)\s*[-=*#~+._^─═]{2,}\s+\S.{0,60}\s*[-=*#~+._^─═]{2,}",
            r"(?m)[-=*#~+._^─═/]{5,}\s*(?:\*/|-->)",
            r"[╔╗╚╝║│┌┐└┘├┤┬┴┼]{3,}",
            r"(?m)(?://[/!]?|#|--|/\*+|<!--)\s*[-=*#~+._^─═]{1,}\s*\[.{1,60}\]\s*[-=*#~+._^─═]{2,}",
        ];
        SOURCES
            .iter()
            .map(|src| Regex::new(src).expect("valid banner pattern"))
            .collect()
    })
}

fn bare_label_line_pattern() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(
            r"^\s*(?://[/!]?\s*|#\s*|/\*\*?\s*|\*\s*)([A-Za-z0-9_]+(?:\s+[A-Za-z0-9_]+)?)\s*(?:\*/)?\s*$",
        )
        .expect("valid bare-label pattern")
    })
}

fn history_pattern() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(
            r"(?i)^\s*(?://[/!]?|#|/\*\*?|\*)\s*(?:Refactored|Previously|Migrated|Was formerly|\
              Changed from|Replaced|Old approach|Removed|Used to|Originally|This (?:function|\
              method|class|module) was)",
        )
        .expect("valid history pattern")
    })
}

fn encoding_declaration_pattern() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(r"(?i)^\s*#\s*-\*-\s*coding\s*[:=]\s*[-\w.]+\s*-\*-\s*$")
            .expect("valid encoding-declaration pattern")
    })
}

/// Bare section-label words: a comment containing only one of these (or two
/// of them together) adds no information over the code itself.
const BARE_LABELS: &[&str] = &[
    "handlers",
    "handler",
    "utilities",
    "utility",
    "utils",
    "util",
    "helpers",
    "helper",
    "private",
    "public",
    "protected",
    "internal",
    "constants",
    "config",
    "configuration",
    "types",
    "interfaces",
    "imports",
    "exports",
    "methods",
    "functions",
    "properties",
    "variables",
    "state",
    "components",
    "styles",
    "tests",
    "setup",
    "cleanup",
    "init",
    "initialization",
    "main",
    "api",
    "routes",
    "middleware",
    "models",
    "views",
    "controllers",
    "services",
    "repositories",
    "definitions",
    "declarations",
    "enums",
    "structs",
    "classes",
    "traits",
    "implementations",
    "constructors",
    "getters",
    "setters",
    "callbacks",
    "listeners",
    "observers",
    "factories",
    "validators",
    "formatters",
    "parsers",
    "serializers",
    "converters",
    "transformers",
    "mappers",
    "resolvers",
    "actions",
    "reducers",
    "selectors",
    "mutations",
    "queries",
    "subscriptions",
    "hooks",
    "providers",
    "context",
    "effects",
    "signals",
    "stores",
    "computed",
];

fn is_bare_label_line(line: &str) -> bool {
    let Some(caps) = bare_label_line_pattern().captures(line) else {
        return false;
    };
    let words = caps
        .get(1)
        .expect("group 1 present whenever the pattern matches")
        .as_str()
        .to_lowercase();
    words.split_whitespace().all(|w| BARE_LABELS.contains(&w))
}

/// Every banner-pattern match within one comment's own text, deduped to one
/// per absolute line, skipping lines that are a Python encoding cookie
/// (`# -*- coding: ... -*-`, functional syntax rather than decoration).
fn banner_matches(comment: &Comment) -> Vec<(usize, String)> {
    let lines: Vec<&str> = comment.text.split('\n').collect();
    let mut by_line: BTreeMap<usize, String> = BTreeMap::new();
    for pattern in banner_patterns() {
        for m in pattern.find_iter(&comment.text) {
            let offset = comment.text[..m.start()].matches('\n').count();
            let Some(&line_text) = lines.get(offset) else {
                continue;
            };
            if encoding_declaration_pattern().is_match(line_text) {
                continue;
            }
            by_line
                .entry(comment.start_line + offset)
                .or_insert_with(|| line_text.trim().to_string());
        }
    }
    by_line.into_iter().collect()
}

/// Banner, bare-label, and history findings across every comment, in source
/// order. A line already flagged by an earlier pass is never flagged again:
/// banner matches (whole-file pass) take precedence over bare-label/history
/// (per-line pass), matching the TS version's single shared dedup set.
fn categorical_findings(comments: &[Comment]) -> Vec<Finding> {
    let mut seen: HashSet<usize> = HashSet::new();
    let mut findings = Vec::new();

    for comment in comments {
        for (line, text) in banner_matches(comment) {
            if seen.insert(line) {
                findings.push(Finding {
                    line,
                    text,
                    category: Category::Banner,
                });
            }
        }
    }

    for comment in comments {
        for (offset, line_text) in comment.text.split('\n').enumerate() {
            let line = comment.start_line + offset;
            if is_bare_label_line(line_text) {
                if seen.insert(line) {
                    findings.push(Finding {
                        line,
                        text: line_text.trim().to_string(),
                        category: Category::BareLabel,
                    });
                }
                continue;
            }
            if history_pattern().is_match(line_text) && seen.insert(line) {
                findings.push(Finding {
                    line,
                    text: line_text.trim().to_string(),
                    category: Category::History,
                });
            }
        }
    }

    findings.sort_by_key(|f| f.line);
    findings
}

/// Standalone, non-doc comment blocks at or above this many lines are
/// flagged as long prose. Corpus census: p50=1, p75=2, p90=p95=3 lines.
const LONG_PROSE_MIN_LINES: usize = 4;

/// Comment blocks starting within this many lines of the file are exempt
/// from the long-prose nudge as legitimate module/file headers.
const FILE_HEADER_EXEMPT_LINES: usize = 5;

/// Trailing comments exceeding this many words are flagged as verbose.
/// Corpus census p95 = 9 words.
const VERBOSE_TRAILING_MAX_WORDS: usize = 10;

fn long_prose_findings(blocks: &[CommentBlock]) -> Vec<Finding> {
    blocks
        .iter()
        .filter(|b| b.placement == Placement::Standalone)
        .filter(|b| b.kind != CommentKind::Doc)
        .filter(|b| b.start_line >= FILE_HEADER_EXEMPT_LINES)
        .filter(|b| b.end_line - b.start_line + 1 >= LONG_PROSE_MIN_LINES)
        .map(|b| Finding {
            line: b.start_line,
            text: representative_line(&b.comments[0].text),
            category: Category::LongProse,
        })
        .collect()
}

fn verbose_trailing_findings(comments: &[Comment]) -> Vec<Finding> {
    comments
        .iter()
        .filter(|c| c.placement == Placement::Trailing)
        .filter_map(|c| {
            let word_count = strip_markers(&c.text).split_whitespace().count();
            (word_count > VERBOSE_TRAILING_MAX_WORDS).then(|| Finding {
                line: c.start_line,
                text: c.text.trim().to_string(),
                category: Category::VerboseTrailing,
            })
        })
        .collect()
}

fn representative_line(text: &str) -> String {
    text.lines().next().unwrap_or(text).trim().to_string()
}

/// Comment-syntax markers stripped before counting words, longest first so
/// e.g. `///` isn't left with a stray `/` after a `//` match.
const COMMENT_PREFIXES: [&str; 9] = ["///", "//!", "//", "/**", "/*", "<!--", "#!", "#", "--"];

fn strip_markers(text: &str) -> &str {
    let mut s = text.trim();
    for suffix in ["-->", "*/"] {
        if let Some(rest) = s.strip_suffix(suffix) {
            s = rest.trim_end();
        }
    }
    for prefix in COMMENT_PREFIXES {
        if let Some(rest) = s.strip_prefix(prefix) {
            return rest.trim_start();
        }
    }
    s
}

/// The logged rule identifier for `category`.
pub fn rule_id(category: Category) -> String {
    format!("comment-lint.{}", category.id())
}

const MAX_EXAMPLES_PER_CATEGORY: usize = 5;
const CATEGORICAL_ORDER: [Category; 3] = [Category::Banner, Category::BareLabel, Category::History];
const NUDGE_ORDER: [Category; 2] = [Category::LongProse, Category::VerboseTrailing];

fn append_category_section(
    out: &mut Vec<String>,
    file_path: &str,
    findings: &[Finding],
    category: Category,
) {
    let matching: Vec<&Finding> = findings.iter().filter(|f| f.category == category).collect();
    if matching.is_empty() {
        return;
    }
    for f in matching.iter().take(MAX_EXAMPLES_PER_CATEGORY) {
        out.push(format!("  {file_path}:{}: {}", f.line + 1, f.text));
    }
    if matching.len() > MAX_EXAMPLES_PER_CATEGORY {
        out.push(format!(
            "  ... and {} more",
            matching.len() - MAX_EXAMPLES_PER_CATEGORY
        ));
    }
    out.push(String::new());
    out.push(category.message().to_string());
}

/// Render a blocking stderr report: categorical findings first, then any
/// nudges, grouped by category with a 5-example cap per group.
pub fn format_block_report(file_path: &str, report: &Report) -> String {
    let mut out = vec![format!("Comment lint issues in {file_path}:")];
    for category in CATEGORICAL_ORDER {
        append_category_section(&mut out, file_path, &report.categorical, category);
    }
    for category in NUDGE_ORDER {
        append_category_section(&mut out, file_path, &report.nudges, category);
    }
    out.push(String::new());
    out.push(
        "If this is a false positive (generated file, intentional delimiter), disregard."
            .to_string(),
    );
    out.join("\n")
}

/// Render a non-blocking nudge summary for the `additionalContext` field.
pub fn format_nudge_context(file_path: &str, nudges: &[Finding]) -> String {
    let mut out = vec![format!(
        "Comment style nudges in {file_path} (non-blocking):"
    )];
    for category in NUDGE_ORDER {
        append_category_section(&mut out, file_path, nudges, category);
    }
    out.join("\n").trim_end().to_string()
}

/// Suffixes excluded from comment-lint entirely: docs, lockfiles,
/// binary/snapshot formats.
const EXCLUDED_SUFFIXES: &[&str] = &[".md", ".lock", ".svg", ".snap"];

/// Filename substrings marking a file as minified or code-generated.
const EXCLUDED_FILENAME_MARKERS: &[&str] = &[".min.", ".gen.", ".generated."];

/// Directory names that mark vendored, generated, or build output: pruned
/// from `guard check`'s directory walk (see `check::should_skip_dir`) and
/// excluded from comment-lint wherever they appear as a path component,
/// including on the hook path. This is the single shared source of truth
/// for both.
pub const EXCLUDED_DIR_NAMES: &[&str] = &[
    "node_modules",
    "migrations",
    "vendor",
    "generated",
    "target",
    "venv",
    "dist",
    "build",
    "coverage",
    "__pycache__",
    "storybook-static",
    ".svelte-kit",
    ".next",
    ".nuxt",
    "out",
    "site-packages",
    "third_party",
    "external",
];

/// Two-segment directory paths excluded even though neither segment alone
/// is generic enough to exclude on its own (e.g. a project's own `include/`
/// is fine; a vendored `install/include/` is not).
const EXCLUDED_MULTI_SEGMENT_DIRS: &[&str] = &["install/include"];

/// Whether `path` is excluded from comment-lint by suffix, filename marker,
/// or directory component, independent of whether its extension is even a
/// recognized language.
pub fn is_excluded_path(path: &Path) -> bool {
    let file_name = path
        .file_name()
        .map(|n| n.to_string_lossy())
        .unwrap_or_default();
    if EXCLUDED_SUFFIXES.iter().any(|s| file_name.ends_with(s)) {
        return true;
    }
    if EXCLUDED_FILENAME_MARKERS
        .iter()
        .any(|m| file_name.contains(m))
    {
        return true;
    }
    has_excluded_dir_component(path)
}

/// Whether any single path component matches [`EXCLUDED_DIR_NAMES`], or any
/// consecutive pair matches [`EXCLUDED_MULTI_SEGMENT_DIRS`]. Component-aware
/// rather than a raw substring search over the whole path, so a segment like
/// `out` cannot false-positive on an unrelated name that merely contains it
/// (e.g. `routes`).
fn has_excluded_dir_component(path: &Path) -> bool {
    let components: Vec<String> = path
        .components()
        .map(|c| c.as_os_str().to_string_lossy().into_owned())
        .collect();
    components
        .iter()
        .any(|c| EXCLUDED_DIR_NAMES.contains(&c.as_str()))
        || components.windows(2).any(|pair| {
            let joined = format!("{}/{}", pair[0], pair[1]);
            EXCLUDED_MULTI_SEGMENT_DIRS.contains(&joined.as_str())
        })
}

/// Files above this size are skipped unread. The largest hand-written source
/// file across the author's projects is ~1.1MB and parses in ~100ms, so
/// anything past this is generated or vendored and not worth linting.
pub const MAX_SOURCE_BYTES: u64 = 2 * 1024 * 1024;

/// Read and analyze `path` for comment-lint purposes: `None` when the path
/// is excluded, too large, unreadable, or its extension is unrecognized.
pub fn analyze_file(path: &Path) -> Option<Analysis> {
    if is_excluded_path(path) {
        return None;
    }
    if std::fs::metadata(path).ok()?.len() > MAX_SOURCE_BYTES {
        return None;
    }
    let source = std::fs::read_to_string(path).ok()?;
    crate::lang::analyze(path, &source)
}

#[cfg(test)]
mod tests;
