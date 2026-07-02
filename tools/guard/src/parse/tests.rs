use super::*;

fn pls(cmd: &str) -> Vec<PipelineInfo> {
    PipelineInfo::extract(cmd).expect("parse")
}

/// The single capturable pipeline in `cmd`, or panic.
fn only_capturable(cmd: &str) -> PipelineInfo {
    let mut caps: Vec<PipelineInfo> = pls(cmd).into_iter().filter(|p| p.capturable()).collect();
    assert_eq!(caps.len(), 1, "expected exactly one capturable pipeline in {cmd:?}");
    caps.pop().unwrap()
}

fn none_capturable(cmd: &str) {
    assert!(
        !pls(cmd).iter().any(|p| p.capturable()),
        "expected no capturable pipeline in {cmd:?}"
    );
}

#[test]
fn simple_filter_is_capturable() {
    let pl = only_capturable("ls -la | head -5");
    assert_eq!(pl.stages.len(), 2);
    assert_eq!(pl.stages[0].argv0(), Some("ls"));
    assert_eq!(pl.stages[1].argv0(), Some("head"));
}

#[test]
fn two_filters_capturable() {
    let pl = only_capturable("ls | grep foo | head");
    assert_eq!(pl.stages.len(), 3);
}

#[test]
fn single_stage_not_capturable() {
    none_capturable("ls -la");
}

#[test]
fn no_filter_stage_not_capturable() {
    // `cat` is not a recognized filter; nothing to narrow.
    none_capturable("ls | cat");
}

#[test]
fn unresolvable_argv0_not_capturable() {
    // Alias/function shield (predicate rule 4).
    none_capturable("definitely_not_a_real_binary_xyz | head");
}

#[test]
fn follow_not_capturable() {
    none_capturable("cat file | tail -f");
    none_capturable("tail -f log | grep err");
}

#[test]
fn background_not_capturable() {
    none_capturable("ls | head &");
}

#[test]
fn negation_and_time_not_capturable() {
    none_capturable("! ls | grep x");
    none_capturable("time ls | grep x");
}

#[test]
fn dollar_expansion_excluded() {
    none_capturable("ls $HOME | head");
    none_capturable("echo \"$PWD\" | grep x | head");
    none_capturable("cmd `date` | head");
}

#[test]
fn compound_first_stage_capturable() {
    let pl = only_capturable("{ echo hdr; ls; } | grep foo");
    assert!(matches!(pl.stages[0], Stage::Compound { .. }));
    assert_eq!(pl.stages[1].argv0(), Some("grep"));
}

#[test]
fn compound_later_stage_not_capturable() {
    none_capturable("ls | { grep foo; }");
}

#[test]
fn cd_stays_outside_wrapped_span() {
    // Scenario 4: only the pipeline is captured; `cd` is a separate statement.
    let caps: Vec<_> = pls("cd crates && ls -la | head -5")
        .into_iter()
        .filter(|p| p.capturable())
        .collect();
    assert_eq!(caps.len(), 1);
    assert_eq!(caps[0].text, "ls -la | head -5");
}

#[test]
fn multi_statement_yields_two_captures() {
    // Scenario 5: one guard per pipeline.
    let caps: Vec<_> = pls("ls src | head; ls -la | tail")
        .into_iter()
        .filter(|p| p.capturable())
        .collect();
    assert_eq!(caps.len(), 2);
    assert_eq!(caps[0].text, "ls src | head");
    assert_eq!(caps[1].text, "ls -la | tail");
}

#[test]
fn byte_span_slices_original_exactly() {
    let cmd = "cd x && ls -la | grep foo | head -3";
    let pl = only_capturable(cmd);
    let (s, e) = pl.byte_span;
    assert_eq!(&cmd[s..e], "ls -la | grep foo | head -3");
    assert_eq!(pl.text, &cmd[s..e]);
}

#[test]
fn redirect_dup_extracted() {
    // `2>&1` on the source is modeled as a Dup on the first stage.
    let pl = only_capturable("ls foo 2>&1 | grep bar | tail");
    match &pl.stages[0] {
        Stage::Simple { redirs, .. } => {
            assert!(redirs.contains(&Redir::Dup { from: 2, to: 1 }), "got {redirs:?}");
        }
        s => panic!("expected simple stage, got {s:?}"),
    }
}

#[test]
fn env_assignment_becomes_stage_env() {
    let pl = only_capturable("FOO=bar ls | head");
    match &pl.stages[0] {
        Stage::Simple { assignments, argv, .. } => {
            assert_eq!(assignments, &[("FOO".to_string(), "bar".to_string())]);
            assert_eq!(argv[0], "ls");
        }
        s => panic!("expected simple stage, got {s:?}"),
    }
}

/// argv of a simple stage, or panic.
fn argv_of(stage: &Stage) -> &[String] {
    match stage {
        Stage::Simple { argv, .. } => argv,
        s => panic!("expected simple stage, got {s:?}"),
    }
}

#[test]
fn single_quoted_arg_is_unquoted() {
    // The regression: `-E 'binary(roundtrip)'` must reach the child without its
    // quotes, since the runner spawns the stage with no shell to strip them.
    let pl = only_capturable("cargo nextest run -E 'binary(roundtrip)' | tail -25");
    assert_eq!(
        argv_of(&pl.stages[0]),
        &["cargo", "nextest", "run", "-E", "binary(roundtrip)"]
    );
}

#[test]
fn double_quoted_arg_is_unquoted() {
    let pl = only_capturable("cargo nextest run -E \"binary(roundtrip)\" | tail");
    assert_eq!(argv_of(&pl.stages[0]).last().unwrap(), "binary(roundtrip)");
}

#[test]
fn quoted_arg_with_space_is_one_word() {
    let pl = only_capturable("ls | grep 'foo bar'");
    assert_eq!(argv_of(&pl.stages[1]), &["grep", "foo bar"]);
}

#[test]
fn adjacent_quote_concatenation() {
    // `pre'mid'post` is one word after quote removal.
    let pl = only_capturable("ls | grep pre'mid'post");
    assert_eq!(argv_of(&pl.stages[1]), &["grep", "premidpost"]);
}

#[test]
fn quoted_assignment_value_is_unquoted() {
    let pl = only_capturable("DB='a b' ls | head");
    match &pl.stages[0] {
        Stage::Simple { assignments, .. } => {
            assert_eq!(assignments, &[("DB".to_string(), "a b".to_string())]);
        }
        s => panic!("expected simple stage, got {s:?}"),
    }
}

#[test]
fn unquoted_glob_not_capturable() {
    // The runner can't glob-expand; fail open so it runs unwrapped.
    none_capturable("ls *.rs | head");
    none_capturable("ls | grep foo?bar");
}

#[test]
fn tilde_not_capturable() {
    // Tilde expansion needs the shell; fail open.
    none_capturable("cat ~/file | head");
}

#[test]
fn quoted_glob_is_literal_and_capturable() {
    // Inside quotes the glob is literal, so the stage is reproducible.
    let pl = only_capturable("ls | grep '*.rs'");
    assert_eq!(argv_of(&pl.stages[1]), &["grep", "*.rs"]);
}

#[test]
fn process_substitution_not_capturable() {
    none_capturable("diff <(ls a) b | head");
}

#[test]
fn non_ascii_byte_span() {
    // char index != byte offset; the conversion table must hold.
    let cmd = "echo café | grep é";
    let pl = only_capturable(cmd);
    let (s, e) = pl.byte_span;
    assert_eq!(&cmd[s..e], cmd); // whole thing is one pipeline
    assert_eq!(pl.stages.len(), 2);
}
