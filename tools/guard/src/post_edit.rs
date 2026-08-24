//! `PostToolUse` handling for `Write`/`Edit`: runs comment-lint's two rule
//! tiers over the freshly written file when the project's config enables it.

use crate::comment::{self, Report};
use crate::config::Config;
use crate::lang::Analysis;
use crate::outcome::Outcome;
use crate::payload::Payload;
use std::path::Path;

pub fn main(payload: &Payload) -> Outcome {
    let Some(file_path) = payload.tool_input.file_path.as_deref() else {
        return Outcome::allow();
    };
    let cwd = payload
        .cwd
        .as_deref()
        .map(Path::new)
        .unwrap_or_else(|| Path::new(file_path));
    let config = Config::load(cwd);
    let relative = relative_file(file_path, cwd);
    if !config.hook_enabled("comment-lint", Some(&relative)) {
        return Outcome::allow();
    }
    let Some(analysis) = analyze(payload) else {
        return Outcome::allow();
    };
    let Some(report) = comment::evaluate_trusted(&analysis) else {
        return Outcome::abstain();
    };
    respond(file_path, &report)
}

/// `file_path` relative to its enclosing repo root, for matching against
/// `comment-lint.ignore` globs; falls back to the raw path when no repo is
/// found.
fn relative_file(file_path: &str, cwd: &Path) -> String {
    crate::git::find_repo(cwd)
        .and_then(|repo| {
            Path::new(file_path)
                .strip_prefix(&repo.root)
                .ok()
                .map(Path::to_path_buf)
        })
        .map(|p| p.to_string_lossy().into_owned())
        .unwrap_or_else(|| file_path.to_string())
}

fn analyze(payload: &Payload) -> Option<Analysis> {
    let file_path = payload.tool_input.file_path.as_deref()?;
    comment::analyze_file(Path::new(file_path))
}

fn respond(file_path: &str, report: &Report) -> Outcome {
    if !report.categorical.is_empty() {
        eprintln!("{}", comment::format_block_report(file_path, report));
        return Outcome::block(rule_ids(report));
    }
    if !report.nudges.is_empty() {
        let context = comment::format_nudge_context(file_path, &report.nudges);
        println!(
            "{}",
            serde_json::json!({
                "hookSpecificOutput": {
                    "hookEventName": "PostToolUse",
                    "additionalContext": context,
                }
            })
        );
        return Outcome::nudge(rule_ids(report));
    }
    Outcome::allow()
}

fn rule_ids(report: &Report) -> Vec<String> {
    let mut ids: Vec<String> = report
        .categorical
        .iter()
        .chain(&report.nudges)
        .map(|f| comment::rule_id(f.category))
        .collect();
    ids.sort();
    ids.dedup();
    ids
}

#[cfg(test)]
mod tests;
