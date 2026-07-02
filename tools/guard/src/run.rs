//! `guard run '<pipeline>'`: execute one pipeline, tap the unfiltered source
//! stream, and print a footer. Spawns each stage directly (no shell) with pipes
//! wired by hand, so PATH resolution, redirects, and signal behavior match the
//! unwrapped command. A compound first stage runs via `$SHELL -c`.

use crate::parse::{basename, FileKind, PipelineInfo, Redir, Stage};
use crate::tap::{Capture, Sink};
use std::fs::{File, OpenOptions};
use std::io::{self, PipeReader, PipeWriter, Read, Write};
use std::os::fd::{AsFd, OwnedFd};
use std::os::unix::process::ExitStatusExt;
use std::process::{Child, Command, ExitStatus, Stdio};
use std::thread;
use std::time::{Duration, Instant};

pub fn main(pipeline: &str) -> i32 {
    match PipelineInfo::single(pipeline).filter(PipelineInfo::capturable) {
        Some(pl) => match run_captured(&pl) {
            Ok(code) => code,
            // Only pre-spawn errors reach here; nothing has run yet.
            Err(_) => exec_fallback(pipeline),
        },
        None => exec_fallback(pipeline),
    }
}

/// Run the pipeline unchanged through the harness shell, no capture, no footer.
fn exec_fallback(pipeline: &str) -> i32 {
    let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".into());
    Command::new(shell)
        .arg("-c")
        .arg(pipeline)
        .status()
        .map(status_code)
        .unwrap_or(127)
}

fn run_captured(pl: &PipelineInfo) -> io::Result<i32> {
    let n = pl.stages.len();
    let label = source_label(&pl.stages[0]);

    // Boundaries owned by guard: source->tap (a), tap->first filter (b),
    // last filter->final tap (z). Inter-filter boundaries are direct.
    let (a_r, a_w) = io::pipe()?;
    let (b_r, b_w) = io::pipe()?;
    let (z_r, z_w) = io::pipe()?;

    let mut children: Vec<Child> = Vec::with_capacity(n);
    let started = Instant::now();

    // Source: stdin inherited, stdout -> a_w.
    children.push(spawn_stage(&pl.stages[0], Stdin::Inherit, a_w)?);

    // Filters. First reads b_r; last writes z_w; middles use fresh pipes.
    let mut prev_out: Option<PipeReader> = Some(b_r);
    let mut z_w = Some(z_w);
    for i in 1..n {
        let stdin = Stdin::Pipe(prev_out.take().expect("prev out"));
        let stdout = if i == n - 1 {
            z_w.take().expect("z_w") // guard keeps only z_r
        } else {
            let (r, w) = io::pipe()?;
            prev_out = Some(r);
            w
        };
        children.push(spawn_stage(&pl.stages[i], stdin, stdout)?);
    }

    // Source tap: copy a_r -> b_w, counting and buffering. Propagate close on
    // downstream EPIPE so the source dies like it would under a real shell.
    let pid = std::process::id();
    let tap = thread::spawn(move || {
        let mut sink = Sink::new(pid);
        let mut a_r = a_r;
        let mut b_w = b_w;
        let mut buf = [0u8; 64 * 1024];
        loop {
            match a_r.read(&mut buf) {
                Ok(0) | Err(_) => break,
                Ok(k) => {
                    sink.push(&buf[..k]);
                    if b_w.write_all(&buf[..k]).is_err() {
                        break; // downstream closed: stop, drop a_r to close source
                    }
                }
            }
        }
        sink
    });

    // Final tap: relay z_r -> stdout, counting visible bytes.
    let final_tap = thread::spawn(move || {
        let mut z_r = z_r;
        let mut out = io::stdout();
        let mut buf = [0u8; 64 * 1024];
        let mut total: u64 = 0;
        loop {
            match z_r.read(&mut buf) {
                Ok(0) | Err(_) => break,
                Ok(k) => {
                    total += k as u64;
                    if out.write_all(&buf[..k]).is_err() {
                        break;
                    }
                }
            }
        }
        let _ = out.flush();
        total
    });

    // Wait in reverse order (final -> source) to avoid a full-pipe deadlock.
    let mut statuses: Vec<ExitStatus> = Vec::with_capacity(n);
    for child in children.iter_mut().rev() {
        statuses.push(child.wait()?);
    }
    statuses.reverse(); // now indexed by stage
    let source_status = statuses[0];
    let final_status = statuses[n - 1];
    let duration = started.elapsed();

    let sink = tap.join().unwrap_or_else(|_| Sink::new(pid));
    let final_bytes = final_tap.join().unwrap_or(0);
    let capture = sink.finish(final_bytes);

    print_footer(&label, source_status, duration, &capture);
    Ok(status_code(final_status))
}

enum Stdin {
    Inherit,
    Pipe(PipeReader),
}

/// One of a simple stage's three standard fds while redirects are resolved:
/// either still inheriting the parent's fd `n`, or bound to an owned fd.
enum Slot {
    Inherit(i32),
    Owned(OwnedFd),
}

impl Slot {
    /// Duplicate this slot's current target, mirroring `dup2`'s "copy the
    /// referent" semantics.
    fn clone_fd(&self) -> io::Result<OwnedFd> {
        match self {
            Slot::Inherit(n) => dup_parent(*n),
            Slot::Owned(f) => f.as_fd().try_clone_to_owned(),
        }
    }

    fn into_stdio(self) -> Stdio {
        match self {
            // Explicit inherit is equivalent to leaving the fd unset.
            Slot::Inherit(_) => Stdio::inherit(),
            Slot::Owned(f) => Stdio::from(f),
        }
    }
}

