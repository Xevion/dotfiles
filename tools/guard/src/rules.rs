//! Discipline rules: block dangerous commands, warn on wasteful ones. Walks the
//! full command tree (into compound bodies and function defs) so a banned
//! pattern can't hide inside a brace group or loop. Fail-open: a parse error
//! yields no issues, and the command runs untouched.

use crate::nested::{nested_payload, Nested};
use crate::parse::{basename, parse_opt, TRANSPARENT};
use brush_parser::ast;
use std::collections::HashSet;

/// Cap on nested-shell recursion (`bash -c` inside `bash -c` ...); real
/// commands nest one or two deep, this is a backstop against pathological input.
const MAX_DEPTH: usize = 6;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Verdict {
    Warn,
    Block,
}

pub struct Issue {
    pub verdict: Verdict,
    pub message: String,
}

/// One simple command in the tree, with the context a rule needs.
struct Cmd {
    /// Effective basename after stripping transparent prefixes.
    name: String,
    /// Full argv including argv0, raw source text.
    argv: Vec<String>,
    right_of_pipe: bool,
    in_pipeline: bool,
    redirs: Vec<Redir>,
}

struct Redir {
    fd: i32,
    kind: RedirKind,
    target: String,
}

#[derive(PartialEq)]
enum RedirKind {
    Write,
    Append,
    Read,
    Dup,
    Other,
}

pub fn evaluate(command: &str) -> Vec<Issue> {
    let Some(prog) = parse_opt(command) else {
        return Vec::new();
    };
    let mut ev = Evaluator::default();
    for cc in &prog.complete_commands {
        ev.walk_list(cc);
    }
    ev.issues
}

#[derive(Default)]
struct Evaluator {
    issues: Vec<Issue>,
    seen: HashSet<String>,
    depth: usize,
}

impl Evaluator {
    fn push(&mut self, verdict: Verdict, message: String) {
        if self.seen.insert(message.clone()) {
            self.issues.push(Issue { verdict, message });
        }
    }

    fn walk_list(&mut self, list: &ast::CompoundList) {
        for item in &list.0 {
            self.walk_andor(&item.0);
        }
    }

    fn walk_andor(&mut self, andor: &ast::AndOrList) {
        self.walk_pipeline(&andor.first);
        for a in &andor.additional {
            match a {
                ast::AndOr::And(p) => self.walk_pipeline(p),
                ast::AndOr::Or(p) => {
                    self.check_or_fallback(p);
                    self.walk_pipeline(p);
                }
            }
        }
    }

    fn walk_pipeline(&mut self, pl: &ast::Pipeline) {
        let in_pipeline = pl.seq.len() > 1;
        for (i, cmd) in pl.seq.iter().enumerate() {
            self.walk_command(cmd, i > 0, in_pipeline);
        }
    }

    fn walk_command(&mut self, cmd: &ast::Command, right_of_pipe: bool, in_pipeline: bool) {
        match cmd {
            ast::Command::Simple(s) => {
                if let Some(ctx) = build_cmd(s, right_of_pipe, in_pipeline) {
                    self.apply_rules(&ctx);
                    self.recurse_local(&ctx.argv);
                }
            }
            ast::Command::Compound(cc, _) => self.walk_compound(cc),
            ast::Command::Function(f) => self.walk_compound(&f.body.0),
            ast::Command::ExtendedTest(_, _) => {}
        }
    }

    fn walk_compound(&mut self, cc: &ast::CompoundCommand) {
        use ast::CompoundCommand as C;
        match cc {
            C::BraceGroup(b) => self.walk_list(&b.list),
            C::Subshell(s) => self.walk_list(&s.list),
            C::ForClause(f) => self.walk_list(&f.body.list),
            C::WhileClause(w) | C::UntilClause(w) => {
                self.walk_list(&w.0);
                self.walk_list(&w.1.list);
            }
            C::IfClause(i) => {
                self.walk_list(&i.condition);
                self.walk_list(&i.then);
                if let Some(elses) = &i.elses {
                    for e in elses {
                        if let Some(cond) = &e.condition {
                            self.walk_list(cond);
                        }
                        self.walk_list(&e.body);
                    }
                }
            }
            C::CaseClause(c) => {
                for item in &c.cases {
                    if let Some(cmd) = &item.cmd {
                        self.walk_list(cmd);
                    }
                }
            }
            C::Arithmetic(_) | C::ArithmeticForClause(_) | C::Coprocess(_) => {}
        }
    }

    /// `|| true` / `|| :` swallowing a preceding status: the footer now reports
    /// exit, so this is unnecessary.
    fn check_or_fallback(&mut self, pl: &ast::Pipeline) {
        if pl.seq.len() != 1 {
            return;
        }
        if let ast::Command::Simple(s) = &pl.seq[0] {
            if let Some(w) = &s.word_or_name {
                if matches!(w.value.as_str(), "true" | ":") {
                    self.push(
                        Verdict::Warn,
                        "`|| true` / `|| :` swallows the exit code. guard's footer now reports \
                         it; let the real status surface."
                            .into(),
                    );
                }
            }
        }
    }

