//! Bash grammar to pipelines with byte spans, a stage model, and the capture
//! predicate. Shared by `guard hook` (locate rewrites) and `guard run`
//! (execute a pipeline). Every fallible path fails open: parse errors and
//! unrecognized constructs yield "not capturable", never a wrong rewrite.
//!
//! brush-parser is a parser, not an evaluator, so word values keep their source
//! text (`$VAR` stays literal) - the predicate leans on that to spot
//! unexpanded expansions.

use brush_parser::ast::{self, SourceLocation};
use brush_parser::{Parser, ParserOptions, SourceSpan};
use std::io::Cursor;
use std::path::Path;

/// Filter commands that make a pipeline worth capturing when present as a
/// non-first stage.
pub const FILTERS: &[&str] = &[
    "head", "tail", "grep", "rg", "sed", "awk", "jq", "wc", "sort", "uniq", "cut", "tr",
];

/// Names Claude Code rewires to embedded binaries (ripgrep / bfs / ugrep). The
/// runner replicates the wrapper so a captured filter runs the same tool.
pub const HARNESS_WRAPPED: &[&str] = &["rg", "find", "grep"];

/// A redirection on a stage, restricted to the reproducible set. Anything else
/// makes the stage `Unsupported`.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Redir {
    /// `N>&M`, e.g. `2>&1`.
    Dup { from: i32, to: i32 },
    /// `>f` / `>>f` / `<f` / `2>f`.
    File { fd: i32, kind: FileKind, path: String },
    /// `&>f` / `&>>f`.
    OutErr { path: String, append: bool },
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum FileKind {
    Write,
    Append,
    Read,
}

/// One stage of a pipeline.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Stage {
    /// Simple command: leading `VAR=val`, an argv, supported redirects.
    Simple {
        assignments: Vec<(String, String)>,
        argv: Vec<String>,
        redirs: Vec<Redir>,
    },
    /// Compound command (brace group, subshell, loop). Carries source text so
    /// the runner can exec it via `$SHELL -c`.
    Compound { text: String },
    /// Anything the runner can't reproduce: process substitution, fd-closing,
    /// heredocs, function defs, extended tests.
    Unsupported,
}

impl Stage {
    /// argv0 of a simple stage; None for compound / unsupported / empty.
    pub fn argv0(&self) -> Option<&str> {
        match self {
            Stage::Simple { argv, .. } => argv.first().map(String::as_str),
            _ => None,
        }
    }
}

/// A command-position pipeline and the byte span it occupies in the source.
#[derive(Clone, Debug)]
pub struct PipelineInfo {
    /// Byte offsets `[start, end)` within the original command.
    pub byte_span: (usize, usize),
    pub stages: Vec<Stage>,
    /// Terminated by `&`; never captured.
    pub is_async: bool,
    /// `! pipeline`; excluded in v1.
    pub bang: bool,
    /// `time pipeline`; excluded in v1.
    pub timed: bool,
    /// Raw source text (the `byte_span` slice).
    pub text: String,
}

impl PipelineInfo {
    /// Extract the top-level command-position pipelines of `cmd`, each with its
    /// byte span. Fail-open: None on parse error. Pipelines nested in compound
    /// bodies are not descended into; such a parent is a single compound stage.
    pub fn extract(cmd: &str) -> Option<Vec<PipelineInfo>> {
        let prog = parse_opt(cmd)?;
        let table = CharBytes::new(cmd);
        let mut out = Vec::new();

        for cc in &prog.complete_commands {
            for item in &cc.0 {
                let is_async = matches!(item.1, ast::SeparatorOperator::Async);
                let andor = &item.0;
                let mut pls: Vec<&ast::Pipeline> = vec![&andor.first];
                for a in &andor.additional {
                    match a {
                        ast::AndOr::And(p) | ast::AndOr::Or(p) => pls.push(p),
                    }
                }
                for pl in pls {
                    let Some((bs, be)) = pl.location().and_then(|sp| table.span(&sp)) else {
                        continue;
                    };
                    let stages = pl.seq.iter().map(|c| extract_stage(c, cmd, &table)).collect();
                    out.push(PipelineInfo {
                        byte_span: (bs, be),
                        stages,
                        is_async,
                        bang: pl.bang,
                        timed: pl.timed.is_some(),
                        text: cmd.get(bs..be).unwrap_or_default().to_string(),
                    });
                }
            }
        }
        Some(out)
    }

