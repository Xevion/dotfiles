use super::*;
use assert2::{assert, check};
use rstest::*;

#[rstest]
#[case::nothing_dropped_writes_no_file(b"hello\nworld\n".as_slice(), 12, 12, 2, false)]
#[case::dropped_data_writes_file(b"a\nb\nc\nd\n".as_slice(), 4, 8, 4, true)]
#[case::trailing_unterminated_line_counts(b"one\ntwo".as_slice(), 0, 7, 2, true)]
fn sink_accounting(
    #[case] input: &[u8],
    #[case] final_bytes: u64,
    #[case] bytes: u64,
    #[case] lines: u64,
    #[case] spilled: bool,
) {
    let mut s = Sink::new(std::process::id());
    s.push(input);
    let cap = s.finish(final_bytes);
    check!(cap.bytes == bytes);
    check!(cap.lines == lines);
    check!(cap.path.is_some() == spilled);
    if let Some(p) = cap.path {
        std::fs::remove_file(p).ok();
    }
}

#[test]
fn dropped_data_writes_file() {
    let mut s = Sink::new(std::process::id());
    s.push(b"a\nb\nc\nd\n");
    let cap = s.finish(4); // filter kept only half
    assert!(let Some(path) = cap.path);
    assert!(std::fs::read(&path).unwrap() == b"a\nb\nc\nd\n");
    std::fs::remove_file(path).ok();
}

#[test]
fn identical_content_hashes_to_same_path() {
    let content = b"repeatable output\nsecond line\n";
    let run = || {
        let mut s = Sink::new(std::process::id());
        s.push(content);
        s.finish(0).path.unwrap()
    };
    let p1 = run();
    let p2 = run();
    assert!(p1 == p2); // same content should dedup to one file
    std::fs::remove_file(p1).ok();
}

#[test]
fn spills_past_mem_cap() {
    let mut s = Sink::new(std::process::id());
    let chunk = vec![b'x'; 100 * 1024];
    for _ in 0..4 {
        s.push(&chunk); // 400 KiB > 256 KiB MEM_CAP
    }
    let cap = s.finish(0);
    check!(cap.bytes == 400 * 1024);
    assert!(let Some(path) = cap.path);
    assert!(std::fs::metadata(&path).unwrap().len() == 400 * 1024);
    std::fs::remove_file(path).ok();
}