    /// Descend into a local shell payload (`bash -c '<script>'`) so discipline
    /// rules apply inside it too: `bash -c 'sudo ...'` must still block. ssh
    /// (remote) payloads are skipped - the rule messages assume this
    /// environment, and "sudo won't work here" is false on a remote host.
    fn recurse_local(&mut self, argv: &[String]) {
        if self.depth >= MAX_DEPTH {
            return;
        }
        let Some(Nested::Local(payload)) = nested_payload(argv) else {
            return;
        };
        let Some(prog) = parse_opt(&payload) else {
            return;
        };
        self.depth += 1;
        for cc in &prog.complete_commands {
            self.walk_list(cc);
        }
        self.depth -= 1;
    }

    fn apply_rules(&mut self, c: &Cmd) {
        self.rule_sudo(c);
        self.rule_pipe_to_shell(c);
        self.rule_git_stash(c);
        self.rule_cat(c);
        self.rule_find(c);
        self.rule_rg_replace(c);
        self.rule_dev_null(c);
        self.rule_echo_status(c);
    }

    fn rule_sudo(&mut self, c: &Cmd) {
        if c.name == "sudo" {
            self.push(
                Verdict::Block,
                "sudo will not work in this environment. If elevation is genuinely required, \
                 tell the user and stop."
                    .into(),
            );
        }
    }

    fn rule_pipe_to_shell(&mut self, c: &Cmd) {
        if !c.right_of_pipe || !matches!(c.name.as_str(), "bash" | "sh" | "zsh" | "fish") {
            return;
        }
        // `| bash -c '...'` is an explicit script, not remote-pipe-to-shell.
        if c.argv.iter().any(|a| a == "-c") {
            return;
        }
        self.push(
            Verdict::Block,
            "Do not pipe remote output into a shell. Download, inspect, then execute.".into(),
        );
    }

    fn rule_git_stash(&mut self, c: &Cmd) {
        if c.name != "git" {
            return;
        }
        if git_subcommand(&c.argv) != Some("stash") {
            return;
        }
        self.push(
            Verdict::Block,
            "STOP. `git stash` is BANNED in this environment. Do NOT use it, and do NOT try to \
             work around it with `git stash push/save`, plumbing equivalents, or any other \
             variation. This is a HARD RULE the user set deliberately. Do NOT touch the user's \
             git state to work around it: no commits, branches, resets, checkouts, or `git add`. \
             Leave the working tree exactly as it is and pick a different approach. If you truly \
             cannot proceed, STOP and ask the user."
                .into(),
        );
    }

    fn rule_cat(&mut self, c: &Cmd) {
        if c.name != "cat" || c.in_pipeline || !c.redirs.is_empty() || c.argv.len() != 2 {
            return;
        }
        let arg = &c.argv[1];
        if arg.starts_with('-') || arg.starts_with("<<") {
            return;
        }
        self.push(
            Verdict::Warn,
            "Prefer the Read tool over `cat <file>` - it gives line numbers and offset/limit."
                .into(),
        );
    }

    fn rule_find(&mut self, c: &Cmd) {
        if c.name != "find" || c.right_of_pipe {
            return;
        }
        if c.argv.iter().any(|a| matches!(a.as_str(), "-name" | "-iname" | "-type")) {
            self.push(
                Verdict::Warn,
                "Consider `fd` or `rg --files` over `find -name/-type` - faster and respects \
                 .gitignore."
                    .into(),
            );
        }
    }

    fn rule_rg_replace(&mut self, c: &Cmd) {
        if c.name != "rg" {
            return;
        }
        for a in c.argv.iter().skip(1) {
            if a == "--" {
                break;
            }
            // grep muscle memory: `-rN` bundles. ripgrep recurses by default and
            // `-r` is `--replace`, so `-rn` means "replace matches with 'n'".
            if a.len() > 2
                && a.starts_with("-r")
                && a[2..].chars().all(|ch| ch.is_ascii_alphabetic())
            {
                let repl = &a[2..];
                self.push(
                    Verdict::Warn,
                    format!(
                        "`rg {a}` is grep muscle memory: ripgrep recurses by default and `-r` is \
                         `--replace`, so this prints each match with the matched text replaced by \
                         \"{repl}\" instead of line numbers. Use `rg -n` for line numbers, or \
                         `rg --replace=TEXT` to actually replace."
                    ),
                );
                return;
            }
        }
    }