    /// Parse a string expected to be exactly one pipeline (the `guard run`
    /// argument). None if it is not a single pipeline.
    pub fn single(cmd: &str) -> Option<PipelineInfo> {
        let pls = PipelineInfo::extract(cmd)?;
        if pls.len() != 1 {
            return None;
        }
        pls.into_iter().next()
    }

    /// Capturable only when the runner can execute it byte-identically and there
    /// is something to narrow. Any doubt returns false.
    pub fn capturable(&self) -> bool {
        if self.is_async || self.bang || self.timed || self.stages.len() < 2 {
            return false;
        }
        // Unexpanded expansions excluded in v1 to keep single-quote splicing
        // safe. A raw scan covers parameter, arithmetic, and command subst.
        if self.text.contains('$') || self.text.contains('`') {
            return false;
        }

        let mut has_filter = false;
        for (i, stage) in self.stages.iter().enumerate() {
            match stage {
                Stage::Unsupported => return false,
                // Compound allowed only as the first stage.
                Stage::Compound { .. } if i != 0 => return false,
                Stage::Compound { .. } => {}
                Stage::Simple { argv, .. } => {
                    let Some(argv0) = argv.first() else {
                        return false; // assignment-only stage
                    };
                    let base = basename(argv0);
                    // Idempotency; alias/builtin shield.
                    if base == "guard" || !resolves_on_path(argv0) {
                        return false;
                    }
                    // Never wrap a follow.
                    if matches!(base, "tail" | "head")
                        && argv.iter().any(|a| a == "-f" || a.starts_with("--follow"))
                    {
                        return false;
                    }
                    if i > 0 && FILTERS.contains(&base) {
                        has_filter = true;
                    }
                }
            }
        }
        has_filter
    }
}

/// Parse a command line, fail-open on any error.
pub fn parse_opt(cmd: &str) -> Option<ast::Program> {
    let opts = ParserOptions::default();
    let mut p = Parser::new(Cursor::new(cmd.as_bytes().to_vec()), &opts);
    p.parse_program().ok()
}

/// char-index (what brush spans report) to byte-offset conversion.
struct CharBytes {
    table: Vec<usize>,
}

impl CharBytes {
    fn new(cmd: &str) -> Self {
        let mut table: Vec<usize> = cmd.char_indices().map(|(b, _)| b).collect();
        table.push(cmd.len());
        Self { table }
    }

    fn span(&self, span: &SourceSpan) -> Option<(usize, usize)> {
        Some((
            *self.table.get(span.start.index)?,
            *self.table.get(span.end.index)?,
        ))
    }
}

fn extract_stage(cmd: &ast::Command, src: &str, table: &CharBytes) -> Stage {
    match cmd {
        ast::Command::Simple(s) => extract_simple(s),
        ast::Command::Compound(_, _) => match cmd.location().and_then(|sp| table.span(&sp)) {
            Some((s, e)) => Stage::Compound {
                text: src.get(s..e).unwrap_or_default().to_string(),
            },
            None => Stage::Unsupported,
        },
        ast::Command::Function(_) | ast::Command::ExtendedTest(_, _) => Stage::Unsupported,
    }
}

fn extract_simple(s: &ast::SimpleCommand) -> Stage {
    let mut st = SimpleBuilder::new();
    if let Some(prefix) = &s.prefix {
        for it in &prefix.0 {
            st.item(it);
        }
    }
    if let Some(w) = &s.word_or_name {
        st.argv.push(w.value.clone());
    }
    if let Some(suffix) = &s.suffix {
        for it in &suffix.0 {
            st.item(it);
        }
    }
    st.finish()
}

struct SimpleBuilder {
    assignments: Vec<(String, String)>,
    argv: Vec<String>,
    redirs: Vec<Redir>,
    ok: bool,
}

