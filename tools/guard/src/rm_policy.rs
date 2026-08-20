//! Dynamic auto-allow policy for `rm`: skip the confirmation prompt only when
//! every operand is confidently disposable - not tracked by git, and sitting
//! under `/tmp` or inside a recognized build/cache directory. Anything this
//! can't classify with confidence falls through to the normal ask flow rather
//! than guessing; a wrong "safe" call here deletes something for real.

use crate::parse::unquote;
use std::path::{Path, PathBuf};
use std::process::Command;

/// Path components that mark a directory as ephemeral build/cache output,
/// safe to delete freely as long as nothing git-tracked lives inside it.
const EPHEMERAL_DIRS: &[&str] = &[
    "target",
    "node_modules",
    "dist",
    "build",
    "out",
    ".next",
    ".nuxt",
    ".svelte-kit",
    ".turbo",
    ".nx",
    ".vite",
    ".parcel-cache",
    "coverage",
    ".cache",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    ".tox",
    "__pycache__",
    ".venv",
    "venv",
    ".wrangler",
    ".gradle",
    "zig-cache",
    "zig-out",
    ".dart_tool",
    "tmp",
    ".tmp",
];

/// Whether every operand of this `rm` invocation (`argv[0] == "rm"`) is safe
/// to delete without confirmation.
pub fn is_safe(argv: &[String], cwd: Option<&str>) -> bool {
    let targets = rm_targets(argv);
    if targets.is_empty() {
        return false;
    }
    let base = cwd.map(Path::new).unwrap_or_else(|| Path::new("."));
    targets.iter().all(|t| is_safe_target(t, base))
}

/// The literal path operands of an `rm` invocation, skipping flags. An empty
/// result (including one unresolvable operand poisoning the whole call) means
/// "don't guess" - the caller then refuses to auto-allow.
fn rm_targets(argv: &[String]) -> Vec<String> {
    let mut out = Vec::new();
    let mut past_flags = false;
    for raw in argv.iter().skip(1) {
        if !past_flags {
            if raw == "--" {
                past_flags = true;
                continue;
            }
            if raw.starts_with('-') && raw.len() > 1 {
                continue;
            }
            past_flags = true;
        }
        match unquote(raw) {
            Some(path) if !path.is_empty() => out.push(path),
            _ => return Vec::new(),
        }
    }
    out
}

fn is_safe_target(target: &str, cwd: &Path) -> bool {
    let resolved = resolve(target, cwd);
    let ephemeral = resolved.starts_with("/tmp") || has_ephemeral_component(&resolved);
    ephemeral && !is_git_tracked(&resolved)
}

fn resolve(target: &str, cwd: &Path) -> PathBuf {
    let p = Path::new(target);
    if p.is_absolute() {
        p.to_path_buf()
    } else {
        cwd.join(p)
    }
}

fn has_ephemeral_component(p: &Path) -> bool {
    p.components().any(|c| {
        c.as_os_str()
            .to_str()
            .is_some_and(|s| EPHEMERAL_DIRS.contains(&s))
    })
}

/// Whether git knows about `path` at all: tracked itself, or (if a directory)
/// containing any tracked file. No repo, or git unavailable, counts as "not
/// tracked" - there is nothing to lose via git either way.
fn is_git_tracked(path: &Path) -> bool {
    let Some(dir) = path.parent() else {
        return false;
    };
    let out = Command::new("git")
        .arg("-C")
        .arg(dir)
        .arg("ls-files")
        .arg("--")
        .arg(path)
        .output();
    match out {
        Ok(o) if o.status.success() => !o.stdout.is_empty(),
        _ => false,
    }
}

#[cfg(test)]
mod tests;
