use super::*;
use assert2::{assert, check};
use rstest::*;

fn pls(cmd: &str) -> Vec<PipelineInfo> {
    PipelineInfo::extract(cmd).expect("parse")
}

/// The single capturable pipeline in `cmd`, or panic.
fn only_capturable(cmd: &str) -> PipelineInfo {
    let mut caps: Vec<PipelineInfo> = pls(cmd).into_iter().filter(|p| p.capturable()).collect();
    assert!(caps.len() == 1, "expected exactly one capturable pipeline in {cmd:?}");
    caps.pop().unwrap()
}

fn none_capturable(cmd: &str) {
    assert!(
        !pls(cmd).iter().any(|p| p.capturable()),
        "expected no capturable pipeline in {cmd:?}"
    );
}

/// argv of a simple stage, or panic.
fn argv_of(stage: &Stage) -> &[String] {
    assert!(let Stage::Simple { argv, .. } = stage);
    argv
}

#[test]
fn simple_filter_is_capturable() {
    let pl = only_capturable("ls -la | head -5");
    check!(pl.stages.len() == 2);
    check!(pl.stages[0].argv0() == Some("ls"));
    check!(pl.stages[1].argv0() == Some("head"));
}

#[test]
fn two_filters_capturable() {
    let pl = only_capturable("ls | grep foo | head");
    assert!(pl.stages.len() == 3);
}

#[rstest]
#[case::single_stage("ls -la")]
#[case::no_filter_stage("ls | cat")] // `cat` is not a recognized filter; nothing to narrow.
#[case::unresolvable_argv0("definitely_not_a_real_binary_xyz | head")] // Alias/function shield (predicate rule 4).
#[case::follow_source("cat file | tail -f")]
#[case::follow_sink("tail -f log | grep err")]
#[case::background("ls | head &")]
#[case::negation("! ls | grep x")]
#[case::time_prefix("time ls | grep x")]
#[case::dollar_var("ls $HOME | head")] // `$` expansion excluded
#[case::dollar_quoted_var("echo \"$PWD\" | grep x | head")] // `$` expansion excluded
#[case::dollar_backtick("cmd `date` | head")] // `$` expansion excluded
#[case::compound_later_stage("ls | { grep foo; }")]
#[case::process_substitution("diff <(ls a) b | head")]
#[case::unquoted_glob_star("ls *.rs | head")] // runner can't glob-expand; fail open so it runs unwrapped
#[case::unquoted_glob_question("ls | grep foo?bar")] // runner can't glob-expand; fail open so it runs unwrapped
#[case::tilde("cat ~/file | head")] // tilde expansion needs the shell; fail open
fn not_capturable(#[case] cmd: &str) {
    none_capturable(cmd);
}

#[test]
fn compound_first_stage_capturable() {
    let pl = only_capturable("{ echo hdr; ls; } | grep foo");
    assert!(let Stage::Compound { .. } = &pl.stages[0]);
    assert!(pl.stages[1].argv0() == Some("grep"));
}

#[test]
fn cd_stays_outside_wrapped_span() {
    // Scenario 4: only the pipeline is captured; `cd` is a separate statement.
    let caps: Vec<_> = pls("cd crates && ls -la | head -5")
        .into_iter()
        .filter(|p| p.capturable())
        .collect();
    assert!(caps.len() == 1);
    assert!(caps[0].text == "ls -la | head -5");
}

#[test]
fn multi_statement_yields_two_captures() {
    // Scenario 5: one guard per pipeline.
    let caps: Vec<_> = pls("ls src | head; ls -la | tail")
        .into_iter()
        .filter(|p| p.capturable())
        .collect();
    check!(caps.len() == 2);
    check!(caps[0].text == "ls src | head");
    check!(caps[1].text == "ls -la | tail");
}

#[test]
fn byte_span_slices_original_exactly() {
    let cmd = "cd x && ls -la | grep foo | head -3";
    let pl = only_capturable(cmd);
    let (s, e) = pl.byte_span;
    check!(&cmd[s..e] == "ls -la | grep foo | head -3");
    check!(pl.text == &cmd[s..e]);
}

#[test]
fn redirect_dup_extracted() {
    // `2>&1` on the source is modeled as a Dup on the first stage.
    let pl = only_capturable("ls foo 2>&1 | grep bar | tail");
    assert!(let Stage::Simple { redirs, .. } = &pl.stages[0]);
    assert!(redirs.contains(&Redir::Dup { from: 2, to: 1 }), "got {redirs:?}");
}

#[test]
fn env_assignment_becomes_stage_env() {
    let pl = only_capturable("FOO=bar ls | head");
    assert!(let Stage::Simple { assignments, argv, .. } = &pl.stages[0]);
    check!(assignments == &[("FOO".to_string(), "bar".to_string())]);
    check!(argv[0] == "ls");
}

#[rstest]
// The regression: `-E 'binary(roundtrip)'` must reach the child without its
// quotes, since the runner spawns the stage with no shell to strip them.
// Both quote styles below must unquote to the identical argv.
#[case::single_quoted(
    "cargo nextest run -E 'binary(roundtrip)' | tail -25",
    0,
    &["cargo", "nextest", "run", "-E", "binary(roundtrip)"]
)]
#[case::double_quoted(
    "cargo nextest run -E \"binary(roundtrip)\" | tail",
    0,
    &["cargo", "nextest", "run", "-E", "binary(roundtrip)"]
)]
#[case::space_in_quotes("ls | grep 'foo bar'", 1, &["grep", "foo bar"])]
// `pre'mid'post` is one word after quote removal.
#[case::adjacent_quote_concatenation("ls | grep pre'mid'post", 1, &["grep", "premidpost"])]
// Inside quotes the glob is literal, so the stage is reproducible.
#[case::quoted_glob_is_literal("ls | grep '*.rs'", 1, &["grep", "*.rs"])]
fn argv_after_unquoting(#[case] cmd: &str, #[case] stage_idx: usize, #[case] expected: &[&str]) {
    let pl = only_capturable(cmd);
    assert!(argv_of(&pl.stages[stage_idx]) == expected);
}

#[test]
fn quoted_assignment_value_is_unquoted() {
    let pl = only_capturable("DB='a b' ls | head");
    assert!(let Stage::Simple { assignments, .. } = &pl.stages[0]);
    assert!(assignments == &[("DB".to_string(), "a b".to_string())]);
}

#[test]
fn non_ascii_byte_span() {
    // char index != byte offset; the conversion table must hold.
    let cmd = "echo café | grep é";
    let pl = only_capturable(cmd);
    let (s, e) = pl.byte_span;
    check!(&cmd[s..e] == cmd); // whole thing is one pipeline
    check!(pl.stages.len() == 2);
}
