//! `guard hook`: the PreToolUse side. Reads the hook JSON on stdin, runs
//! discipline rules, rewrites capturable pipelines to `guard run '...'`, checks
//! approval, and emits one JSON response.

use crate::approval::{Approval, Decision};
use crate::parse::PipelineInfo;
use crate::rules::{self, Verdict};
use serde::Deserialize;
use std::io::Read;

#[derive(Deserialize)]
struct HookInput {
    tool_name: String,
    tool_input: ToolInput,
}

#[derive(Deserialize)]
struct ToolInput {
    command: Option<String>,
}

/// Result of splicing every capturable pipeline into a `guard run` call.
pub struct Rewrite {
    pub command: String,
    pub count: usize,
}

impl Rewrite {
    /// Wrap each capturable pipeline in `<self_exe> run '<escaped>'`, replacing
    /// spans right-to-left so earlier byte offsets stay valid. Text outside
    /// pipeline spans is preserved byte-for-byte.
    pub fn apply(command: &str, self_exe: &str) -> Rewrite {
        let Some(pls) = PipelineInfo::extract(command) else {
            return Rewrite { command: command.to_string(), count: 0 };
        };
        let mut spans: Vec<(usize, usize, String)> = pls
            .iter()
            .filter(|p| p.capturable())
            .map(|p| {
                let (s, e) = p.byte_span;
                (s, e, format!("{self_exe} run {}", single_quote(&p.text)))
            })
            .collect();
        spans.sort_by_key(|s| std::cmp::Reverse(s.0));

        let mut out = command.to_string();
        for (s, e, repl) in &spans {
            out.replace_range(s..e, repl);
        }
        Rewrite { command: out, count: spans.len() }
    }
}

/// Wrap `s` as one single-quoted shell word, escaping embedded quotes as `'\''`.
pub fn single_quote(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('\'');
    for c in s.chars() {
        if c == '\'' {
            out.push_str("'\\''");
        } else {
            out.push(c);
        }
    }
    out.push('\'');
    out
}

#[derive(Default)]
struct Output {
    permission_allow: bool,
    additional_context: Vec<String>,
    updated_command: Option<String>,
}

impl Output {
    /// Serialize to the PreToolUse hook JSON, or None when nothing to say.
    fn to_json(&self) -> Option<String> {
        if !self.permission_allow && self.additional_context.is_empty() && self.updated_command.is_none()
        {
            return None;
        }
        let mut hook = serde_json::Map::new();
        hook.insert("hookEventName".into(), "PreToolUse".into());
        if self.permission_allow {
            hook.insert("permissionDecision".into(), "allow".into());
        }
        if !self.additional_context.is_empty() {
            hook.insert("additionalContext".into(), self.additional_context.join("\n\n").into());
        }
        if let Some(cmd) = &self.updated_command {
            hook.insert("updatedInput".into(), serde_json::json!({ "command": cmd }));
        }
        Some(serde_json::json!({ "hookSpecificOutput": hook }).to_string())
    }
}

pub fn main() -> i32 {
    let mut buf = String::new();
    if std::io::stdin().read_to_string(&mut buf).is_err() {
        return 0;
    }
    let Ok(input) = serde_json::from_str::<HookInput>(&buf) else {
        return 0;
    };
    if input.tool_name != "Bash" {
        return 0;
    }
    let Some(command) = input.tool_input.command.filter(|c| !c.is_empty()) else {
        return 0;
    };

    // Discipline: any block short-circuits with exit 2.
    let issues = rules::evaluate(&command);
    if issues.iter().any(|i| i.verdict == Verdict::Block) {
        let mut lines: Vec<String> = issues
            .iter()
            .filter(|i| i.verdict == Verdict::Block)
            .map(|i| format!("\u{2022} {}", i.message))
            .collect();
        lines.extend(
            issues
                .iter()
                .filter(|i| i.verdict == Verdict::Warn)
                .map(|i| format!("\u{2022} (warn) {}", i.message)),
        );
        eprintln!("{}", lines.join("\n"));
        return 2;
    }

    let mut out = Output::default();
    for i in issues.iter().filter(|i| i.verdict == Verdict::Warn) {
        out.additional_context.push(format!("\u{2022} {}", i.message));
    }

    // Rewrite capturable pipelines.
    let self_exe = std::env::current_exe()
        .ok()
        .and_then(|p| p.to_str().map(str::to_string))
        .unwrap_or_else(|| "guard".to_string());
    let rewrite = Rewrite::apply(&command, &self_exe);
    if rewrite.count > 0 {
        out.additional_context.push(rewrite_note(rewrite.count));
        out.updated_command = Some(rewrite.command);
    }

    // Approval evaluates the original command; the guard-run wrapper adds no
    // approval surface, so approving the original is equivalent.
    match Approval::load().decide(&command) {
        Decision::Deny => {
            eprintln!("Command contains a denied sub-command.");
            return 2;
        }
        Decision::Allow => out.permission_allow = true,
        Decision::Passthrough => {}
    }

    if let Some(json) = out.to_json() {
        println!("{json}");
    }
    0
}

fn rewrite_note(count: usize) -> String {
    let word = if count == 1 { "pipeline" } else { "pipelines" };
    format!(
        "[guard] Wrapped {count} {word} with `guard run`: filters run as typed while the \
         full pre-filter stream is captured. A [guard] footer reports exit code, duration, \
         and (if output was dropped) the path to the complete output - read it with the \
         Read tool, do not re-run with a larger -n."
    )
}

#[cfg(test)]
mod tests;
