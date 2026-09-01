//! Dynamic auto-allow policy for `rm`: skip the confirmation prompt for
//! almost every delete. A tracked file is recoverable via git and an
//! untracked one is the caller's call to make, not ours to second-guess path
//! by path - so the only things still worth an ask are deletes that can't be
//! undone at all: wiping a top-level system directory, the home directory as
//! a whole, or credential material (SSH/GPG keys, an encryption identity,
//! `.git` itself). Anything this can't resolve to a concrete path falls
//! through to the normal ask flow rather than guessing.

use crate::parse::unquote;
use std::path::{Path, PathBuf};

/// Exact-match system roots: deleting these wholesale breaks the machine,
/// not just loses some files. Sub-paths inside them are ordinary projects -
/// `/home/xevion/projects/foo` is fine, `/home` itself is not.
const PROTECTED_ROOTS: &[&str] = &[
    "/", "/home", "/usr", "/var", "/etc", "/mnt", "/root", "/bin", "/sbin", "/lib", "/lib64",
    "/boot", "/dev", "/proc", "/sys", "/run", "/srv", "/opt",
];

/// Path components that mark credential material worth an extra prompt
/// regardless of location: deleting these can lock you out or destroy a
/// signing/encryption identity or a repo's entire history with no way back.
const CREDENTIAL_COMPONENTS: &[&str] = &[".ssh", ".gnupg", ".git"];

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
    !is_protected(&resolve(target, cwd))
}

fn resolve(target: &str, cwd: &Path) -> PathBuf {
    let p = Path::new(target);
    if p.is_absolute() {
        p.to_path_buf()
    } else {
        cwd.join(p)
    }
}

fn is_protected(p: &Path) -> bool {
    if is_protected_root(p) || is_home_dir(p) {
        return true;
    }
    let has_credential_component = p.components().any(|c| {
        c.as_os_str()
            .to_str()
            .is_some_and(|s| CREDENTIAL_COMPONENTS.contains(&s))
    });
    if has_credential_component {
        return true;
    }
    p.file_name()
        .and_then(|n| n.to_str())
        .is_some_and(is_credential_filename)
}

fn is_protected_root(p: &Path) -> bool {
    let s = p.to_string_lossy();
    let trimmed = s.trim_end_matches('/');
    let trimmed = if trimmed.is_empty() { "/" } else { trimmed };
    PROTECTED_ROOTS.contains(&trimmed)
}

fn is_home_dir(p: &Path) -> bool {
    let Ok(home) = std::env::var("HOME") else {
        return false;
    };
    p.to_string_lossy().trim_end_matches('/') == home.trim_end_matches('/')
}

/// Filenames recognized as private key material even outside a
/// `.ssh`/`.gnupg` directory - a stray `id_ed25519` copied to a project root
/// is still a key, and this repo's bootstrapped age identity lands as a bare
/// `key.txt` in the home directory.
fn is_credential_filename(name: &str) -> bool {
    const KEY_PREFIXES: &[&str] = &["id_rsa", "id_ed25519", "id_ecdsa", "id_dsa"];
    const KEY_SUFFIXES: &[&str] = &[".pem", ".pfx", ".p12"];
    KEY_PREFIXES.iter().any(|p| name.starts_with(p))
        || KEY_SUFFIXES.iter().any(|s| name.ends_with(s))
        || name == "key.txt"
}

#[cfg(test)]
mod tests;
