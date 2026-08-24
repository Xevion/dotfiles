//! `guard check <path>`: a human-only, dry-run pass of both comment-lint
//! rule tiers over a file or directory tree, for reviewing rule output
//! across a real corpus before wiring the hook into a project. Never used
//! on the hook path.

use crate::comment::{self, Category, Finding};
use crate::lang::{self, CommentBlock, Language, ParseQuality};
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

pub fn main(root: &str) -> i32 {
    if root.is_empty() {
        eprintln!("usage: guard check <path>");
        return 2;
    }
    let root = Path::new(root);
    if !root.exists() {
        eprintln!("guard check: path does not exist: {}", root.display());
        return 2;
    }

    let mut files = Vec::new();
    collect_files(root, &mut files);
    files.sort();

    let mut totals = Totals::default();
    for file in &files {
        scan_file(file, &mut totals);
    }
    print_totals(&totals);
    0
}

/// Whether a directory should be pruned during the walk: any hidden
/// (dot-prefixed) directory, plus [`comment::EXCLUDED_DIR_NAMES`] - the same
/// list the hook path uses to exclude generated/vendored files.
fn should_skip_dir(name: &str) -> bool {
    name.starts_with('.') || comment::EXCLUDED_DIR_NAMES.contains(&name)
}

fn collect_files(path: &Path, out: &mut Vec<PathBuf>) {
    let Ok(meta) = std::fs::symlink_metadata(path) else {
        return;
    };
    if meta.is_file() {
        out.push(path.to_path_buf());
        return;
    }
    if !meta.is_dir() {
        return;
    }
    let Ok(entries) = std::fs::read_dir(path) else {
        return;
    };
    for entry in entries.filter_map(Result::ok) {
        let Ok(file_type) = entry.file_type() else {
            continue;
        };
        let name = entry.file_name();
        let name = name.to_string_lossy();
        if file_type.is_dir() {
            if !should_skip_dir(&name) {
                collect_files(&entry.path(), out);
            }
        } else if file_type.is_file() {
            out.push(entry.path());
        }
    }
}

fn language_name(language: Language) -> &'static str {
    match language {
        Language::Rust => "rust",
        Language::TypeScript => "typescript",
        Language::Tsx => "tsx",
        Language::JavaScript => "javascript",
        Language::Go => "go",
        Language::Python => "python",
        Language::C => "c",
        Language::Cpp => "cpp",
        Language::Java => "java",
        Language::CSharp => "csharp",
        Language::Bash => "bash",
        Language::Css => "css",
        Language::Kotlin => "kotlin",
        Language::Svelte => "svelte",
        Language::Sql => "sql",
    }
}

fn quality_name(quality: ParseQuality) -> &'static str {
    match quality {
        ParseQuality::Clean => "clean",
        ParseQuality::Degraded => "degraded",
        ParseQuality::Untrusted => "untrusted",
    }
}

#[derive(Default)]
struct QualityCounts {
    clean: usize,
    degraded: usize,
    untrusted: usize,
}

#[derive(Default)]
struct RuleHits {
    count: usize,
    examples: Vec<String>,
}

const MAX_TOTAL_EXAMPLES: usize = 3;

#[derive(Default)]
struct Totals {
    files_scanned: usize,
    by_language: BTreeMap<&'static str, usize>,
    quality: QualityCounts,
    rule_hits: BTreeMap<&'static str, RuleHits>,
}

fn record_hit(totals: &mut Totals, finding: &Finding, display: &str) {
    let hits = totals.rule_hits.entry(finding.category.id()).or_default();
    hits.count += 1;
    if hits.examples.len() < MAX_TOTAL_EXAMPLES {
        hits.examples
            .push(format!("{display}:{}: {}", finding.line + 1, finding.text));
    }
}

