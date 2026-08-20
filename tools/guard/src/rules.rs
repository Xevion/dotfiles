//! Discipline rules: block dangerous commands, warn on wasteful ones. Walks the
//! full command tree (into compound bodies and function defs) so a banned
//! pattern can't hide inside a brace group or loop. Fail-open: a parse error
//! yields no issues, and the command runs untouched.

use crate::nested::{nested_payload, Nested};
use crate::parse::{basename, git_subcommand_index, parse_opt, skip_transparent};
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
        self.rule_find_root_scope(c);
        self.rule_find_unquoted_glob(c);
        self.rule_find_bare(c);
        self.rule_grep_recursive(c);
        self.rule_du_unbounded(c);
        self.rule_tree_unbounded(c);
        self.rule_locate(c);
        self.rule_rg_replace(c);
        self.rule_dev_null(c);
        self.rule_echo_status(c);
        self.rule_head_tail(c);
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

    /// `find /` (or another huge system root) with no `-maxdepth`: walks the
    /// whole tree, almost always slower and noisier than intended. A
    /// `-maxdepth` on the same root is treated as deliberate, not wasteful -
    /// any generous value (e.g. 20) is a cheap, always-available escape
    /// hatch, so blocking here doesn't cost real capability.
    fn rule_find_root_scope(&mut self, c: &Cmd) {
        if c.name != "find" {
            return;
        }
        let own = find_own_args(c);
        let paths = find_leading_paths(own);
        if paths.iter().any(|p| is_dangerous_find_root(p)) && !own.iter().any(|a| a == "-maxdepth")
        {
            self.push(
                Verdict::Block,
                format!(
                    "`find {}` scans without a depth limit from a large system directory - \
                     likely slow and mostly noise. Scope it to a project directory, add \
                     -maxdepth, or use `fd` if a system-wide search is genuinely needed.",
                    paths.join(" ")
                ),
            );
        }
    }

    /// `-name *.rs` (unquoted): if a local match exists, the shell glob-expands
    /// the pattern before `find` ever sees it, so `find` silently searches for
    /// one literal filename instead of matching the pattern. Detectable
    /// because brush-parser keeps quote characters in the raw word text.
    fn rule_find_unquoted_glob(&mut self, c: &Cmd) {
        if c.name != "find" {
            return;
        }
        const NAME_LIKE: &[&str] = &[
            "-name",
            "-iname",
            "-path",
            "-ipath",
            "-wholename",
            "-iwholename",
            "-regex",
            "-iregex",
        ];
        let own = find_own_args(c);
        for w in own.windows(2) {
            let (flag, val) = (w[0].as_str(), w[1].as_str());
            if !NAME_LIKE.contains(&flag) {
                continue;
            }
            let quoted = val.starts_with('\'') || val.starts_with('"');
            let has_glob = val.contains(['*', '?', '[']);
            if has_glob && !quoted {
                self.push(
                    Verdict::Warn,
                    format!(
                        "`{flag} {val}` looks unquoted - if a local match exists the shell \
                         glob-expands it before find runs, so find silently searches for one \
                         literal filename instead of the pattern. Quote it: `{flag} '{val}'`."
                    ),
                );
                return;
            }
        }
    }

    /// `find <path>` with no `-name`/`-type`/`-size`/etc.: lists every path
    /// under the tree with nothing to narrow it. `-maxdepth` alone counts as a
    /// deliberate bound, not a wasteful dump.
    fn rule_find_bare(&mut self, c: &Cmd) {
        if c.name != "find" {
            return;
        }
        const NON_FILTERING: &[&str] = &[
            "-print",
            "-print0",
            "-fprintf",
            "-depth",
            "-follow",
            "-H",
            "-L",
            "-P",
            "-D",
            "-daystart",
            "-noleaf",
            "-xdev",
            "-mount",
        ];
        let own = find_own_args(c);
        let flags = &own[find_leading_paths(own).len()..];
        if flags.iter().any(|a| a == "-maxdepth") {
            return;
        }
        let has_filter = flags
            .iter()
            .any(|a| a.starts_with('-') && !NON_FILTERING.contains(&a.as_str()));
        if !has_filter {
            self.push(
                Verdict::Warn,
                "Unscoped `find` with no -name/-type/-size (or similar) filter lists every path \
                 under the tree. Add a filter, cap depth with -maxdepth, or use `fd`/`rg \
                 --files` for a faster, .gitignore-aware listing."
                    .into(),
            );
        }
    }

    /// `grep -r`/`-R` (bare or bundled): unlike `rg`, plain grep ignores
    /// `.gitignore` and crawls `node_modules`/`.git`/build directories.
    fn rule_grep_recursive(&mut self, c: &Cmd) {
        if !matches!(c.name.as_str(), "grep" | "egrep" | "fgrep") {
            return;
        }
        for a in c.argv.iter().skip(1) {
            if a == "--" {
                break;
            }
            let exact = matches!(
                a.as_str(),
                "-r" | "-R" | "--recursive" | "--dereference-recursive"
            );
            let bundled = a.len() > 2
                && a.starts_with('-')
                && !a.starts_with("--")
                && a[1..].chars().all(|ch| ch.is_ascii_alphabetic())
                && a[1..].contains(['r', 'R']);
            if exact || bundled {
                self.push(
                    Verdict::Warn,
                    format!(
                        "`{} {a}` doesn't respect .gitignore and will crawl \
                         node_modules/.git/build directories. Use `rg` instead - same basic \
                         syntax, .gitignore-aware, much faster.",
                        c.name
                    ),
                );
                return;
            }
        }
    }

    /// `du` with no `-s`/`-d`: prints a line per subdirectory across the whole
    /// tree. The `disk-reclaim` skill already asks for `duc` instead of
    /// re-running `du`/`ncdu`; this enforces that at the command level.
    fn rule_du_unbounded(&mut self, c: &Cmd) {
        if c.name != "du" {
            return;
        }
        let bounded = c.argv.iter().skip(1).any(|a| {
            a == "-d"
                || a == "--summarize"
                || a.starts_with("--max-depth")
                || (a.starts_with('-')
                    && !a.starts_with("--")
                    && a[1..].chars().all(|ch| ch.is_ascii_alphabetic())
                    && a[1..].contains('s'))
        });
        if bounded {
            return;
        }
        self.push(
            Verdict::Warn,
            "Unbounded `du` prints a line per subdirectory - slow and noisy on a large tree. \
             Use `duc index` once and `duc ls`/`duc find` to query it, or add -s/-d N to bound \
             this."
                .into(),
        );
    }

    /// `tree` with no `-L`: prints the entire subtree.
    fn rule_tree_unbounded(&mut self, c: &Cmd) {
        if c.name != "tree" {
            return;
        }
        if c.argv.iter().skip(1).any(|a| a == "-L") {
            return;
        }
        self.push(
            Verdict::Warn,
            "Unbounded `tree` prints the entire subtree - add `-L N` to cap depth, or use `fd` \
             for a filtered listing."
                .into(),
        );
    }

    /// `locate`: reads a periodically-updated index, not a live filesystem
    /// walk, so it can miss files created or moved recently. A different
    /// failure mode than the other rules here (stale, not slow).
    fn rule_locate(&mut self, c: &Cmd) {
        if !matches!(c.name.as_str(), "locate" | "mlocate" | "plocate") {
            return;
        }
        self.push(
            Verdict::Warn,
            "`locate` reads a periodically-updated index and may miss files created or moved \
             recently. Use `fd`/`rg --files` for a live, accurate search."
                .into(),
        );
    }

    /// `rg` has no recursive flag at all, unlike grep, since it always
    /// recurses; `-r` is exclusively `--replace`. Any short-flag cluster
    /// with `r` in it is grep muscle memory (`-rn`, `-nr`, `-vr`, ...), not
    /// intentional replace usage, and the failure is silent: `r` swallows
    /// either the rest of the cluster or, if nothing follows it, the whole
    /// next argument (shifting pattern/path over by one - worse, since it
    /// can search the wrong directory and return empty results instead of
    /// erroring). Bare `-r VALUE` and `--replace=VALUE` are the unambiguous
    /// real forms and stay untouched. Fails open on any short flag rg
    /// doesn't actually have, rather than guessing.
    fn rule_rg_replace(&mut self, c: &Cmd) {
        if c.name != "rg" {
            return;
        }
        // ripgrep 15.1.0's full short-flag set (`rg --help`). Value-taking
        // flags consume the rest of the cluster, or the next argument if
        // nothing remains, and stop that cluster's parsing.
        const VALUE_FLAGS: &[char] = &[
            'e', 'f', 'E', 'm', 'j', 'g', 'd', 't', 'T', 'A', 'B', 'C', 'M', 'r',
        ];
        const BOOL_FLAGS: &[char] = &[
            'z', 's', 'F', 'i', 'v', 'x', 'U', 'P', 'S', 'a', 'w', 'L', 'u', 'b', 'h', 'n', 'N',
            'o', 'p', 'q', 'H', 'I', 'c', 'l', 'V',
        ];
        for a in c.argv.iter().skip(1) {
            if a == "--" {
                break;
            }
            // Long options (`--replace=...`) are unambiguous; only a
            // single-dash cluster can hide a bundled `-r`.
            if !a.starts_with('-') || a.starts_with("--") || a.len() < 2 {
                continue;
            }
            let body = &a[1..];
            // Bare `-r` alone (value as the next argument) is ripgrep's
            // documented form, not a typo.
            if body == "r" {
                continue;
            }
            for (idx, ch) in body.char_indices() {
                if ch == 'r' {
                    let remainder = &body[idx + 1..];
                    let message = if remainder.is_empty() {
                        format!(
                            "`rg {a}` is grep muscle memory: ripgrep has no recursive flag at \
                             all (it always recurses - there's no `-r`/`--recursive` to reach \
                             for), so `-r` here means `--replace`. Nothing follows `r` in this \
                             cluster, so ripgrep swallows your *next* argument whole as the \
                             replacement text, silently shifting your pattern and path arguments \
                             over by one - it can search the wrong directory (or none) and return \
                             misleadingly empty results instead of erroring. Split the flags \
                             apart (e.g. `rg -{prefix} -r TEXT ...`) or spell out \
                             `--replace=TEXT`.",
                            prefix = &body[..idx],
                        )
                    } else {
                        let shown = remainder.strip_prefix('=').unwrap_or(remainder);
                        format!(
                            "`rg {a}` is grep muscle memory: ripgrep has no recursive flag at \
                             all (it always recurses), so `-r` here means `--replace`. Everything \
                             after `r` in this cluster becomes the literal replacement text, so \
                             every match is silently swapped for \"{shown}\" instead of your \
                             other flags taking effect. Split the flags apart (`rg -n`, `rg -i`, \
                             ...) and only reach for `rg -r TEXT` (space-separated) or \
                             `rg --replace=TEXT` when you actually want to replace matched text."
                        )
                    };
                    self.push(Verdict::Block, message);
                    return;
                }
                if VALUE_FLAGS.contains(&ch) {
                    // A different value-taking flag claims the rest of this
                    // cluster (or the next token); `r` never gets a chance
                    // to mean `--replace` here.
                    break;
                }
                if !BOOL_FLAGS.contains(&ch) {
                    // Not a real rg short flag - rg will reject it itself.
                    break;
                }
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

    /// Piping into `head`/`tail` to peek at a command's own output: the Bash
    /// tool already returns it in full, so this just truncates what's visible
    /// and invites a second run with a bigger count.
    fn rule_head_tail(&mut self, c: &Cmd) {
        if !c.right_of_pipe || !matches!(c.name.as_str(), "head" | "tail") {
            return;
        }
        self.push(
            Verdict::Warn,
            format!(
                "Piping into `{}` to peek at output - the Bash tool already returns it in full; \
                 read it there, or get the command right once instead of re-running with a \
                 bigger count.",
                c.name
            ),
        );
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
    git_subcommand_index(argv).map(|i| argv[i].as_str())
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
            kind: if *append {
                RedirKind::Append
            } else {
                RedirKind::Write
            },
            target: w.value.clone(),
        }),
        ast::IoRedirect::HereDocument(_, _) | ast::IoRedirect::HereString(_, _) => None,
    }
}

