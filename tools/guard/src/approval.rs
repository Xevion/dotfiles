//! Approval: decompose a compound command into its sub-commands and match each
//! against the user's allow/deny/ask prefixes. Auto-allow when every part is
//! allowed, deny when any part is denied, otherwise defer to the normal flow.
//! Pipeline filters (head/grep/...) are trusted, so a wrapped `cargo test | tail`
//! auto-allows on `cargo test` alone.

use crate::parse::{basename, parse_opt, FILTERS};
use brush_parser::ast;
use serde::Deserialize;
use std::collections::BTreeSet;

const TRANSPARENT: &[&str] = &[
    "command", "builtin", "env", "nice", "nohup", "time", "doas", "exec", "strace", "ltrace",
    "xargs",
];

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Decision {
    Allow,
    Deny,
    Passthrough,
}

#[derive(Default, Deserialize)]
struct Settings {
    permissions: Option<Permissions>,
}

#[derive(Default, Deserialize)]
struct Permissions {
    #[serde(default)]
    allow: Vec<String>,
    #[serde(default)]
    deny: Vec<String>,
    #[serde(default)]
    ask: Vec<String>,
}

pub struct Approval {
    allow: Vec<String>,
    deny: Vec<String>,
    ask: Vec<String>,
}

impl Approval {
    pub fn load() -> Self {
        let mut allow = BTreeSet::new();
        let mut deny = BTreeSet::new();
        let mut ask = BTreeSet::new();
        for file in settings_files() {
            let Ok(text) = std::fs::read_to_string(&file) else {
                continue;
            };
            let Ok(s) = serde_json::from_str::<Settings>(&text) else {
                continue;
            };
            let Some(p) = s.permissions else { continue };
            extend_prefixes(&mut allow, &p.allow);
            extend_prefixes(&mut deny, &p.deny);
            extend_prefixes(&mut ask, &p.ask);
        }
        Approval {
            allow: allow.into_iter().collect(),
            deny: deny.into_iter().collect(),
            ask: ask.into_iter().collect(),
        }
    }

    /// Decide approval for the original command. Passthrough leaves the normal
    /// permission flow untouched.
    pub fn decide(&self, command: &str) -> Decision {
        let cmds = collect_commands(command);
        if cmds.is_empty() {
            return Decision::Passthrough;
        }
        if self.is_safe_xevion_content(&cmds) {
            return Decision::Allow;
        }
        if !is_compound(command) || self.allow.is_empty() {
            return Decision::Passthrough;
        }
        let mut all_allowed = true;
        for argv in &cmds {
            match self.status(argv) {
                Status::Denied => return Decision::Deny,
                Status::Allowed => {}
                Status::Unknown => all_allowed = false,
            }
        }
        if all_allowed {
            Decision::Allow
        } else {
            Decision::Passthrough
        }
    }

    fn status(&self, argv: &[String]) -> Status {
        // Pipeline filters are read-only transforms; trust them so a wrapped
        // pipeline auto-allows on its source command alone.
        if argv.first().map(|a| FILTERS.contains(&basename(a))).unwrap_or(false) {
            return Status::Allowed;
        }
        let cands = candidates(argv);
        if cands.iter().any(|c| matches_any(c, &self.deny)) {
            return Status::Denied;
        }
        if cands.iter().any(|c| matches_any(c, &self.allow)) {
            return Status::Allowed;
        }
        Status::Unknown
    }

    /// Safe `xevion projects content <verb>` body edits: single, non-compound,
    /// neither denied nor gated by an ask rule. Suppresses the built-in
    /// brace-quote prompt on PM-JSON payloads.
    fn is_safe_xevion_content(&self, cmds: &[Vec<String>]) -> bool {
        const SAFE_VERBS: &[&str] = &["list", "get", "insert", "replace", "rm", "move"];
        if cmds.len() != 1 {
            return false;
        }
        let argv = strip_transparent(&cmds[0]);
        if basename(argv.first().map(String::as_str).unwrap_or("")) != "xevion" {
            return false;
        }
        if argv.get(1).map(String::as_str) != Some("projects")
            || argv.get(2).map(String::as_str) != Some("content")
            || !SAFE_VERBS.contains(&argv.get(3).map(String::as_str).unwrap_or(""))
        {
            return false;
        }
        let cands = candidates(&cmds[0]);
        !cands.iter().any(|c| matches_any(c, &self.deny) || matches_any(c, &self.ask))
    }
}

