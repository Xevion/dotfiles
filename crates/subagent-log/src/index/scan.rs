//! Cheap, cached, incremental scans of one transcript. Transcripts are
//! append-only and total several gigabytes, so a rescan resumes from the
//! offset the last one stopped at instead of rereading the file, and an
//! unchanged file costs one `stat`.

use std::collections::HashMap;
use std::ffi::OsStr;
use std::fs::{self, File};
use std::io::{BufRead, BufReader, Seek as _, SeekFrom};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, PoisonError};
use std::time::SystemTime;

use memchr::memmem;

use crate::event::{self, Closure};

const READ_BUFFER: usize = 256 * 1024;

/// What a file is scanned for. A session's main transcript only feeds
/// working-directory matching, so it skips the turn tracking.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Track {
    CwdsOnly,
    CwdsAndTurns,
}

#[derive(Debug, Clone, Default)]
pub struct FileScan {
    len: u64,
    mtime: Option<SystemTime>,
    /// Offset just past the last newline-terminated line scanned. A trailing
    /// partial line is left for the next pass, once its writer finishes it.
    consumed: u64,
    /// Distinct, in first-seen order; the first is the file's own cwd.
    pub cwds: Vec<PathBuf>,
    pub closure: Closure,
    /// A handback call whose result has not been seen yet.
    pending_handback: Option<String>,
}

impl FileScan {
    fn note_cwd(&mut self, cwd: &str) {
        let known = |p: &PathBuf| p.as_os_str() == OsStr::new(cwd);
        // Nearly every line repeats the previous one's cwd.
        if self.cwds.last().is_some_and(known) || self.cwds.iter().any(known) {
            return;
        }
        self.cwds.push(PathBuf::from(cwd));
    }

    /// Feeds complete lines from `reader`, returning how many bytes they
    /// took. Only the last assistant line of the pass is fully parsed.
    fn advance(&mut self, mut reader: impl BufRead, track: Track) -> u64 {
        let mut line = Vec::new();
        // The last two assistant candidates: a corrupted line must not
        // displace the valid one before it.
        let mut last_assistant = Vec::new();
        let mut prev_assistant = Vec::new();
        let mut unclassified = false;
        let mut taken = 0u64;

        loop {
            line.clear();
            match reader.read_until(b'\n', &mut line) {
                Ok(0) | Err(_) => break,
                Ok(_) if line.last() != Some(&b'\n') => break,
                Ok(n) => taken += n as u64,
            }

            if let Some(cwd) = event::extract_cwd(&line) {
                self.note_cwd(cwd);
            }
            if track == Track::CwdsOnly {
                continue;
            }

            if event::is_assistant_line(&line) {
                self.pending_handback = None;
                if event::mentions_handback_call(&line) {
                    if let Some(message) = event::parse_assistant(&line) {
                        self.closure = message.closure();
                        self.pending_handback = message.handback_call_id().map(str::to_string);
                    }
                    unclassified = false;
                    last_assistant.clear();
                    prev_assistant.clear();
                } else {
                    std::mem::swap(&mut prev_assistant, &mut last_assistant);
                    std::mem::swap(&mut last_assistant, &mut line);
                    unclassified = true;
                }
            } else if let Some(id) = &self.pending_handback
                && event::is_user_line(&line)
                && memmem::find(&line, id.as_bytes()).is_some()
            {
                if let Some(event::Line::User(message)) = event::parse_line(&line)
                    && message.fails_handback(id)
                {
                    self.closure = Closure::Open;
                }
                self.pending_handback = None;
            }
        }

        if unclassified
            && let Some(message) = event::parse_assistant(&last_assistant)
                .or_else(|| event::parse_assistant(&prev_assistant))
        {
            self.closure = message.closure();
        }
        taken
    }
}

#[derive(Default)]
pub struct ScanCache {
    entries: Mutex<HashMap<(PathBuf, Track), Arc<FileScan>>>,
}

