use super::*;
use assert2::{assert, check};
use rstest::*;

#[rstest]
#[case::nothing_dropped_writes_no_file(b"hello\nworld\n".as_slice(), 12, 12, 2, false)]
#[case::dropped_data_writes_file(b"a\nb\nc\nd\n".as_slice(), 4, 8, 4, true)]
#[case::trailing_unterminated_line_counts(b"one\ntwo".as_slice(), 0, 7, 2, true)]
// Visible output larger than source (a filter expanded it): nothing was
// narrowed, so no file is kept.
#[case::expansion_keeps_no_file(b"hi\n".as_slice(), 100, 3, 1, false)]
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
    // Content unique to this test: the spill path is content-hashed, so sharing
    // bytes with another test would collide on one path and race under the
    // parallel test runner.
    let content = b"dropped-file-roundtrip\nsecond line\n";
    let mut s = Sink::new(std::process::id());
    s.push(content);
    let cap = s.finish(0); // filter kept nothing visible
    assert!(let Some(path) = cap.path);
    assert!(std::fs::read(&path).unwrap() == content);
    std::fs::remove_file(path).ok();
}

#[test]
fn concurrent_identical_content_reads_are_never_truncated() {
    // Identical output hashes to one destination path across Sinks. Committing
    // must be atomic (unique pending file + rename), so a reader of the final
    // path never observes a truncated or half-written file. Under the old
    // direct File::create(dest), a parallel thread could read during another's
    // truncate and see an empty file.
    let content: &[u8] = b"concurrent-dedup\nsecond line\nthird line\n";
    let handles: Vec<_> = (0..16)
        .map(|_| {
            std::thread::spawn(move || {
                let mut s = Sink::new(std::process::id());
                s.push(content);
                let cap = s.finish(0);
                let path = cap.path.expect("spill path");
                let read = std::fs::read(&path).expect("read back");
                (path, read)
            })
        })
        .collect();

    let mut published = None;
    for h in handles {
        let (path, read) = h.join().expect("thread");
        check!(read == content);
        published = Some(path);
    }
    if let Some(p) = published {
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

#[test]
fn spill_file_bounded_to_exactly_cap() {
    // A source larger than SPILL_CAP truncates the file to exactly the cap — no
    // whole-chunk overshoot — while the counts still reflect the full stream.
    let mut s = Sink::new(std::process::id());
    let chunk = vec![b'z'; 1024 * 1024]; // 1 MiB
    let pushes = SPILL_CAP / chunk.len() as u64 + 4; // a few MiB past the cap
    for _ in 0..pushes {
        s.push(&chunk);
    }
    let total = pushes * chunk.len() as u64;
    let cap = s.finish(0); // nothing visible -> file kept
    check!(cap.bytes == total); // full stream counted
    check!(cap.truncated); // flagged truncated
    assert!(let Some(path) = cap.path);
    let on_disk = std::fs::metadata(&path).unwrap().len();
    check!(
        on_disk == SPILL_CAP,
        "spill file must be exactly the cap, got {on_disk}"
    );
    std::fs::remove_file(path).ok();
}

/// A TTL of zero makes every existing file expired, which is what lets the
/// sweep be tested without backdating mtimes.
#[test]
fn reap_dir_deletes_expired_files_and_keeps_its_stamp() {
    let dir = std::env::temp_dir().join(format!("guard-reap-{}", std::process::id()));
    std::fs::create_dir_all(&dir).unwrap();
    let spill = dir.join("abcdef.log");
    std::fs::write(&spill, b"stale").unwrap();

    reap_dir(&dir, Duration::ZERO, Duration::ZERO);

    check!(!spill.exists(), "expired spill file should be gone");
    check!(dir.join(REAP_STAMP).exists(), "stamp should be written");
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn reap_dir_skips_the_sweep_until_the_interval_elapses() {
    let dir = std::env::temp_dir().join(format!("guard-reap-skip-{}", std::process::id()));
    std::fs::create_dir_all(&dir).unwrap();
    std::fs::write(dir.join(REAP_STAMP), b"").unwrap();
    let spill = dir.join("abcdef.log");
    std::fs::write(&spill, b"fresh").unwrap();

    // Stamp is newer than the interval, so nothing is swept despite a zero TTL.
    reap_dir(&dir, Duration::ZERO, Duration::from_secs(3600));

    check!(
        spill.exists(),
        "sweep must not run before the interval elapses"
    );
    std::fs::remove_dir_all(&dir).ok();
}
