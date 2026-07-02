use super::*;
use assert2::assert;
use rstest::*;

fn approval(allow: &[&str], deny: &[&str], ask: &[&str]) -> Approval {
    Approval {
        allow: allow.iter().map(|s| s.to_string()).collect(),
        deny: deny.iter().map(|s| s.to_string()).collect(),
        ask: ask.iter().map(|s| s.to_string()).collect(),
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
#[case::non_compound_passes_through(&["ls"], &[], &[], "ls -la", Decision::Passthrough)]
#[case::compound_all_allowed_is_allow(&["cd", "cargo test"], &[], &[], "cd crates && cargo test", Decision::Allow)]
#[case::compound_unknown_part_passes_through(&["cd"], &[], &[], "cd crates && frobnicate", Decision::Passthrough)]
#[case::denied_part_denies(&["ls"], &["rm"], &[], "ls && rm -rf x", Decision::Deny)]
// `tail` need not be allow-listed; the source command carries approval.
#[case::filters_are_trusted(&["cargo test"], &[], &[], "cargo test 2>&1 | tail -20", Decision::Allow)]
#[case::empty_allow_list_passes_through(&[], &["rm"], &[], "ls && cargo build", Decision::Passthrough)]
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

#[rstest]
#[case::stripped_command("cargo test")]
#[case::transparent_prefixed("env cargo test")]
fn candidates_include(#[case] expected: &str) {
    let cands = candidates(&["env".into(), "cargo".into(), "test".into()]);
    assert!(cands.contains(&expected.to_string()));
}
