use super::*;
use assert2::{assert, check};
use rstest::*;
use std::process::Command;

const SELF: &str = "/opt/guard";

fn rw(cmd: &str) -> Rewrite {
    Rewrite::apply(cmd, SELF)
}

#[rstest]
#[case::wraps_simple_pipeline("ls -la | head -5", 1, "/opt/guard run 'ls -la | head -5'")]
// `cd` stays outside; only the pipeline span is replaced.
#[case::preserves_prefix_and_suffix_bytes(
    "cd crates && ls foo 2>&1 | grep x | tail",
    1,
    "cd crates && /opt/guard run 'ls foo 2>&1 | grep x | tail'"
)]
#[case::wraps_each_pipeline_independently(
    "ls | head; ls -la | tail",
    2,
    "/opt/guard run 'ls | head'; /opt/guard run 'ls -la | tail'"
)]
#[case::no_capture_leaves_command_untouched("ls -la", 0, "ls -la")]
fn rewrite_cases(#[case] input: &str, #[case] count: usize, #[case] expected: &str) {
    let r = rw(input);
    check!(r.count == count);
    check!(r.command == expected);
}

#[test]
fn idempotent_on_already_wrapped() {
    let once = rw("ls | head");
    let twice = rw(&once.command);
    check!(twice.count == 0);
    check!(twice.command == once.command);
}

#[rstest]
#[case::plain("abc", "'abc'")]
#[case::embedded_quote("a'b", "'a'\\''b'")]
fn single_quote_basic(#[case] input: &str, #[case] expected: &str) {
    assert!(single_quote(input) == expected);
}

/// The escaped word, evaluated by a real shell, must reproduce the original
/// pipeline text byte-for-byte. This is the one text transform in the system.
#[rstest]
#[case::simple_pipe("ls | head")]
#[case::quoted_arg("grep 'a b' file | tail -5")]
#[case::dollar_in_single_quotes("awk '{print $1}' x | head")]
#[case::double_quoted_arg("sed \"s/a/b/\" f | uniq")]
#[case::apostrophe("echo it's | cat")]
#[case::printf_format("printf '%s\\n' a | head")]
#[case::escaped_single_quote("grep -e 'x'\\''y' f | wc -l")]
#[case::non_ascii("café | grep é")]
#[case::multi_stage_pipe("a | b | c | d")]
#[case::redirect_input("tr -d '\\n' < f | head")]
fn single_quote_roundtrips_through_sh(#[case] sample: &str) {
    let quoted = single_quote(sample);
    // printf %s <quoted> should emit exactly sample.
    let out = Command::new("sh")
        .arg("-c")
        .arg(format!("printf %s {quoted}"))
        .output()
        .expect("run sh");
    assert!(out.status.success(), "sh failed for {sample:?}");
    assert!(
        String::from_utf8_lossy(&out.stdout) == sample,
        "roundtrip mismatch for {sample:?} (quoted: {quoted})"
    );
}

#[test]
fn output_json_shapes() {
    let mut o = Output::default();
    assert!(let None = o.to_json());

    o.permission_allow = true;
    assert!(let Some(s) = o.to_json());
    let j: serde_json::Value = serde_json::from_str(&s).unwrap();
    assert!(j["hookSpecificOutput"]["permissionDecision"] == "allow");
    assert!(j["hookSpecificOutput"]["hookEventName"] == "PreToolUse");

    o.updated_command = Some("x run 'a | b'".into());
    o.additional_context.push("note".into());
    assert!(let Some(s) = o.to_json());
    let j: serde_json::Value = serde_json::from_str(&s).unwrap();
    assert!(j["hookSpecificOutput"]["updatedInput"]["command"] == "x run 'a | b'");
    assert!(j["hookSpecificOutput"]["additionalContext"] == "note");
}
