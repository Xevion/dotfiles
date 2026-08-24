//! Locate the enclosing git repository from a starting directory, resolving
//! `.git` whether it is a directory or a worktree/submodule pointer file.

use std::path::{Path, PathBuf};

pub struct Repo {
    /// The working tree root: the directory that contains `.git`.
    pub root: PathBuf,
    /// The actual git directory holding `info/`, `HEAD`, etc. For a plain
    /// repo this is `<root>/.git`; for a worktree it is the shared common
    /// directory resolved through the `gitdir:`/`commondir` pointer chain.
    pub git_dir: PathBuf,
}

/// Walk up from `start` looking for a `.git` entry and resolve it.
pub fn find_repo(start: &Path) -> Option<Repo> {
    let mut dir = if start.is_absolute() {
        start.to_path_buf()
    } else {
        std::env::current_dir().ok()?.join(start)
    };
    loop {
        let dot_git = dir.join(".git");
        if dot_git.is_dir() {
            return Some(Repo {
                root: dir,
                git_dir: dot_git,
            });
        }
        if dot_git.is_file() {
            let git_dir = resolve_gitdir_file(&dot_git, &dir)?;
            return Some(Repo { root: dir, git_dir });
        }
        dir = dir.parent()?.to_path_buf();
    }
}

/// Parse a `gitdir: <path>` pointer file and resolve to the effective git
/// directory, following `commondir` for worktrees.
fn resolve_gitdir_file(pointer: &Path, root: &Path) -> Option<PathBuf> {
    let text = std::fs::read_to_string(pointer).ok()?;
    let target = text.trim().strip_prefix("gitdir:")?.trim();
    let target = if Path::new(target).is_absolute() {
        PathBuf::from(target)
    } else {
        root.join(target)
    };

    let Ok(rel) = std::fs::read_to_string(target.join("commondir")) else {
        return Some(target);
    };
    let common = target.join(rel.trim());
    Some(common.canonicalize().unwrap_or(common))
}

#[cfg(test)]
mod tests;