#[derive(PartialEq)]
enum Status {
    Allowed,
    Denied,
    Unknown,
}

fn extend_prefixes(set: &mut BTreeSet<String>, patterns: &[String]) {
    for p in patterns {
        if let Some(prefix) = extract_bash_prefix(p) {
            set.insert(prefix);
        }
    }
}

/// `Bash(cargo test:*)` -> `cargo test`. None for non-Bash patterns.
fn extract_bash_prefix(pattern: &str) -> Option<String> {
    let inner = pattern.strip_prefix("Bash(")?;
    let inner = inner.strip_suffix(')').unwrap_or(inner);
    let inner = inner
        .strip_suffix(":*")
        .or_else(|| inner.strip_suffix(" *"))
        .or_else(|| inner.strip_suffix('*'))
        .unwrap_or(inner);
    let trimmed = inner.trim_end();
    (!trimmed.is_empty()).then(|| trimmed.to_string())
}

fn settings_files() -> Vec<std::path::PathBuf> {
    let mut files = Vec::new();
    if let Ok(home) = std::env::var("HOME") {
        files.push(format!("{home}/.claude/settings.json").into());
        files.push(format!("{home}/.claude/settings.local.json").into());
    }
    match git_root() {
        Some(root) => {
            files.push(format!("{root}/.claude/settings.json").into());
            files.push(format!("{root}/.claude/settings.local.json").into());
        }
        None => {
            files.push(".claude/settings.json".into());
            files.push(".claude/settings.local.json".into());
        }
    }
    files
}

fn git_root() -> Option<String> {
    let out = std::process::Command::new("git")
        .args(["rev-parse", "--show-toplevel"])
        .output()
        .ok()?;
    if !out.status.success() {
        return None;
    }
    Some(String::from_utf8_lossy(&out.stdout).trim().to_string())
}

/// Every simple command's argv in the tree, plus one level of `bash -c` /
/// `sh -c` payload expansion.
fn collect_commands(command: &str) -> Vec<Vec<String>> {
    let Some(prog) = parse_opt(command) else {
        return Vec::new();
    };
    let mut out = Vec::new();
    for cc in &prog.complete_commands {
        collect_list(cc, &mut out);
    }
    let mut expanded = out.clone();
    for argv in &out {
        if let Some(payload) = shell_c_payload(argv) {
            if let Some(inner) = parse_opt(&payload) {
                for cc in &inner.complete_commands {
                    collect_list(cc, &mut expanded);
                }
            }
        }
    }
    expanded
}

fn collect_list(list: &ast::CompoundList, out: &mut Vec<Vec<String>>) {
    for item in &list.0 {
        collect_andor(&item.0, out);
    }
}

fn collect_andor(andor: &ast::AndOrList, out: &mut Vec<Vec<String>>) {
    collect_pipeline(&andor.first, out);
    for a in &andor.additional {
        match a {
            ast::AndOr::And(p) | ast::AndOr::Or(p) => collect_pipeline(p, out),
        }
    }
}

fn collect_pipeline(pl: &ast::Pipeline, out: &mut Vec<Vec<String>>) {
    for cmd in &pl.seq {
        collect_command(cmd, out);
    }
}

fn collect_command(cmd: &ast::Command, out: &mut Vec<Vec<String>>) {
    use ast::CompoundCommand as C;
    match cmd {
        ast::Command::Simple(s) => {
            let argv = simple_argv(s);
            if !argv.is_empty() {
                out.push(argv);
            }
        }
        ast::Command::Compound(cc, _) => match cc {
            C::BraceGroup(b) => collect_list(&b.list, out),
            C::Subshell(s) => collect_list(&s.list, out),
            C::ForClause(f) => collect_list(&f.body.list, out),
            C::WhileClause(w) | C::UntilClause(w) => {
                collect_list(&w.0, out);
                collect_list(&w.1.list, out);
            }
            C::IfClause(i) => {
                collect_list(&i.condition, out);
                collect_list(&i.then, out);
                if let Some(elses) = &i.elses {
                    for e in elses {
                        if let Some(c) = &e.condition {
                            collect_list(c, out);
                        }
                        collect_list(&e.body, out);
                    }
                }
            }
            C::CaseClause(c) => {
                for item in &c.cases {
                    if let Some(l) = &item.cmd {
                        collect_list(l, out);
                    }
                }
            }
            C::Arithmetic(_) | C::ArithmeticForClause(_) | C::Coprocess(_) => {}
        },
        ast::Command::Function(f) => collect_command_compound(&f.body.0, out),
        ast::Command::ExtendedTest(_, _) => {}
    }
}

