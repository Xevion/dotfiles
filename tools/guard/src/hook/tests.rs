use super::*;
use std::process::Command;

const SELF: &str = "/opt/guard";

fn rw(cmd: &str) -> Rewrite {
    Rewrite::apply(cmd, SELF)
}

#[test]
fn wraps_simple_pipeline() {
    let r = rw("ls -la | head -5");
    assert_eq!(r.count, 1);
    assert_eq!(r.command, "/opt/guard run 'ls -la | head -5'");
}

#[test]
fn preserves_prefix_and_suffix_bytes() {
    // `cd` stays outside; only the pipeline span is replaced.
    let r = rw("cd crates && ls foo 2>&1 | grep x | tail");
    assert_eq!(r.count, 1);
    assert_eq!(
        r.command,
        "cd crates && /opt/guard run 'ls foo 2>&1 | grep x | tail'"
    );
}

#[test]
fn wraps_each_pipeline_independently() {
    let r = rw("ls | head; ls -la | tail");
    assert_eq!(r.count, 2);
    assert_eq!(r.command, "/opt/guard run 'ls | head'; /opt/guard run 'ls -la | tail'");
}

#[test]
fn no_capture_leaves_command_untouched() {
    let r = rw("ls -la");
    assert_eq!(r.count, 0);
    assert_eq!(r.command, "ls -la");
}

#[test]
fn idempotent_on_already_wrapped() {
    let once = rw("ls | head");
    let twice = rw(&once.command);
    assert_eq!(twice.count, 0);
    assert_eq!(twice.command, once.command);
}

#[test]
fn single_quote_basic() {
    assert_eq!(single_quote("abc"), "'abc'");
    assert_eq!(single_quote("a'b"), "'a'\\''b'");
}

/// The escaped word, evaluated by a real shell, must reproduce the original
/// pipeline text byte-for-byte. This is the one text transform in the system.
#[test]
fn single_quote_roundtrips_through_sh() {
    let samples = [
        "ls | head",
        "grep 'a b' file | tail -5",
        "awk '{print $1}' x | head", // has $ but quoting must still be exact
        "sed \"s/a/b/\" f | uniq",
        "echo it's | cat",
        "printf '%s\\n' a | head",
        "grep -e 'x'\\''y' f | wc -l",
        "café | grep é",
        "a | b | c | d",
        "tr -d '\\n' < f | head",
    ];
    for s in samples {
        let quoted = single_quote(s);
        // printf %s <quoted> should emit exactly s.
        let out = Command::new("sh")
            .arg("-c")
            .arg(format!("printf %s {quoted}"))
            .output()
            .expect("run sh");
        assert!(out.status.success(), "sh failed for {s:?}");
        assert_eq!(
            String::from_utf8_lossy(&out.stdout),
            s,
            "roundtrip mismatch for {s:?} (quoted: {quoted})"
        );
    }
}

#[test]
fn output_json_shapes() {
    let mut o = Output::default();
    assert_eq!(o.to_json(), None);

    o.permission_allow = true;
    let j: serde_json::Value = serde_json::from_str(&o.to_json().unwrap()).unwrap();
    assert_eq!(j["hookSpecificOutput"]["permissionDecision"], "allow");
    assert_eq!(j["hookSpecificOutput"]["hookEventName"], "PreToolUse");

    o.updated_command = Some("x run 'a | b'".into());
    o.additional_context.push("note".into());
    let j: serde_json::Value = serde_json::from_str(&o.to_json().unwrap()).unwrap();
    assert_eq!(j["hookSpecificOutput"]["updatedInput"]["command"], "x run 'a | b'");
    assert_eq!(j["hookSpecificOutput"]["additionalContext"], "note");
}