impl ScanCache {
    /// An unreadable file yields an empty scan rather than an error: it just
    /// contributes nothing to its session.
    #[must_use]
    pub fn scan(&self, path: &Path, track: Track) -> Arc<FileScan> {
        let Ok(metadata) = fs::metadata(path) else {
            return Arc::default();
        };
        let len = metadata.len();
        let mtime = metadata.modified().ok();

        let previous = self
            .entries
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .get(&(path.to_path_buf(), track))
            .cloned();

        if let Some(previous) = &previous
            && previous.len == len
            && previous.mtime == mtime
        {
            return Arc::clone(previous);
        }

        // Grown since last time: resume. Shrunk or rewritten: start over.
        let mut scan = match previous {
            Some(previous) if previous.len < len => (*previous).clone(),
            _ => FileScan::default(),
        };

        let Ok(mut file) = File::open(path) else {
            return Arc::new(scan);
        };
        if file.seek(SeekFrom::Start(scan.consumed)).is_ok() {
            scan.consumed += scan.advance(BufReader::with_capacity(READ_BUFFER, file), track);
        }
        scan.len = len;
        scan.mtime = mtime;

        let scan = Arc::new(scan);
        self.entries
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .insert((path.to_path_buf(), track), Arc::clone(&scan));
        scan
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;
    use std::io::Write as _;

    const USER_PROMPT: &str =
        r#"{"type":"user","cwd":"/work/a","message":{"role":"user","content":"go"}}"#;
    const WORKING: &str = r#"{"type":"assistant","cwd":"/work/a","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{}}]}}"#;
    const SAID_DONE: &str = r#"{"type":"assistant","cwd":"/work/b","message":{"content":[{"type":"text","text":"done"}]}}"#;
    const HANDBACK: &str = r#"{"type":"assistant","message":{"content":[{"type":"tool_use","id":"h1","name":"SubagentHandback","input":{"message":"r"}}]}}"#;
    const HANDBACK_OK: &str = r#"{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"h1","content":[{"type":"text","text":"{\"success\":true}"}]}]}}"#;
    const HANDBACK_REFUSED: &str = r#"{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"h1","content":[{"type":"text","text":"{\"success\":false}"}]}]}}"#;

    fn scan_lines(lines: &[&str]) -> FileScan {
        let mut scan = FileScan::default();
        let data = format!("{}\n", lines.join("\n"));
        scan.advance(data.as_bytes(), Track::CwdsAndTurns);
        scan
    }

    #[test]
    fn open_tool_call_leaves_the_turn_open() {
        check!(scan_lines(&[USER_PROMPT, WORKING]).closure == Closure::Open);
    }

    #[test]
    fn closing_text_closes_the_turn() {
        check!(scan_lines(&[USER_PROMPT, WORKING, SAID_DONE]).closure == Closure::Text);
    }

    #[test]
    fn handback_is_a_closed_turn_without_any_closing_text() {
        check!(scan_lines(&[WORKING, HANDBACK, HANDBACK_OK]).closure == Closure::HandedBack);
    }

    #[test]
    fn handback_still_awaiting_its_result_counts_as_handed_back() {
        check!(scan_lines(&[WORKING, HANDBACK]).closure == Closure::HandedBack);
    }

    #[test]
    fn refused_handback_reopens_the_turn() {
        check!(scan_lines(&[WORKING, HANDBACK, HANDBACK_REFUSED]).closure == Closure::Open);
    }

    #[test]
    fn work_after_a_handback_reopens_the_turn() {
        check!(scan_lines(&[HANDBACK, HANDBACK_OK, WORKING]).closure == Closure::Open);
    }

    #[test]
    fn corrupted_assistant_line_does_not_displace_the_valid_one_before_it() {
        let corrupted =
            r#"{"type":"assistant","message":{"content":[{"type":"tool_use",{"parentUuid":"x"#;
        check!(scan_lines(&[WORKING, SAID_DONE, corrupted]).closure == Closure::Text);
    }

    #[test]
    fn cwds_are_distinct_and_keep_first_seen_order() {
        let scan = scan_lines(&[USER_PROMPT, WORKING, SAID_DONE, USER_PROMPT]);
        check!(scan.cwds == [PathBuf::from("/work/a"), PathBuf::from("/work/b")]);
    }

    #[test]
    fn cwds_only_tracking_ignores_turns() {
        let mut scan = FileScan::default();
        let data = format!("{SAID_DONE}\n");
        scan.advance(data.as_bytes(), Track::CwdsOnly);
        check!(scan.closure == Closure::Open);
        check!(scan.cwds == [PathBuf::from("/work/b")]);
    }

    #[test]
    fn trailing_partial_line_is_left_unconsumed() {
        let mut scan = FileScan::default();
        let data = format!("{SAID_DONE}\n{{\"type\":\"assistant\",\"mess");
        let taken = scan.advance(data.as_bytes(), Track::CwdsAndTurns);
        check!(taken == SAID_DONE.len() as u64 + 1);
        check!(scan.closure == Closure::Text);
    }

    fn write_file(path: &Path, lines: &[&str]) {
        let mut f = File::create(path).unwrap();
        for line in lines {
            writeln!(f, "{line}").unwrap();
        }
    }

    fn append(path: &Path, lines: &[&str]) {
        let mut f = fs::OpenOptions::new().append(true).open(path).unwrap();
        for line in lines {
            writeln!(f, "{line}").unwrap();
        }
    }

    #[test]
    fn rescan_resumes_from_where_it_stopped() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("agent-x.jsonl");
        write_file(&path, &[USER_PROMPT, WORKING]);

        let cache = ScanCache::default();
        let first = cache.scan(&path, Track::CwdsAndTurns);
        check!(first.closure == Closure::Open);

        append(&path, &[SAID_DONE]);
        let second = cache.scan(&path, Track::CwdsAndTurns);
        check!(second.closure == Closure::Text);
        check!(second.cwds.len() == 2);
        check!(second.consumed > first.consumed);
    }

