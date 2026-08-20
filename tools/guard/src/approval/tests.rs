use super::*;
use assert2::assert;
use rstest::*;

fn approval(allow: &[&str], deny: &[&str], ask: &[&str]) -> Approval {
    Approval {
        allow: allow.iter().map(|s| s.to_string()).collect(),
        deny: deny.iter().map(|s| s.to_string()).collect(),
        ask: ask.iter().map(|s| s.to_string()).collect(),
        cwd: None,
    }
}

#[rstest]
#[case::colon_star("Bash(cargo test:*)", Some("cargo test"))]
#[case::space_star("Bash(ls *)", Some("ls"))]
#[case::bare("Bash(git)", Some("git"))]
#[case::non_bash_tool("Read(*)", None)]
fn extract_prefix(#[case] input: &str, #[case] expected: Option<&str>) {
    assert!(extract_bash_prefix(input) == expected.map(str::to_string));
}

#[rstest]
#[case::stderr_merge("bq query foo 2>&1", "bq query foo")]
#[case::redirect_out("cmd > file", "cmd")]
#[case::redirect_devnull("cmd 2>/dev/null", "cmd")]
#[case::append("cmd a >> log", "cmd a")]
#[case::no_redirect("plain args here", "plain args here")]
fn strip_redirects_cases(#[case] input: &str, #[case] expected: &str) {
    assert!(strip_redirects(input) == expected);
}

#[rstest]
// Single, non-compound commands now go through the same decompose+match path
// as compound ones, not just a passthrough to Claude's own literal matcher.
#[case::single_command_now_allows(&["ls"], &[], &[], "ls -la", Decision::Allow)]
#[case::single_command_unknown_passes_through(&["ls"], &[], &[], "frobnicate -x", Decision::Passthrough)]
// The single-command path is what fixes env-var-prefixed invocations of an
// already-allowed tool: `psql`/`cargo run` are allowed, the leading
// assignment just used to make the whole call opaque outside a compound.
#[case::single_command_env_prefix_allows(&["cargo run"], &[], &[], "PORT=1 cargo run -p foo", Decision::Allow)]
// ... and `git -C dir <subcommand>` / `-c k=v`, which previously never
// matched a bare `git status` allow rule because the flags sat in between.
#[case::single_command_git_dash_c_allows(&["git status"], &[], &[], "git -C /tmp/repo status", Decision::Allow)]
#[case::single_command_git_dash_c_lowercase_allows(&["git log"], &[], &[], "git -c color.ui=always log", Decision::Allow)]
// A denied git subcommand behind -C must still deny, not silently allow.
#[case::single_command_git_dash_c_denies(&["git"], &["git push"], &[], "git -C /tmp/repo push", Decision::Deny)]
#[case::compound_all_allowed_is_allow(&["cd", "cargo test"], &[], &[], "cd crates && cargo test", Decision::Allow)]
#[case::compound_unknown_part_passes_through(&["cd"], &[], &[], "cd crates && frobnicate", Decision::Passthrough)]
#[case::denied_part_denies(&["ls"], &["rm"], &[], "ls && rm -rf x", Decision::Deny)]
// `tail` need not be allow-listed; the source command carries approval.
#[case::filters_are_trusted(&["cargo test"], &[], &[], "cargo test 2>&1 | tail -20", Decision::Allow)]
#[case::empty_allow_list_passes_through(&[], &["rm"], &[], "ls && cargo build", Decision::Passthrough)]
// A `timeout`/`flock` wrapper doesn't grant its own blanket allow (they're
// TRANSPARENT wrappers, not commands); the wrapped command decides.
#[case::timeout_wrapped_allowed_command_allows(&["cargo test"], &[], &[], "timeout 30 cargo test", Decision::Allow)]
#[case::timeout_wrapped_denied_command_denies(&["timeout"], &["rm"], &[], "timeout 5 rm -rf /", Decision::Deny)]
#[case::timeout_wrapped_unknown_command_passes_through(&["ls"], &[], &[], "timeout 5 rm -rf /", Decision::Passthrough)]
#[case::flock_wrapped_allowed_command_allows(&["cargo test"], &[], &[], "flock /tmp/x.lock cargo test", Decision::Allow)]
// bash -c payload is unquoted and classified: the inner command decides. The
// wrapper is transparent, so an all-allowed payload allows the compound.
#[case::bash_c_payload_allows(&["cd", "echo"], &[], &[], "cd x && bash -c 'echo hi'", Decision::Allow)]
#[case::bash_c_payload_denies(&["cd"], &["rm"], &[], "cd x && bash -c 'rm -rf /'", Decision::Deny)]
#[case::bash_c_payload_unknown_passes(&["cd"], &[], &[], "cd x && bash -c 'frobnicate'", Decision::Passthrough)]
// A payload we cannot expand keeps the wrapper opaque, so it cannot auto-allow.
#[case::bash_c_dollar_keeps_wrapper(&["cd"], &[], &[], "cd x && bash -c \"$UNKNOWN\"", Decision::Passthrough)]
// ssh is gated on its own: an all-allowed remote payload does NOT auto-allow
// the ssh (the wrapper stays unknown), but a denied remote command still denies.
#[case::ssh_payload_does_not_allow(&["cd", "ls"], &[], &[], "cd x && ssh roman 'ls'", Decision::Passthrough)]
#[case::ssh_denied_remote_denies(&["cd"], &["cargo clean"], &[], "cd x && ssh roman 'cargo clean'", Decision::Deny)]
fn decide_cases(
    #[case] allow: &[&str],
    #[case] deny: &[&str],
    #[case] ask: &[&str],
    #[case] cmd: &str,
    #[case] expected: Decision,
) {
    let a = approval(allow, deny, ask);
    assert!(a.decide(cmd) == expected);
}

#[test]
fn bash_c_payload_expanded() {
    let a = approval(&["echo"], &[], &[]);
    // Outer `bash -c` and inner `echo` both considered; unknown bash prefix
    // means passthrough unless bash is allowed too.
    let d = a.decide("bash -c 'echo hi' | cat");
    assert!(let (Decision::Passthrough | Decision::Allow) = d);
}

#[test]
fn path_invoked_binary_matches_basename_allow_rule() {
    // Resolve a real absolute path to `cat` the same way PATH search would,
    // so this doesn't hardcode a location that may not exist on every box.
    let path = std::env::split_paths(&std::env::var("PATH").unwrap())
        .map(|d| d.join("cat"))
        .find(|p| p.is_file())
        .expect("cat must exist on PATH for this test");
    let a = approval(&["cat"], &[], &[]);
    assert!(a.decide(&format!("{} foo.txt", path.display())) == Decision::Allow);
}

#[test]
fn path_invoked_binary_that_does_not_resolve_stays_unknown() {
    // basename "totally-fake-tool-xyz" doesn't resolve on PATH, so a same-
    // named allow rule can't be borrowed just by inventing a directory.
    let a = approval(&["totally-fake-tool-xyz"], &[], &[]);
    let d = a.decide("/tmp/nonexistent-dir/totally-fake-tool-xyz --flag");
    assert!(d == Decision::Passthrough);
}

#[rstest]
#[case::stripped_command("cargo test")]
#[case::transparent_prefixed("env cargo test")]
fn candidates_include(#[case] expected: &str) {
    let cands = candidates(&["env".into(), "cargo".into(), "test".into()]);
    assert!(cands.contains(&expected.to_string()));
}