fn collect_command_compound(cc: &ast::CompoundCommand, out: &mut Vec<Vec<String>>) {
    // Function bodies wrap a compound; reuse the compound arm.
    let wrapper = ast::Command::Compound(cc.clone(), None);
    collect_command(&wrapper, out);
}

fn simple_argv(s: &ast::SimpleCommand) -> Vec<String> {
    let mut argv = Vec::new();
    if let Some(prefix) = &s.prefix {
        for it in &prefix.0 {
            if let ast::CommandPrefixOrSuffixItem::Word(w) = it {
                argv.push(w.value.clone());
            }
        }
    }
    if let Some(w) = &s.word_or_name {
        argv.push(w.value.clone());
    }
    if let Some(suffix) = &s.suffix {
        for it in &suffix.0 {
            if let ast::CommandPrefixOrSuffixItem::Word(w) = it {
                argv.push(w.value.clone());
            }
        }
    }
    argv
}

/// The payload of `bash -c '<payload>'` / `sh -c '<payload>'`, if this is one.
fn shell_c_payload(argv: &[String]) -> Option<String> {
    let base = basename(argv.first()?);
    if base != "bash" && base != "sh" {
        return None;
    }
    let idx = argv.iter().position(|a| a == "-c")?;
    argv.get(idx + 1).cloned()
}

fn strip_transparent(argv: &[String]) -> Vec<String> {
    let mut i = 0;
    while i < argv.len() && TRANSPARENT.contains(&basename(&argv[i])) {
        i += 1;
    }
    argv[i..].to_vec()
}

/// Candidate strings to match against prefixes: the full argv joined, with and
/// without trailing redirects, plus transparent-stripped variants.
fn candidates(argv: &[String]) -> Vec<String> {
    let mut cands = Vec::new();
    let full = argv.join(" ");
    push_unique(&mut cands, strip_redirects(&full));
    push_unique(&mut cands, full);
    let stripped = strip_transparent(argv);
    if !stripped.is_empty() && stripped.len() < argv.len() {
        let s = stripped.join(" ");
        push_unique(&mut cands, strip_redirects(&s));
        push_unique(&mut cands, s);
    }
    cands
}

fn push_unique(v: &mut Vec<String>, s: String) {
    if !v.contains(&s) {
        v.push(s);
    }
}

/// Drop trailing redirects so `bq query foo 2>&1` matches `bq query`. Handles
/// both glued (`2>&1`, `>f`) and split (`> f`, `2>> log`) forms.
fn strip_redirects(cmd: &str) -> String {
    let mut toks: Vec<&str> = cmd.split_whitespace().collect();
    loop {
        let n = toks.len();
        if n == 0 {
            break;
        }
        if is_redirect_glued(toks[n - 1]) {
            toks.pop();
            continue;
        }
        if n >= 2 && is_redirect_op(toks[n - 2]) {
            toks.truncate(n - 2);
            continue;
        }
        break;
    }
    toks.join(" ")
}

/// A bare redirect operator token: `>`, `>>`, `<`, `2>`, `&>`, `>&`, `>|`.
fn is_redirect_op(t: &str) -> bool {
    let op = t.trim_start_matches(|c: char| c.is_ascii_digit());
    matches!(op, ">" | ">>" | "<" | ">&" | "&>" | "<&" | ">|")
}

/// A redirect glued to its target: `2>&1`, `>file`, `2>/dev/null`, `&>out`.
fn is_redirect_glued(t: &str) -> bool {
    if let Some(rest) = t.strip_prefix("&>") {
        return !rest.is_empty();
    }
    let rest = t.trim_start_matches(|c: char| c.is_ascii_digit());
    (rest.starts_with('>') || rest.starts_with('<')) && rest.len() > 1
}

fn matches_any(cmd: &str, prefixes: &[String]) -> bool {
    prefixes.iter().any(|p| matches_prefix(cmd, p))
}

fn matches_prefix(cmd: &str, prefix: &str) -> bool {
    cmd == prefix
        || cmd.starts_with(&format!("{prefix} "))
        || cmd.starts_with(&format!("{prefix}/"))
}

fn is_compound(command: &str) -> bool {
    command.contains(['|', '&', ';', '`'])
        || command.contains("$(")
        || command.contains("<(")
        || command.contains(">(")
}

#[cfg(test)]
mod tests;
