//! Nested-command extraction: pull the runnable payload out of a wrapper
//! command so discipline rules and approval can analyze what will actually run
//! instead of treating the wrapper as one opaque string.
//!
//! Two wrapper shapes are recognized:
//!   - a local shell script: `bash -c '<payload>'`, `sh -c`, `zsh -c`, ... The
//!     wrapper is transparent; the payload runs in this same environment.
//!   - a remote command: `ssh [flags] host <command...>`, `tailscale ssh ...`.
//!     The wrapper is itself gated, and ssh reassembles the trailing argv into
//!     one string for the remote shell, so quoting is flattened the same way.
//!
//! Every path fails open: an unrecognized wrapper, a payload we cannot unquote
//! (an unexpanded `$VAR`), or a missing operand yields None, and the caller
//! leaves the wrapper opaque rather than analyzing a guessed string.

use crate::parse::{basename, unquote_lenient, TRANSPARENT};

/// A command payload recovered from a wrapper.
#[derive(Debug, PartialEq, Eq)]
pub enum Nested {
    /// Runs locally in the same environment (`bash -c`, `sh -c`, ...). The
    /// wrapper adds nothing of its own, so callers analyze only the payload.
    Local(String),
    /// Runs on a remote host (`ssh`, `tailscale ssh`). The wrapper is gated on
    /// its own, so callers keep it and treat the payload as added surface.
    Remote(String),
}

/// Shells whose `-c` argument is a script to run in this environment.
const SHELLS: &[&str] = &["bash", "sh", "dash", "zsh", "ksh", "mksh", "ash", "fish"];

/// ssh short options that consume an argument (the rest of their cluster, or
/// the next token). Everything else is a boolean flag. Note `-c` here is the
/// cipher spec, not a command - ssh's remote command is a trailing operand.
const SSH_TAKES_ARG: &[char] = &[
    'B', 'b', 'c', 'D', 'E', 'e', 'F', 'I', 'i', 'J', 'L', 'l', 'm', 'O', 'o', 'p', 'Q', 'R', 'S',
    'W', 'w',
];

/// Extract the nested payload of `argv`, if it is a recognized wrapper.
pub fn nested_payload(argv: &[String]) -> Option<Nested> {
    let rest = strip_leading(argv);
    let base = basename(rest.first()?);
    if SHELLS.contains(&base) {
        return shell_c_payload(&rest[1..]).map(Nested::Local);
    }
    if base == "ssh" || base == "slogin" {
        return ssh_remote_command(&rest[1..]).map(Nested::Remote);
    }
    if base == "tailscale" && rest.get(1).map(String::as_str) == Some("ssh") {
        return ssh_remote_command(&rest[2..]).map(Nested::Remote);
    }
    None
}

/// Skip leading transparent prefixes (`env`, `nohup`, ...) and `NAME=value`
/// assignments so `env FOO=1 bash -c ...` still reaches `bash`.
fn strip_leading(argv: &[String]) -> &[String] {
    let mut i = 0;
    while i < argv.len() {
        let a = &argv[i];
        if TRANSPARENT.contains(&basename(a)) || is_assignment(a) {
            i += 1;
        } else {
            break;
        }
    }
    &argv[i..]
}

fn is_assignment(tok: &str) -> bool {
    let Some((name, _)) = tok.split_once('=') else {
        return false;
    };
    !name.is_empty()
        && name.chars().next().is_some_and(|c| c.is_ascii_alphabetic() || c == '_')
        && name.chars().all(|c| c.is_ascii_alphanumeric() || c == '_')
}

/// The script string of a shell `-c` invocation. `opts` is the argv after the
/// shell name; the command string is the token following the cluster that
/// carries `-c` (`-c`, `-lc`, `-ic`, ...), unquoted.
fn shell_c_payload(opts: &[String]) -> Option<String> {
    let mut i = 0;
    while i < opts.len() {
        let a = &opts[i];
        if is_c_flag(a) {
            return unquote_lenient(opts.get(i + 1)?);
        }
        // `-o`/`-O` (and the `+` forms) take the next token (`-O extglob`); it
        // is an option argument, not the script-file operand.
        if matches!(a.as_str(), "-o" | "+o" | "-O" | "+O") {
            i += 2;
            continue;
        }
        // The first non-option operand is the script file, after which `-c` is
        // no longer the command flag - so there is no inline command.
        if !a.starts_with('-') {
            return None;
        }
        i += 1;
    }
    None
}

/// A short-option cluster carrying the shell command flag: `-c`, or a cluster
/// of ascii letters containing `c` (`-lc`, `-ic`). Long options (`--rcfile`)
/// and anything non-alphabetic are excluded.
fn is_c_flag(tok: &str) -> bool {
    if tok.starts_with("--") {
        return false;
    }
    let Some(flags) = tok.strip_prefix('-') else {
        return false;
    };
    !flags.is_empty() && flags.chars().all(|c| c.is_ascii_alphabetic()) && flags.contains('c')
}

/// The remote command of an ssh invocation, reassembled the way ssh itself
/// does: join every operand after the host with a single space (ssh flattens
/// the trailing argv, dropping the local shell's quoting). `args` is the argv
/// after the `ssh` word.
fn ssh_remote_command(args: &[String]) -> Option<String> {
    let host = ssh_host_index(args)?;
    let tokens = &args[host + 1..];
    if tokens.is_empty() {
        return None;
    }
    let mut parts = Vec::with_capacity(tokens.len());
    for t in tokens {
        parts.push(unquote_lenient(t)?);
    }
    Some(parts.join(" "))
}

/// Index of the host operand: the first non-option token, skipping flags and
/// the arguments the arg-taking ones consume. None for an all-options argv (an
/// interactive session with no remote command to analyze).
fn ssh_host_index(args: &[String]) -> Option<usize> {
    let mut i = 0;
    while i < args.len() {
        let a = &args[i];
        if a == "--" {
            return (i + 1 < args.len()).then_some(i + 1);
        }
        let Some(flags) = a.strip_prefix('-') else {
            return Some(i);
        };
        if flags.is_empty() {
            return Some(i);
        }
        let cluster: Vec<char> = flags.chars().collect();
        let mut consumes_next = false;
        for (j, c) in cluster.iter().enumerate() {
            if SSH_TAKES_ARG.contains(c) {
                // The rest of the cluster is this option's glued argument;
                // if it ends the cluster, the next token is the argument.
                consumes_next = j + 1 == cluster.len();
                break;
            }
        }
        i += if consumes_next { 2 } else { 1 };
    }
    None
}

#[cfg(test)]
mod tests;