    fn rule_dev_null(&mut self, c: &Cmd) {
        let hides = c
            .redirs
            .iter()
            .any(|r| r.fd == 2 && r.kind == RedirKind::Write && r.target == "/dev/null");
        // Allow feature-detection: `command -v`/`which`/`type`/`hash`. `command`
        // is a transparent prefix, so also check the raw argv0.
        let head = basename(&c.argv[0]);
        let detect = |n: &str| matches!(n, "command" | "which" | "type" | "hash");
        if !hides || detect(&c.name) || detect(head) {
            return;
        }
        self.push(
            Verdict::Warn,
            "`2>/dev/null` hides the diagnosis when things break. Prefer letting errors surface."
                .into(),
        );
    }

    fn rule_echo_status(&mut self, c: &Cmd) {
        if c.name == "echo" && c.argv.iter().any(|a| a.contains("$?")) {
            self.push(
                Verdict::Warn,
                "`echo $?` is unnecessary - guard's footer already reports the exit code.".into(),
            );
        }
    }
}

/// Build a `Cmd` context from a simple command; None when it has no argv0.
fn build_cmd(s: &ast::SimpleCommand, right_of_pipe: bool, in_pipeline: bool) -> Option<Cmd> {
    let mut argv = Vec::new();
    let mut redirs = Vec::new();
    collect_items(s.prefix.as_ref().map(|p| &p.0), &mut argv, &mut redirs);
    if let Some(w) = &s.word_or_name {
        argv.push(w.value.clone());
    }
    collect_items(s.suffix.as_ref().map(|p| &p.0), &mut argv, &mut redirs);
    if argv.is_empty() {
        return None;
    }
    Some(Cmd {
        name: effective_name(&argv),
        argv,
        right_of_pipe,
        in_pipeline,
        redirs,
    })
}

/// The git subcommand, skipping global options that consume a following value
/// (`git -c k=v stash`, `git -C dir stash`). None when there is no subcommand.
fn git_subcommand(argv: &[String]) -> Option<&str> {
    const TAKES_ARG: &[&str] = &[
        "-c",
        "-C",
        "--git-dir",
        "--work-tree",
        "--namespace",
        "--super-prefix",
        "--config-env",
    ];
    let mut i = 1;
    while i < argv.len() {
        let a = &argv[i];
        if a.starts_with('-') {
            // `--git-dir=x` carries its value in one token; bare `-c` takes the next.
            i += if TAKES_ARG.contains(&a.as_str()) { 2 } else { 1 };
            continue;
        }
        return Some(a);
    }
    None
}

fn collect_items(
    items: Option<&Vec<ast::CommandPrefixOrSuffixItem>>,
    argv: &mut Vec<String>,
    redirs: &mut Vec<Redir>,
) {
    let Some(items) = items else { return };
    for it in items {
        match it {
            ast::CommandPrefixOrSuffixItem::Word(w) => argv.push(w.value.clone()),
            ast::CommandPrefixOrSuffixItem::AssignmentWord(a, _) => argv.push(format!("{a}")),
            ast::CommandPrefixOrSuffixItem::IoRedirect(io) => {
                if let Some(r) = describe_redir(io) {
                    redirs.push(r);
                }
            }
            ast::CommandPrefixOrSuffixItem::ProcessSubstitution(_, _) => {}
        }
    }
}

fn describe_redir(io: &ast::IoRedirect) -> Option<Redir> {
    use ast::{IoFileRedirectKind as K, IoFileRedirectTarget as T};
    match io {
        ast::IoRedirect::File(fd, kind, target) => {
            let (rk, default_fd) = match kind {
                K::Write | K::Clobber => (RedirKind::Write, 1),
                K::Append => (RedirKind::Append, 1),
                K::Read => (RedirKind::Read, 0),
                K::DuplicateOutput | K::DuplicateInput => (RedirKind::Dup, 1),
                K::ReadAndWrite => (RedirKind::Other, 0),
            };
            let tgt = match target {
                T::Filename(w) | T::Duplicate(w) => w.value.clone(),
                T::Fd(n) => n.to_string(),
                T::ProcessSubstitution(_, _) => String::new(),
            };
            Some(Redir {
                fd: fd.unwrap_or(default_fd),
                kind: rk,
                target: tgt,
            })
        }
        ast::IoRedirect::OutputAndError(w, append) => Some(Redir {
            fd: 1,
            kind: if *append { RedirKind::Append } else { RedirKind::Write },
            target: w.value.clone(),
        }),
        ast::IoRedirect::HereDocument(_, _) | ast::IoRedirect::HereString(_, _) => None,
    }
}

/// argv with transparent prefixes stripped, reduced to a basename.
fn effective_name(argv: &[String]) -> String {
    for a in argv {
        let base = basename(a);
        if !TRANSPARENT.contains(&base) {
            return base.to_string();
        }
    }
    argv.last().map(|a| basename(a).to_string()).unwrap_or_default()
}

#[cfg(test)]
mod tests;
