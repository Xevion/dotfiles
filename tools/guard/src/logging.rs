//! Appends one JSONL record per hook invocation to
//! `~/.claude/hooks/logs/<project>/<YYYY-MM>.jsonl`. Every failure is
//! swallowed - a broken log path must never affect the hook's exit code.

use crate::outcome::Outcome;
use crate::payload::Payload;
use serde::Serialize;
use std::fs::OpenOptions;
use std::io::{Error, Write};
use std::path::{Path, PathBuf};
use std::time::Duration;

#[derive(Serialize)]
struct Record<'a> {
    timestamp: String,
    session_id: Option<&'a str>,
    event: &'a str,
    tool: &'a str,
    cwd: Option<&'a str>,
    project: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    file: Option<&'a str>,
    decision: &'a str,
    rules: &'a [String],
    duration_ms: u64,
}

pub fn record(payload: &Payload, outcome: &Outcome, duration: Duration) {
    let _ = try_record(payload, outcome, duration);
}

fn try_record(payload: &Payload, outcome: &Outcome, duration: Duration) -> std::io::Result<()> {
    let home = std::env::var("HOME").map_err(|_| Error::other("HOME not set"))?;
    write_record(Path::new(&home), payload, outcome, duration)
}

/// Write one record under `home/.claude/hooks/logs/...`. Split out from
/// `try_record` so tests can point `home` at a scratch directory instead of
/// mutating the process-wide `HOME` environment variable.
fn write_record(
    home: &Path,
    payload: &Payload,
    outcome: &Outcome,
    duration: Duration,
) -> std::io::Result<()> {
    let start_dir = payload
        .cwd
        .as_ref()
        .map(PathBuf::from)
        .or_else(|| std::env::current_dir().ok())
        .ok_or_else(|| Error::other("no cwd available"))?;
    let project_dir = crate::git::find_repo(&start_dir)
        .map(|r| r.root)
        .unwrap_or_else(|| start_dir.clone());
    let project = project_slug(&project_dir);

    let now = jiff::Zoned::now();
    let month = format!("{:04}-{:02}", now.year(), now.month());
    let path = home
        .join(".claude/hooks/logs")
        .join(&project)
        .join(format!("{month}.jsonl"));
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)?;
    }

    let record = Record {
        timestamp: now.timestamp().to_string(),
        session_id: payload.session_id.as_deref(),
        event: &payload.hook_event_name,
        tool: &payload.tool_name,
        cwd: payload.cwd.as_deref(),
        project: &project,
        file: payload.tool_input.file_path.as_deref(),
        decision: decision_for(outcome),
        rules: &outcome.rules,
        duration_ms: u64::try_from(duration.as_millis()).unwrap_or(u64::MAX),
    };
    let mut line = serde_json::to_string(&record).map_err(Error::other)?;
    line.push('\n');

    let mut file = OpenOptions::new().create(true).append(true).open(&path)?;
    file.write_all(line.as_bytes())
}

/// `outcome.abstained` overrides the exit-code-derived label: an abstain
/// exits 0 like an allow, but means "did not look", not "looked, allowed".
fn decision_for(outcome: &Outcome) -> &'static str {
    if outcome.abstained {
        "abstain"
    } else {
        decision_label(outcome.exit_code)
    }
}

fn decision_label(exit_code: i32) -> &'static str {
    match exit_code {
        0 => "allow",
        2 => "block",
        _ => "error",
    }
}

/// A filesystem-safe project slug: the sanitized directory basename plus an
/// 8-hex-char suffix derived from the full path, so two different paths that
/// happen to share a basename never collide.
fn project_slug(path: &Path) -> String {
    let name = path
        .file_name()
        .map(|n| n.to_string_lossy().into_owned())
        .unwrap_or_else(|| "unknown".to_string());
    let hash = blake3::hash(path.to_string_lossy().as_bytes());
    format!("{}-{}", sanitize(&name), &hash.to_hex()[..8])
}

fn sanitize(name: &str) -> String {
    let cleaned: String = name
        .chars()
        .map(|c| {
            if c.is_ascii_alphanumeric() || c == '-' || c == '_' || c == '.' {
                c
            } else {
                '_'
            }
        })
        .collect();
    if cleaned.is_empty() {
        "unknown".to_string()
    } else {
        cleaned
    }
}

#[cfg(test)]
mod tests;
