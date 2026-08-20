use super::*;
use assert2::{assert, check};
use rstest::*;
use std::sync::atomic::{AtomicU32, Ordering};

fn argv(s: &str) -> Vec<String> {
    s.split_whitespace().map(String::from).collect()
}

#[rstest]
#[case::simple("rm foo.txt", &["foo.txt"])]
#[case::multiple("rm a b c", &["a", "b", "c"])]
#[case::recursive_force("rm -rf /tmp/x", &["/tmp/x"])]
#[case::separate_flags("rm -r -f /tmp/x", &["/tmp/x"])]
#[case::double_dash("rm -- -weird-name", &["-weird-name"])]
fn rm_targets_extracts_paths(#[case] cmd: &str, #[case] expected: &[&str]) {
    let got = rm_targets(&argv(cmd));
    assert!(got == expected.iter().map(|s| s.to_string()).collect::<Vec<_>>());
}

#[test]
fn rm_targets_unquotes_a_single_quoted_operand() {
    // argv tokens keep their raw source text (quotes included), matching
    // what the real AST-based collector hands rm_targets in production.
    let argv: Vec<String> = vec!["rm".into(), "'/tmp/has space'".into()];
    assert!(rm_targets(&argv) == vec!["/tmp/has space".to_string()]);
}

#[rstest]
#[case::glob("rm /tmp/*")]
#[case::dollar_var("rm $FILE")]
#[case::tilde("rm ~/foo")]
fn rm_targets_refuses_unresolvable(#[case] cmd: &str) {
    assert!(rm_targets(&argv(cmd)).is_empty());
}

#[test]
fn rm_targets_empty_for_flags_only() {
    assert!(rm_targets(&argv("rm --help")).is_empty());
}

#[rstest]
#[case::tmp_root("/tmp/foo", true)]
#[case::tmp_nested("/tmp/a/b/c", true)]
#[case::not_tmp("/home/xevion/projects/foo", false)]
fn has_ephemeral_component_or_tmp(#[case] path: &str, #[case] expect_tmp: bool) {
    check!(Path::new(path).starts_with("/tmp") == expect_tmp);
}

#[rstest]
#[case::target_dir("/home/x/proj/target/debug", true)]
#[case::node_modules("/home/x/proj/node_modules/foo", true)]
#[case::wrangler_state("/home/x/proj/.wrangler/state/v3", true)]
#[case::plain_source("/home/x/proj/src/main.rs", false)]
#[case::plain_project_root("/home/x/proj", false)]
fn ephemeral_component_detection(#[case] path: &str, #[case] expect: bool) {
    assert!(has_ephemeral_component(Path::new(path)) == expect);
}

static COUNTER: AtomicU32 = AtomicU32::new(0);

/// A scratch directory under the real temp dir, cleaned up on drop.
struct Scratch {
    path: PathBuf,
}

impl Scratch {
    fn new() -> Self {
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        let path =
            std::env::temp_dir().join(format!("guard-rm-policy-test-{}-{n}", std::process::id()));
        std::fs::create_dir_all(&path).expect("create scratch dir");
        Scratch { path }
    }

    fn git_init(&self) {
        run(&self.path, &["init", "-q"]);
        run(&self.path, &["config", "user.email", "test@example.com"]);
        run(&self.path, &["config", "user.name", "test"]);
    }

    fn git_add(&self, rel: &str) {
        run(&self.path, &["add", rel]);
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.path);
    }
}

fn run(dir: &Path, args: &[&str]) {
    let status = Command::new("git")
        .current_dir(dir)
        .args(args)
        .status()
        .expect("run git");
    assert!(status.success(), "git {args:?} failed");
}

#[test]
fn untracked_file_under_ephemeral_dir_is_safe() {
    let s = Scratch::new();
    s.git_init();
    let target = s.path.join("target");
    std::fs::create_dir_all(&target).unwrap();
    std::fs::write(target.join("scratch.bin"), b"x").unwrap();
    check!(is_safe_target(
        target.join("scratch.bin").to_str().unwrap(),
        Path::new(".")
    ));
}

#[test]
fn tracked_file_under_ephemeral_dir_is_not_safe() {
    let s = Scratch::new();
    s.git_init();
    let target = s.path.join("target");
    std::fs::create_dir_all(&target).unwrap();
    std::fs::write(target.join("committed.bin"), b"x").unwrap();
    s.git_add("target/committed.bin");
    check!(!is_safe_target(
        target.join("committed.bin").to_str().unwrap(),
        Path::new(".")
    ));
}

#[test]
fn untracked_file_outside_ephemeral_dir_is_not_safe() {
    // Not under /tmp, no ephemeral-dir component in the path - existence is
    // irrelevant to the classification, so no real fixture is needed here.
    check!(!is_safe_target(
        "/home/xevion/projects/some-repo/notes.txt",
        Path::new(".")
    ));
}

#[test]
fn tracked_dir_containing_a_tracked_file_is_not_safe() {
    let s = Scratch::new();
    s.git_init();
    let target = s.path.join("target").join("release");
    std::fs::create_dir_all(&target).unwrap();
    std::fs::write(target.join("app"), b"x").unwrap();
    s.git_add("target/release/app");
    // Deleting the parent `target/` dir wholesale should still be unsafe:
    // it contains a tracked file.
    check!(!is_safe_target(
        s.path.join("target").to_str().unwrap(),
        Path::new(".")
    ));
}

#[test]
fn no_git_repo_under_ephemeral_dir_is_safe() {
    // /tmp itself is (almost always) not inside a git repo, so a plain
    // untracked scratch file there should classify as safe.
    let s = Scratch::new();
    let f = s.path.join("scratch.log");
    std::fs::write(&f, b"x").unwrap();
    check!(is_safe(&argv(&format!("rm {}", f.to_str().unwrap())), None));
}

#[test]
fn is_safe_requires_every_operand_safe() {
    let s = Scratch::new();
    s.git_init();
    let target = s.path.join("target");
    std::fs::create_dir_all(&target).unwrap();
    std::fs::write(target.join("a"), b"x").unwrap();
    // Second operand is a real path outside both /tmp and any ephemeral dir.
    let cmd = format!(
        "rm -rf {} /home/xevion/projects/some-repo/important.txt",
        target.join("a").display()
    );
    check!(!is_safe(&argv(&cmd), None));
}
