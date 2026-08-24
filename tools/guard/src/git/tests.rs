use super::*;
use assert2::check;
use std::sync::atomic::{AtomicU32, Ordering};

static COUNTER: AtomicU32 = AtomicU32::new(0);

struct Scratch {
    path: PathBuf,
}

impl Scratch {
    fn new() -> Self {
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        let path = std::env::temp_dir().join(format!("guard-git-test-{}-{n}", std::process::id()));
        std::fs::create_dir_all(&path).expect("create scratch dir");
        Scratch { path }
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.path);
    }
}

#[test]
fn plain_repo_dir_resolves_directly() {
    let s = Scratch::new();
    std::fs::create_dir_all(s.path.join(".git")).unwrap();
    let repo = find_repo(&s.path).expect("repo found");
    check!(repo.root == s.path);
    check!(repo.git_dir == s.path.join(".git"));
}

#[test]
fn walks_up_from_a_nested_subdirectory() {
    let s = Scratch::new();
    std::fs::create_dir_all(s.path.join(".git")).unwrap();
    let nested = s.path.join("src").join("deep");
    std::fs::create_dir_all(&nested).unwrap();
    let repo = find_repo(&nested).expect("repo found");
    check!(repo.root == s.path);
}

#[test]
fn no_repo_anywhere_up_the_chain_returns_none() {
    // temp_dir() itself is not expected to be inside a git repo.
    let outside =
        std::env::temp_dir().join(format!("guard-git-test-no-repo-{}", std::process::id()));
    std::fs::create_dir_all(&outside).unwrap();
    let found = find_repo(&outside).is_some();
    std::fs::remove_dir_all(&outside).unwrap();
    check!(!found);
}

#[test]
fn worktree_pointer_file_resolves_via_commondir() {
    let s = Scratch::new();
    let common = s.path.join("main-repo").join(".git");
    let worktree_gitdir = common.join("worktrees").join("feature");
    std::fs::create_dir_all(&worktree_gitdir).unwrap();
    std::fs::write(worktree_gitdir.join("commondir"), "../..\n").unwrap();

    let worktree_root = s.path.join("feature-checkout");
    std::fs::create_dir_all(&worktree_root).unwrap();
    std::fs::write(
        worktree_root.join(".git"),
        format!("gitdir: {}\n", worktree_gitdir.display()),
    )
    .unwrap();

    let repo = find_repo(&worktree_root).expect("repo found");
    check!(repo.root == worktree_root);
    check!(repo.git_dir == common.canonicalize().unwrap());
}

#[test]
fn pointer_file_without_commondir_resolves_to_target_directly() {
    let s = Scratch::new();
    let gitdir = s.path.join("modules").join("sub");
    std::fs::create_dir_all(&gitdir).unwrap();

    let sub_root = s.path.join("submodule-checkout");
    std::fs::create_dir_all(&sub_root).unwrap();
    std::fs::write(
        sub_root.join(".git"),
        format!("gitdir: {}\n", gitdir.display()),
    )
    .unwrap();

    let repo = find_repo(&sub_root).expect("repo found");
    check!(repo.git_dir == gitdir);
}