impl SimpleBuilder {
    fn new() -> Self {
        Self {
            assignments: Vec::new(),
            argv: Vec::new(),
            redirs: Vec::new(),
            ok: true,
        }
    }

    fn item(&mut self, it: &ast::CommandPrefixOrSuffixItem) {
        match it {
            ast::CommandPrefixOrSuffixItem::AssignmentWord(a, _) => match (&a.name, &a.value) {
                // Scalar `NAME=value` before argv0 is a child env entry.
                (ast::AssignmentName::VariableName(name), ast::AssignmentValue::Scalar(v))
                    if self.argv.is_empty() =>
                {
                    self.assignments.push((name.clone(), v.value.clone()));
                }
                // After argv0 it is just an argument (`env X=1`).
                _ if !self.argv.is_empty() => self.argv.push(format!("{a}")),
                _ => self.ok = false,
            },
            ast::CommandPrefixOrSuffixItem::Word(w) => self.argv.push(w.value.clone()),
            ast::CommandPrefixOrSuffixItem::IoRedirect(io) => match conv_redir(io) {
                Some(r) => self.redirs.push(r),
                None => self.ok = false,
            },
            ast::CommandPrefixOrSuffixItem::ProcessSubstitution(_, _) => self.ok = false,
        }
    }

    fn finish(self) -> Stage {
        if !self.ok {
            return Stage::Unsupported;
        }
        Stage::Simple {
            assignments: self.assignments,
            argv: self.argv,
            redirs: self.redirs,
        }
    }
}

fn conv_redir(io: &ast::IoRedirect) -> Option<Redir> {
    use ast::{IoFileRedirectKind as K, IoFileRedirectTarget as T};
    match io {
        ast::IoRedirect::File(fd, kind, target) => match (kind, target) {
            (K::Write | K::Clobber, T::Filename(w)) => Some(Redir::File {
                fd: fd.unwrap_or(1),
                kind: FileKind::Write,
                path: w.value.clone(),
            }),
            (K::Append, T::Filename(w)) => Some(Redir::File {
                fd: fd.unwrap_or(1),
                kind: FileKind::Append,
                path: w.value.clone(),
            }),
            (K::Read, T::Filename(w)) => Some(Redir::File {
                fd: fd.unwrap_or(0),
                kind: FileKind::Read,
                path: w.value.clone(),
            }),
            (K::DuplicateOutput | K::DuplicateInput, T::Duplicate(w)) => Some(Redir::Dup {
                from: fd.unwrap_or(1),
                to: w.value.parse().ok()?, // rejects `-` (close) and non-numeric
            }),
            (K::DuplicateOutput | K::DuplicateInput, T::Fd(n)) => Some(Redir::Dup {
                from: fd.unwrap_or(1),
                to: *n,
            }),
            _ => None,
        },
        ast::IoRedirect::OutputAndError(w, append) => Some(Redir::OutErr {
            path: w.value.clone(),
            append: *append,
        }),
        ast::IoRedirect::HereDocument(_, _) | ast::IoRedirect::HereString(_, _) => None,
    }
}

pub fn basename(s: &str) -> &str {
    match s.rfind('/') {
        Some(i) => &s[i + 1..],
        None => s,
    }
}

/// True when argv0 resolves exactly as the harness shell would: an
/// absolute/relative path, or a PATH search. Builtins, aliases, and snapshot
/// functions deliberately fail this.
pub fn resolves_on_path(argv0: &str) -> bool {
    if argv0.is_empty() {
        return false;
    }
    if argv0.contains('/') {
        return is_executable(Path::new(argv0));
    }
    let Ok(path) = std::env::var("PATH") else {
        return false;
    };
    std::env::split_paths(&path).any(|dir| is_executable(&dir.join(argv0)))
}

fn is_executable(p: &Path) -> bool {
    use std::os::unix::fs::PermissionsExt;
    p.metadata()
        .map(|m| m.is_file() && m.permissions().mode() & 0o111 != 0)
        .unwrap_or(false)
}

#[cfg(test)]
mod tests;
