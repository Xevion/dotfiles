use super::*;

#[test]
fn nothing_dropped_writes_no_file() {
    let mut s = Sink::new(std::process::id());
    s.push(b"hello\nworld\n");
    let cap = s.finish(12); // final bytes == source bytes
    assert_eq!(cap.bytes, 12);
    assert_eq!(cap.lines, 2);
    assert!(cap.path.is_none());
}

#[test]
fn dropped_data_writes_file() {
    let mut s = Sink::new(std::process::id());
    s.push(b"a\nb\nc\nd\n");
    let cap = s.finish(4); // filter kept only half
    assert!(cap.path.is_some());
    let path = cap.path.unwrap();
    assert_eq!(std::fs::read(&path).unwrap(), b"a\nb\nc\nd\n");
    std::fs::remove_file(path).ok();
}

#[test]
fn trailing_unterminated_line_counts() {
    let mut s = Sink::new(std::process::id());
    s.push(b"one\ntwo");
    let cap = s.finish(0);
    assert_eq!(cap.lines, 2);
    if let Some(p) = cap.path {
        std::fs::remove_file(p).ok();
    }
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
    assert_eq!(p1, p2, "same content should dedup to one file");
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
    assert_eq!(cap.bytes, 400 * 1024);
    let path = cap.path.expect("spill file");
    assert_eq!(std::fs::metadata(&path).unwrap().len(), 400 * 1024);
    std::fs::remove_file(path).ok();
}
