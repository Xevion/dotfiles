use super::*;

fn approval(allow: &[&str], deny: &[&str], ask: &[&str]) -> Approval {
    Approval {
        allow: allow.iter().map(|s| s.to_string()).collect(),
        deny: deny.iter().map(|s| s.to_string()).collect(),
        ask: ask.iter().map(|s| s.to_string()).collect(),
    }
}

#[test]
fn extract_prefix_forms() {
    assert_eq!(extract_bash_prefix("Bash(cargo test:*)"), Some("cargo test".into()));
    assert_eq!(extract_bash_prefix("Bash(ls *)"), Some("ls".into()));
    assert_eq!(extract_bash_prefix("Bash(git)"), Some("git".into()));
    assert_eq!(extract_bash_prefix("Read(*)"), None);
}

#[test]
fn strip_redirects_forms() {
    assert_eq!(strip_redirects("bq query foo 2>&1"), "bq query foo");
    assert_eq!(strip_redirects("cmd > file"), "cmd");
    assert_eq!(strip_redirects("cmd 2>/dev/null"), "cmd");
    assert_eq!(strip_redirects("cmd a >> log"), "cmd a");
    assert_eq!(strip_redirects("plain args here"), "plain args here");
}

#[test]
fn non_compound_passes_through() {
    let a = approval(&["ls"], &[], &[]);
    assert_eq!(a.decide("ls -la"), Decision::Passthrough);
}

#[test]
fn compound_all_allowed_is_allow() {
    let a = approval(&["cd", "cargo test"], &[], &[]);
    assert_eq!(a.decide("cd crates && cargo test"), Decision::Allow);
}

#[test]
fn compound_unknown_part_passes_through() {
    let a = approval(&["cd"], &[], &[]);
    assert_eq!(a.decide("cd crates && frobnicate"), Decision::Passthrough);
}

#[test]
fn denied_part_denies() {
    let a = approval(&["ls"], &["rm"], &[]);
    assert_eq!(a.decide("ls && rm -rf x"), Decision::Deny);
}

#[test]
fn filters_are_trusted() {
    // `tail` need not be allow-listed; the source command carries approval.
    let a = approval(&["cargo test"], &[], &[]);
    assert_eq!(a.decide("cargo test 2>&1 | tail -20"), Decision::Allow);
}

#[test]
fn bash_c_payload_expanded() {
    let a = approval(&["echo"], &[], &[]);
    // Outer `bash -c` and inner `echo` both considered; unknown bash prefix
    // means passthrough unless bash is allowed too.
    let d = a.decide("bash -c 'echo hi' | cat");
    assert!(matches!(d, Decision::Passthrough | Decision::Allow));
}

#[test]
fn empty_allow_list_passes_through() {
    let a = approval(&[], &["rm"], &[]);
    assert_eq!(a.decide("ls && cargo build"), Decision::Passthrough);
}

#[test]
fn candidates_include_transparent_stripped() {
    let cands = candidates(&["env".into(), "cargo".into(), "test".into()]);
    assert!(cands.contains(&"cargo test".to_string()));
    assert!(cands.contains(&"env cargo test".to_string()));
}
