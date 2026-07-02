//! Differential tests: a captured pipeline run through `guard run` must behave
//! identically to the same pipeline run through real bash. bash is the oracle;
//! any divergence is a guard bug. Cases labelled `bug_*` currently FAIL and
//! document a known behavioral-equivalence defect — they are intentional red
//! flags, not mistakes.

#![cfg(unix)]

use assert2::check;
use rstest::*;
use std::process::Command;

/// Run `pipeline` through bash and return (stdout, exit code). The oracle.
fn bash(pipeline: &str) -> (String, Option<i32>) {
    let out = Command::new("bash")
        .arg("-c")
        .arg(pipeline)
        .output()
        .expect("spawn bash");
    (norm(&String::from_utf8_lossy(&out.stdout)), out.status.code())
}

/// Run `pipeline` through `guard run`. The ambient `$SHELL` is left untouched:
/// guard reproduces fallback/compound stages with bash on its own, so the
/// differential suite would catch any regression back to honoring `$SHELL`. The
/// trailing `[guard] …` footer is stripped so stdout reflects only the pipeline
/// output.
fn guard(pipeline: &str) -> (String, Option<i32>) {
    guard_env(pipeline, &[])
}

/// As `guard`, but with extra environment variables set on the child.
fn guard_env(pipeline: &str, envs: &[(&str, &str)]) -> (String, Option<i32>) {
    let mut cmd = Command::new(env!("CARGO_BIN_EXE_guard"));
    cmd.args(["run", pipeline]);
    for (k, v) in envs {
        cmd.env(k, v);
    }
    let out = cmd.output().expect("spawn guard");
    let raw = String::from_utf8_lossy(&out.stdout);
    // The footer is the last thing printed and always begins with "[guard] ".
    let cut = match raw.rfind("[guard] ") {
        Some(i) => &raw[..i],
        None => &raw,
    };
    (norm(cut), out.status.code())
}

/// Trailing newlines are a footer-boundary artifact, not a behavior under test.
fn norm(s: &str) -> String {
    s.trim_end_matches('\n').to_string()
}

#[rstest]
// Equivalence should hold for these; they pass.
#[case::plain_filter("printf 'a\\nb\\nc\\n' | grep b")]
#[case::two_filters("printf 'a\\nb\\nc\\n' | grep -v a | head -1")]
#[case::quoted_space_arg("printf 'a b\\nc\\n' | grep 'a b'")]
#[case::empty_pattern("printf 'x\\ny\\n' | grep ''")]
#[case::escaped_space_arg("printf 'a b\\n' | grep a\\ b")]
#[case::stderr_merge_supported("sh -c 'echo E 1>&2' 2>&1 | grep E")]
#[case::same_size_transform("printf 'abc' | tr a x")]
#[case::delete_newlines("printf 'a\\nb\\n' | tr -d '\\n'")]
#[case::no_match_exit_code("printf 'a\\n' | grep zzz")]
#[case::narrowed_large("seq 1 1000 | head -3")]
#[case::infinite_source_early_exit("yes | head -1")]
#[case::noncapturable_fallback("FOO=bar sh -c 'echo $FOO' | grep bar")]
#[case::sed_print_line("printf 'a\\nb\\n' | sed -n '2p'")]
#[case::tr_ranges("printf 'A\\nB\\n' | tr 'A-Z' 'a-z'")]
#[case::sort_reorders("printf '3\\n1\\n2\\n' | sort")]
#[case::wc_lines("printf 'a\\nb\\n' | wc -l")]
#[case::cut_field("printf 'a b c\\n' | cut -d' ' -f2")]
#[case::three_stage("printf 'x\\ny\\n' | grep x | sed 's/x/X/'")]
#[case::equals_in_arg("printf 'a=b\\n' | grep =")]
#[case::stdin_redirect_source("sort < /etc/hostname | head -1")]
#[case::source_out_to_devnull("seq 1 5 > /dev/null | grep .")]
#[case::source_stderr_to_devnull("sh -c 'echo O; echo E 1>&2' 2>/dev/null | grep .")]
#[case::source_outerr_to_devnull("sh -c 'echo O; echo E 1>&2' &>/dev/null | grep .")]
#[case::herestring_fallback("grep o <<< 'foo' | head -1")]
#[case::multiple_assignments("A=1 B=2 sh -c 'echo x' | grep x")]
// Redirect forms the runner reproduces via its fd-table simulation (fds 0/1/2).
// `1>&2` on the source: stdout goes to stderr, the pipe stays empty, grep finds
// nothing (exit 1).
#[case::dup_stdout_to_stderr("echo hello 1>&2 | grep hello")]
// Ordering-sensitive: `2>&1 1>/dev/null` sends stderr to the current stdout (the
// pipe), then stdout to /dev/null. "ERR" must still reach grep.
#[case::redir_ordering("sh -c 'echo OUT; echo ERR 1>&2' 2>&1 1>/dev/null | grep -E 'OUT|ERR'")]
// Same dup, but on the sink stage: `grep x 1>&2` routes grep's stdout to stderr,
// leaving visible stdout empty.
#[case::dup_on_filter_stage("printf 'x\\n' | grep x 1>&2")]
// fd > 2 (a stdout/stderr swap via fd 3) is rejected by conv_redir, so the whole
// pipeline falls open to the shell unwrapped — still matching bash exactly, just
// without capture. "E" reaches grep.
#[case::fd_swap_falls_open("sh -c 'echo O; echo E 1>&2' 3>&1 1>&2 2>&3 | grep -E 'O|E'")]
fn guard_matches_bash(#[case] pipeline: &str) {
    let (want_out, want_rc) = bash(pipeline);
    let (got_out, got_rc) = guard(pipeline);
    check!(got_out == want_out, "stdout mismatch for {pipeline:?}");
    check!(got_rc == want_rc, "exit code mismatch for {pipeline:?}");
}

/// guard must reproduce fallback and compound stages with bash, never the
/// user's login `$SHELL`. Pinning a bogus `$SHELL` proves it is ignored: were it
/// honored, the stage would fail to spawn and produce no output. bash-syntax
/// commands run under fish/zsh would otherwise diverge.
#[rstest]
// Not capturable (`cat` is not a filter) -> whole-pipeline exec_fallback.
#[case::fallback("echo hi | cat")]
// Compound first stage -> run via the reproduction shell inside run_captured.
#[case::compound("{ echo hi; } | grep hi")]
fn ignores_login_shell(#[case] pipeline: &str) {
    let (out, rc) = guard_env(pipeline, &[("SHELL", "/nonexistent-shell-xyz")]);
    check!(out == "hi", "output for {pipeline:?}");
    check!(rc == Some(0), "exit code for {pipeline:?}");
}