/// `find`'s own argv: everything after the command word itself, skipping any
/// transparent prefix (`command find ...`, `nice find ...`).
fn find_own_args(c: &Cmd) -> &[String] {
    let idx = skip_transparent(&c.argv);
    let idx = if idx < c.argv.len() {
        idx
    } else {
        c.argv.len().saturating_sub(1)
    };
    &c.argv[idx + 1..]
}

/// The leading path operands of `find`'s own argv - tokens before the first
/// one starting with `-` (find's expression). Per find's grammar, paths
/// always precede the expression.
fn find_leading_paths(args: &[String]) -> &[String] {
    let end = args
        .iter()
        .position(|a| a.starts_with('-'))
        .unwrap_or(args.len());
    &args[..end]
}

/// Whether a `find` path operand is a large system directory worth flagging
/// when scanned without `-maxdepth`. Exact match only - `/home/xevion/proj` is
/// a normal scoped path, not a root scan.
fn is_dangerous_find_root(p: &str) -> bool {
    let trimmed = p.trim_end_matches('/');
    let trimmed = if trimmed.is_empty() { "/" } else { trimmed };
    matches!(
        trimmed,
        "/" | "/home" | "/usr" | "/var" | "/etc" | "/mnt" | "/root" | "~" | "$HOME" | "${HOME}"
    )
}

/// argv with transparent prefixes (and their own flags/positional args)
/// stripped, reduced to a basename.
fn effective_name(argv: &[String]) -> String {
    let idx = skip_transparent(argv);
    if idx < argv.len() {
        basename(&argv[idx]).to_string()
    } else {
        argv.last()
            .map(|a| basename(a).to_string())
            .unwrap_or_default()
    }
}

#[cfg(test)]
mod tests;
