//! `guard run '<pipeline>'`: execute one pipeline, tap the unfiltered source
//! stream, and print a footer. Spawns each stage directly (no shell) with pipes
//! wired by hand, so PATH resolution, redirects, and signal behavior match the
//! unwrapped command. A compound first stage runs via `$SHELL -c`.

use crate::parse::{basename, FileKind, PipelineInfo, Redir, Stage};
use crate::tap::{Capture, Sink};
use std::fs::{File, OpenOptions};
use std::io::{self, PipeReader, PipeWriter, Read, Write};
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

impl Stdin {
    fn into_stdio(self) -> Stdio {
        match self {
            Stdin::Inherit => Stdio::inherit(),
            Stdin::Pipe(r) => Stdio::from(r),
        }
    }
}

/// stdout target for a stage: a guard-owned pipe, or a redirected file.
enum Out {
    Pipe(PipeWriter),
    File(File),
}

impl Out {
    fn try_clone(&self) -> io::Result<Out> {
        Ok(match self {
            Out::Pipe(w) => Out::Pipe(w.try_clone()?),
            Out::File(f) => Out::File(f.try_clone()?),
        })
    }

    fn into_stdio(self) -> Stdio {
        match self {
            Out::Pipe(w) => Stdio::from(w),
            Out::File(f) => Stdio::from(f),
        }
    }
}

fn spawn_stage(stage: &Stage, stdin: Stdin, stdout: PipeWriter) -> io::Result<Child> {
    match stage {
        Stage::Compound { text } => {
            let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".into());
            let mut cmd = Command::new(shell);
            cmd.arg("-c").arg(text);
            cmd.stdin(stdin.into_stdio());
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
            apply_io(&mut cmd, stdin, stdout, redirs)?;
            cmd.spawn()
        }
        Stage::Unsupported => Err(io::Error::other("unsupported stage")),
    }
}

/// Wire stdin/stdout/stderr, honoring the supported redirect set. Redirects
/// override the pipe defaults, matching shell precedence.
fn apply_io(cmd: &mut Command, stdin: Stdin, stdout: PipeWriter, redirs: &[Redir]) -> io::Result<()> {
    let mut out = Out::Pipe(stdout);
    let mut stdin_file: Option<File> = None;
    let mut stderr_to_stdout = false;
    let mut stderr_file: Option<File> = None;

    for r in redirs {
        match r {
            Redir::Dup { from: 2, to: 1 } => {
                stderr_to_stdout = true;
                stderr_file = None;
            }
            Redir::File { fd: 1, kind, path } => out = Out::File(open_out(path, kind)?),
            Redir::File { fd: 2, kind, path } => {
                stderr_file = Some(open_out(path, kind)?);
                stderr_to_stdout = false;
            }
            Redir::File { fd: 0, path, .. } => stdin_file = Some(File::open(path)?),
            Redir::OutErr { path, append } => {
                let kind = if *append { FileKind::Append } else { FileKind::Write };
                let f = open_out(path, &kind)?;
                stderr_file = Some(f.try_clone()?);
                out = Out::File(f);
                stderr_to_stdout = false;
            }
            // Other dup/fd forms are excluded by the predicate; ignore defensively.
            _ => {}
        }
    }

    cmd.stdin(match stdin_file {
        Some(f) => Stdio::from(f),
        None => stdin.into_stdio(),
    });
    if stderr_to_stdout {
        cmd.stderr(out.try_clone()?.into_stdio());
    } else if let Some(f) = stderr_file {
        cmd.stderr(Stdio::from(f));
    }
    cmd.stdout(out.into_stdio());
    Ok(())
}

fn open_out(path: &str, kind: &FileKind) -> io::Result<File> {
    let mut opts = OpenOptions::new();
    opts.write(true).create(true);
    match kind {
        FileKind::Append => opts.append(true),
        _ => opts.truncate(true),
    };
    opts.open(path)
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
