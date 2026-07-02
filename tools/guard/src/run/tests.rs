use super::*;
use assert2::assert;
use rstest::*;

#[rstest]
#[case::sub_second(500, "500ms")]
#[case::just_under_a_second(999, "999ms")]
#[case::exactly_one_second(1000, "1.0s")]
#[case::one_and_a_half(1500, "1.5s")]
#[case::rounds_to_tenths(1234, "1.2s")]
#[case::zero(0, "0ms")]
fn fmt_dur_cases(#[case] millis: u64, #[case] expected: &str) {
    assert!(fmt_dur(Duration::from_millis(millis)) == expected);
}

#[rstest]
#[case::bytes(0, "0 B")]
#[case::bytes_just_under_kb(1023, "1023 B")]
#[case::exactly_one_kb(1024, "1 KB")]
#[case::kb_floors(1536, "1 KB")]
#[case::just_under_mb(1024 * 1024 - 1, "1023 KB")]
#[case::exactly_one_mb(1024 * 1024, "1.0 MB")]
#[case::mb_tenths(5 * 1024 * 1024 + 512 * 1024, "5.5 MB")]
fn fmt_size_cases(#[case] bytes: u64, #[case] expected: &str) {
    assert!(fmt_size(bytes) == expected);
}

#[rstest]
#[case::sigpipe(13, " (SIGPIPE, cut by a downstream filter)")]
#[case::sigkill(9, " (killed by signal 9)")]
#[case::sigterm(15, " (killed by signal 15)")]
fn signal_hint_cases(#[case] sig: i32, #[case] expected: &str) {
    assert!(signal_hint(sig) == expected);
}

/// A source label is the argv0 basename for a simple stage, and a fixed marker
/// for a compound one.
#[rstest]
#[case::path_stripped(Stage::Simple {
    assignments: vec![],
    argv: vec!["/usr/bin/cargo".into(), "test".into()],
    redirs: vec![],
}, "cargo")]
#[case::bare(Stage::Simple {
    assignments: vec![],
    argv: vec!["ls".into()],
    redirs: vec![],
}, "ls")]
#[case::compound(Stage::Compound { text: "{ ls; }".into() }, "{...}")]
#[case::unsupported(Stage::Unsupported, "?")]
fn source_label_cases(#[case] stage: Stage, #[case] expected: &str) {
    assert!(source_label(&stage) == expected);
}