fn scan_file(path: &Path, totals: &mut Totals) {
    if comment::is_excluded_path(path) {
        return;
    }
    if Language::from_path(path).is_none() {
        return;
    }
    let Ok(meta) = std::fs::metadata(path) else {
        return;
    };
    if meta.len() > comment::MAX_SOURCE_BYTES {
        return;
    }
    let Ok(source) = std::fs::read_to_string(path) else {
        return;
    };
    let Some(analysis) = lang::analyze(path, &source) else {
        return;
    };

    totals.files_scanned += 1;
    *totals.by_language.entry(language_name(analysis.language)).or_insert(0) += 1;
    match analysis.quality {
        ParseQuality::Clean => totals.quality.clean += 1,
        ParseQuality::Degraded => totals.quality.degraded += 1,
        ParseQuality::Untrusted => totals.quality.untrusted += 1,
    }

    let display = path.display().to_string();
    let Some(report) = comment::evaluate_trusted(&analysis) else {
        println!(
            "{display}  lang={} quality=untrusted abstained",
            language_name(analysis.language)
        );
        return;
    };

    println!(
        "{display}  lang={} quality={} categorical={} nudges={}",
        language_name(analysis.language),
        quality_name(analysis.quality),
        report.categorical.len(),
        report.nudges.len(),
    );
    for finding in report.categorical.iter().chain(&report.nudges) {
        println!("{}", format_finding(&display, finding, &analysis.blocks));
        record_hit(totals, finding, &display);
    }
}

/// The first this many lines of a long-prose block are shown as a preview.
const LONG_PROSE_PREVIEW_LINES: usize = 3;

fn format_plain_finding(display: &str, finding: &Finding) -> String {
    format!(
        "  {display}:{}: [{}] {}",
        finding.line + 1,
        finding.category.id(),
        finding.text
    )
}

/// Format one finding for human review. `long-prose` findings get their full
/// block extent and a short indented preview, since a single opening line
/// isn't enough to judge whether a 4+ line block is a real false positive.
fn format_finding(display: &str, finding: &Finding, blocks: &[CommentBlock]) -> String {
    if finding.category != Category::LongProse {
        return format_plain_finding(display, finding);
    }
    let Some(block) = blocks.iter().find(|b| b.start_line == finding.line) else {
        return format_plain_finding(display, finding);
    };
    let line_count = block.end_line - block.start_line + 1;
    let mut out = format!(
        "  {display}:{}: [{}] lines {}-{}, {line_count}L",
        finding.line + 1,
        finding.category.id(),
        block.start_line + 1,
        block.end_line + 1,
    );
    let preview = block_preview_lines(block);
    for line in preview.iter().take(LONG_PROSE_PREVIEW_LINES) {
        out.push_str(&format!("\n    {line}"));
    }
    if preview.len() > LONG_PROSE_PREVIEW_LINES {
        out.push_str(&format!(
            "\n    ... ({} more lines)",
            preview.len() - LONG_PROSE_PREVIEW_LINES
        ));
    }
    out
}

/// Every line of a block's text, trimmed. A block is either one atomic
/// multi-line comment (`/* ... */`) or several merged single-line comments;
/// either way this flattens it to one trimmed string per source line.
fn block_preview_lines(block: &CommentBlock) -> Vec<String> {
    if block.comments.len() == 1 {
        block.comments[0].text.lines().map(|l| l.trim().to_string()).collect()
    } else {
        block.comments.iter().map(|c| c.text.trim().to_string()).collect()
    }
}

fn print_totals(totals: &Totals) {
    println!();
    println!("=== totals ===");
    println!("files scanned: {}", totals.files_scanned);
    println!("by language:");
    for (lang_name, count) in &totals.by_language {
        println!("  {lang_name}: {count}");
    }
    println!("parse quality:");
    println!("  clean: {}", totals.quality.clean);
    println!("  degraded: {}", totals.quality.degraded);
    println!("  untrusted: {}", totals.quality.untrusted);
    println!("rule hits:");
    if totals.rule_hits.is_empty() {
        println!("  none");
    }
    for (rule, hits) in &totals.rule_hits {
        println!("  {rule}: {}", hits.count);
        for example in &hits.examples {
            println!("    e.g. {example}");
        }
    }
}

#[cfg(test)]
mod tests;
