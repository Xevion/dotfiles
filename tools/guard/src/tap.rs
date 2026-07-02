//! The source-stream sink: count lines/bytes, hold the stream in memory up to a
//! cap, spill to a content-hashed file beyond it, and decide at the end whether
//! a file is warranted at all. A file is kept only when the filters narrowed the
//! stream — the visible output is fewer bytes than the source. Equal or larger
//! output (pass-through, reordering, or expansion) hid nothing worth recovering.

use std::fs::{self, File};
use std::io::Write;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

const MEM_CAP: usize = 256 * 1024;
const SPILL_CAP: u64 = 64 * 1024 * 1024;
const DIR: &str = "/tmp/claude-guard";

/// Distinguishes pending files of concurrent Sinks that share a pid — notably
/// the parallel test runner, where every thread reports the same process id.
static PENDING_SEQ: AtomicU64 = AtomicU64::new(0);

/// Accumulates the source stream and tracks enough to build the footer.
pub struct Sink {
    hasher: blake3::Hasher,
    mem: Vec<u8>,
    spill: Option<File>,
    pending_path: PathBuf,
    bytes: u64,
    lines: u64,
    last_byte: u8,
    spilled: bool,
    truncated: bool,
    /// Set when the spill dir/file could not be created; capture is disabled.
    broken: bool,
}

/// What the footer needs to know about the captured stream.
pub struct Capture {
    pub bytes: u64,
    pub lines: u64,
    /// Path to the full output, when a file was kept.
    pub path: Option<PathBuf>,
    /// Stream exceeded the spill cap; `path` holds a truncated prefix.
    pub truncated: bool,
    /// Spill was requested but the directory/file was unwritable.
    pub unavailable: bool,
}

impl Sink {
    pub fn new(pid: u32) -> Self {
        Sink {
            hasher: blake3::Hasher::new(),
            mem: Vec::new(),
            spill: None,
            pending_path: PathBuf::from(DIR)
                .join(format!(".pending-{pid}-{}", PENDING_SEQ.fetch_add(1, Ordering::Relaxed))),
            bytes: 0,
            lines: 0,
            last_byte: b'\n',
            spilled: false,
            truncated: false,
            broken: false,
        }
    }

    /// Feed a chunk from the source stream.
    pub fn push(&mut self, buf: &[u8]) {
        if buf.is_empty() {
            return;
        }
        self.hasher.update(buf);
        self.bytes += buf.len() as u64;
        self.lines += bytecount_newlines(buf);
        self.last_byte = buf[buf.len() - 1];

        if self.truncated {
            return; // over spill cap: keep counting, stop storing
        }
        if self.spilled {
            self.write_spill(buf);
            return;
        }
        if self.mem.len() + buf.len() <= MEM_CAP {
            self.mem.extend_from_slice(buf);
            return;
        }
        // Crossing MEM_CAP: flush what we have to disk, then continue there.
        self.begin_spill();
        if !self.broken {
            let mem = std::mem::take(&mut self.mem);
            self.write_spill(&mem);
            self.write_spill(buf);
        }
    }

    fn begin_spill(&mut self) {
        if fs::create_dir_all(DIR).is_err() {
            self.broken = true;
            return;
        }
        match File::create(&self.pending_path) {
            Ok(f) => {
                self.spill = Some(f);
                self.spilled = true;
            }
            Err(_) => self.broken = true,
        }
    }

    fn write_spill(&mut self, buf: &[u8]) {
        if self.broken || self.truncated {
            return;
        }
        if self.bytes.saturating_sub(buf.len() as u64) >= SPILL_CAP {
            self.truncated = true;
            return;
        }
        if let Some(f) = self.spill.as_mut() {
            if f.write_all(buf).is_err() {
                self.broken = true;
            }
        }
    }

    /// Finalize. `final_bytes` is the visible output size; when it equals the
    /// source size the filters dropped nothing and no file is kept.
    pub fn finish(mut self, final_bytes: u64) -> Capture {
        // A trailing line without a newline still counts as a line.
        if self.bytes > 0 && self.last_byte != b'\n' {
            self.lines += 1;
        }
        if self.broken {
            let _ = fs::remove_file(&self.pending_path);
            return self.capture(None, true);
        }
        // Keep a file only when the filters narrowed the stream (visible output
        // smaller than the source). Equal or larger output hid nothing.
        if final_bytes >= self.bytes {
            let _ = fs::remove_file(&self.pending_path); // no-op unless spilled
            return self.capture(None, false);
        }
        let path = if self.spilled || self.truncated {
            self.commit_spill()
        } else {
            self.commit_mem()
        };
        self.capture(path, false)
    }

    fn commit_spill(&mut self) -> Option<PathBuf> {
        if let Some(f) = self.spill.take() {
            let _ = f.sync_all();
        }
        Some(self.commit_pending())
    }

    fn commit_mem(&mut self) -> Option<PathBuf> {
        if fs::create_dir_all(DIR).is_err() {
            return None;
        }
        if fs::write(&self.pending_path, &self.mem).is_err() {
            return None;
        }
        Some(self.commit_pending())
    }

    /// Atomically publish the per-run pending file under its content-hashed
    /// name. Concurrent runs with identical output rename onto the same
    /// destination, but each first writes its own unique pending file, so a
    /// reader of the final path never observes a truncated or half-written file.
    fn commit_pending(&self) -> PathBuf {
        let dest = PathBuf::from(DIR).join(format!("{}.log", self.hash_name()));
        match fs::rename(&self.pending_path, &dest) {
            Ok(()) => dest,
            Err(_) => self.pending_path.clone(),
        }
    }

    fn hash_name(&self) -> String {
        self.hasher.finalize().to_hex()[..12].to_string()
    }

    fn capture(&self, path: Option<PathBuf>, unavailable: bool) -> Capture {
        Capture {
            bytes: self.bytes,
            lines: self.lines,
            path,
            truncated: self.truncated,
            unavailable,
        }
    }
}

fn bytecount_newlines(buf: &[u8]) -> u64 {
    buf.iter().filter(|&&b| b == b'\n').count() as u64
}

#[cfg(test)]
mod tests;