    #[test]
    fn unchanged_file_returns_the_cached_scan() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("agent-x.jsonl");
        write_file(&path, &[USER_PROMPT]);

        let cache = ScanCache::default();
        let first = cache.scan(&path, Track::CwdsAndTurns);
        let second = cache.scan(&path, Track::CwdsAndTurns);
        check!(Arc::ptr_eq(&first, &second));
    }

    #[test]
    fn a_result_arriving_in_a_later_pass_still_resolves_the_handback() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("agent-x.jsonl");
        write_file(&path, &[WORKING, HANDBACK]);

        let cache = ScanCache::default();
        check!(cache.scan(&path, Track::CwdsAndTurns).closure == Closure::HandedBack);

        append(&path, &[HANDBACK_REFUSED]);
        check!(cache.scan(&path, Track::CwdsAndTurns).closure == Closure::Open);
    }

    #[test]
    fn truncated_file_is_rescanned_from_scratch() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("agent-x.jsonl");
        write_file(&path, &[USER_PROMPT, WORKING, SAID_DONE]);

        let cache = ScanCache::default();
        check!(cache.scan(&path, Track::CwdsAndTurns).closure == Closure::Text);

        write_file(&path, &[WORKING]);
        check!(cache.scan(&path, Track::CwdsAndTurns).closure == Closure::Open);
    }

    #[test]
    fn tracking_modes_are_cached_independently() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("agent-x.jsonl");
        write_file(&path, &[USER_PROMPT, SAID_DONE]);

        let cache = ScanCache::default();
        check!(cache.scan(&path, Track::CwdsOnly).closure == Closure::Open);
        check!(cache.scan(&path, Track::CwdsAndTurns).closure == Closure::Text);
    }

    #[test]
    fn missing_file_yields_an_empty_scan() {
        let cache = ScanCache::default();
        let scan = cache.scan(Path::new("/nonexistent/agent-x.jsonl"), Track::CwdsAndTurns);
        check!(scan.cwds.is_empty());
        check!(scan.closure == Closure::Open);
    }
}