/// Duplicate one of the parent's standard fds (0/1/2) as an owned fd.
fn dup_parent(n: i32) -> io::Result<OwnedFd> {
    match n {
        0 => io::stdin().as_fd().try_clone_to_owned(),
        1 => io::stdout().as_fd().try_clone_to_owned(),
        _ => io::stderr().as_fd().try_clone_to_owned(),
    }
}

fn spawn_stage(stage: &Stage, stdin: Stdin, stdout: PipeWriter) -> io::Result<Child> {
    match stage {
        Stage::Compound { text } => {
            let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".into());
            let mut cmd = Command::new(shell);
            cmd.arg("-c").arg(text);
            cmd.stdin(match stdin {
                Stdin::Inherit => Stdio::inherit(),
                Stdin::Pipe(r) => Stdio::from(r),
            });
            cmd.stdout(Stdio::from(stdout));
            cmd.spawn()
        }
        Stage::Simple {
            assignments,
            argv,
            redirs,
        } => {
            let mut cmd = Command::new(&argv[0]);
            cmd.args(&argv[1..]);
            for (k, v) in assignments {
                cmd.env(k, v);
            }
            let (in_io, out_io, err_io) = resolve_fds(stdin, stdout, redirs)?;
            cmd.stdin(in_io);
            cmd.stdout(out_io);
            cmd.stderr(err_io);
            cmd.spawn()
        }
        Stage::Unsupported => Err(io::Error::other("unsupported stage")),
    }
}

/// Resolve a simple stage's stdin/stdout/stderr by replaying its redirects over
/// a three-slot fd table (fds 0/1/2), exactly as the shell would: left to right,
/// each dup copying the *current* referent of the source fd. This reproduces
/// ordering-sensitive forms such as `2>&1 1>/dev/null` (stderr keeps the pipe,
/// stdout goes to the file). fd numbers above 2 are rejected in
/// `parse::conv_redir` and never reach here.
fn resolve_fds(
    stdin: Stdin,
    stdout: PipeWriter,
    redirs: &[Redir],
) -> io::Result<(Stdio, Stdio, Stdio)> {
    let mut slots = [
        match stdin {
            Stdin::Inherit => Slot::Inherit(0),
            Stdin::Pipe(r) => Slot::Owned(OwnedFd::from(r)),
        },
        Slot::Owned(OwnedFd::from(stdout)),
        Slot::Inherit(2),
    ];

    for r in redirs {
        match r {
            Redir::File { fd, kind, path } => {
                slots[*fd as usize] = Slot::Owned(open_target(path, kind)?);
            }
            Redir::Dup { from, to } => {
                let dup = slots[*to as usize].clone_fd()?;
                slots[*from as usize] = Slot::Owned(dup);
            }
            Redir::OutErr { path, append } => {
                let kind = if *append { FileKind::Append } else { FileKind::Write };
                let target = open_target(path, &kind)?;
                let dup = target.as_fd().try_clone_to_owned()?;
                slots[1] = Slot::Owned(target);
                slots[2] = Slot::Owned(dup);
            }
        }
    }

    let [s0, s1, s2] = slots;
    Ok((s0.into_stdio(), s1.into_stdio(), s2.into_stdio()))
}

/// Open a redirect target as an owned fd, honoring the redirect kind.
fn open_target(path: &str, kind: &FileKind) -> io::Result<OwnedFd> {
    let file = match kind {
        FileKind::Read => File::open(path)?,
        FileKind::Write => OpenOptions::new().write(true).create(true).truncate(true).open(path)?,
        FileKind::Append => OpenOptions::new().create(true).append(true).open(path)?,
    };
    Ok(OwnedFd::from(file))
}

fn status_code(status: ExitStatus) -> i32 {
    status
        .code()
        .unwrap_or_else(|| 128 + status.signal().unwrap_or(0))
}

fn source_label(stage: &Stage) -> String {
    match stage {
        Stage::Simple { argv, .. } => argv
            .first()
            .map(|a| basename(a).to_string())
            .unwrap_or_else(|| "?".into()),
        Stage::Compound { .. } => "{...}".into(),
        Stage::Unsupported => "?".into(),
    }
}

fn print_footer(label: &str, source: ExitStatus, dur: Duration, cap: &Capture) {
    let code = status_code(source);
    let sig = if code >= 128 && source.signal().is_some() {
        signal_hint(source.signal().unwrap())
    } else {
        String::new()
    };

    let mut line = format!("[guard] {label}: exit {code}{sig}, {}", fmt_dur(dur));
    if cap.unavailable {
        line.push_str(", capture unavailable");
    } else if let Some(path) = &cap.path {
        line.push_str(&format!(
            ", source {} lines / {}",
            cap.lines,
            fmt_size(cap.bytes)
        ));
        if cap.truncated {
            line.push_str(" (truncated at 64 MiB)");
        }
        line.push_str(&format!(", full: {}", path.display()));
    }
    // Footer goes to stdout, after the visible output, so it reaches user and
    // agent alike and no filter can eat it.
    println!("{line}");
}

fn signal_hint(sig: i32) -> String {
    match sig {
        13 => " (SIGPIPE, cut by a downstream filter)".into(),
        _ => format!(" (killed by signal {sig})"),
    }
}

fn fmt_dur(d: Duration) -> String {
    let s = d.as_secs_f64();
    if s >= 1.0 {
        format!("{s:.1}s")
    } else {
        format!("{}ms", d.as_millis())
    }
}

fn fmt_size(bytes: u64) -> String {
    const KB: u64 = 1024;
    const MB: u64 = 1024 * 1024;
    if bytes >= MB {
        format!("{:.1} MB", bytes as f64 / MB as f64)
    } else if bytes >= KB {
        format!("{} KB", bytes / KB)
    } else {
        format!("{bytes} B")
    }
}

#[cfg(test)]
mod tests;
