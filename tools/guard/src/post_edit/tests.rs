use super::*;
use crate::lang;
use assert2::check;
use std::sync::atomic::{AtomicU32, Ordering};

static COUNTER: AtomicU32 = AtomicU32::new(0);

struct ScratchFile {
    path: std::path::PathBuf,
}

impl ScratchFile {
    fn write(name: &str, contents: &str) -> Self {
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        let path = std::env::temp_dir().join(format!(
            "guard-post-edit-test-{}-{n}-{name}",
            std::process::id()
        ));
        std::fs::write(&path, contents).expect("write scratch file");
        ScratchFile { path }
    }
}

impl Drop for ScratchFile {
    fn drop(&mut self) {
        let _ = std::fs::remove_file(&self.path);
    }
}

/// A repo-less scratch cwd (bare temp dir, no `.git` ancestor) so `Config`
/// always falls back to defaults regardless of where tests run from.
fn payload_for(file_path: &str) -> Payload {
    Payload {
        hook_event_name: "PostToolUse".into(),
        tool_name: "Edit".into(),
        session_id: None,
        cwd: Some(std::env::temp_dir().to_string_lossy().into_owned()),
        tool_input: crate::payload::ToolInput {
            command: None,
            file_path: Some(file_path.into()),
        },
    }
}

struct ScratchRepo {
    root: std::path::PathBuf,
}

impl ScratchRepo {
    fn new(label: &str) -> Self {
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        let root = std::env::temp_dir().join(format!(
            "guard-post-edit-repo-{label}-{}-{n}",
            std::process::id()
        ));
        std::fs::create_dir_all(root.join(".git/info")).expect("create scratch repo");
        ScratchRepo { root }
    }

    fn write_config(&self, contents: &str) {
        std::fs::write(self.root.join(".git/info/guard.toml"), contents).expect("write config");
    }

    fn write_file(&self, rel: &str, contents: &str) -> std::path::PathBuf {
        let path = self.root.join(rel);
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).expect("create parent dirs");
        }
        std::fs::write(&path, contents).expect("write scratch source file");
        path
    }

    fn payload_for(&self, path: &std::path::Path) -> Payload {
        Payload {
            hook_event_name: "PostToolUse".into(),
            tool_name: "Edit".into(),
            session_id: None,
            cwd: Some(self.root.to_string_lossy().into_owned()),
            tool_input: crate::payload::ToolInput {
                command: None,
                file_path: Some(path.to_string_lossy().into_owned()),
            },
        }
    }
}

impl Drop for ScratchRepo {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

#[test]
fn main_always_allows_when_no_findings() {
    let payload = payload_for("src/main.rs");
    check!(main(&payload).exit_code == 0);
}

#[test]
fn main_allows_even_when_file_path_missing() {
    let payload = Payload {
        hook_event_name: "PostToolUse".into(),
        tool_name: "Edit".into(),
        session_id: None,
        cwd: None,
        tool_input: crate::payload::ToolInput::default(),
    };
    check!(main(&payload).exit_code == 0);
}

#[test]
fn analyze_reads_and_parses_a_real_file() {
    let scratch = ScratchFile::write("real.rs", "// one\n// two\nfn f() {}\n");
    let payload = payload_for(scratch.path.to_str().unwrap());
    let analysis = analyze(&payload).expect("known extension, existing file");
    check!(analysis.quality == lang::ParseQuality::Clean);
    check!(analysis.comments.len() == 2);
}

#[test]
fn analyze_returns_none_for_unknown_extension() {
    let scratch = ScratchFile::write("notes.md", "# hello");
    let payload = payload_for(scratch.path.to_str().unwrap());
    check!(analyze(&payload).is_none());
}

#[test]
fn analyze_returns_none_for_missing_file() {
    let payload = payload_for("/nonexistent/path/does-not-exist.rs");
    check!(analyze(&payload).is_none());
}

#[test]
fn banner_comment_blocks_with_exit_two() {
    let scratch = ScratchFile::write("banner.rs", "// ==========================\nfn f() {}\n");
    let payload = payload_for(scratch.path.to_str().unwrap());
    let outcome = main(&payload);
    check!(outcome.exit_code == 2);
    check!(outcome.rules == vec!["comment-lint.banner".to_string()]);
    check!(!outcome.abstained);
}

#[test]
fn long_prose_nudge_allows_with_exit_zero_and_rule_recorded() {
    let source = "fn pad1() {}\nfn pad2() {}\nfn pad3() {}\nfn pad4() {}\nfn pad5() {}\n\
                  fn pad6() {}\n// line one\n// line two\n// line three\n// line four\n\
                  fn f() {}\n";
    let scratch = ScratchFile::write("prose.rs", source);
    let payload = payload_for(scratch.path.to_str().unwrap());
    let outcome = main(&payload);
    check!(outcome.exit_code == 0);
    check!(outcome.rules == vec!["comment-lint.long-prose".to_string()]);
    check!(!outcome.abstained);
}

#[test]
fn severely_broken_file_abstains_without_findings() {
    let source =
        "@#$%^&*(((((]]]]}}}}}{{{{{{{{ )))) nonsense !!! ??? garbage tokens here".repeat(20);
    let scratch = ScratchFile::write("broken.rs", &source);
    let payload = payload_for(scratch.path.to_str().unwrap());
    let outcome = main(&payload);
    check!(outcome.exit_code == 0);
    check!(outcome.abstained);
    check!(outcome.rules.is_empty());
}

#[test]
fn comment_lint_disabled_via_hooks_skips_entirely() {
    let repo = ScratchRepo::new("disabled");
    repo.write_config("[hooks]\ndisabled = [\"comment-lint\"]\n");
    let path = repo.write_file("src/main.rs", "// ==========================\nfn f() {}\n");
    let outcome = main(&repo.payload_for(&path));
    check!(outcome.exit_code == 0);
    check!(outcome.rules.is_empty());
    check!(!outcome.abstained);
}

#[test]
fn comment_lint_ignore_glob_skips_matching_file() {
    let repo = ScratchRepo::new("ignore");
    repo.write_config("[comment-lint]\nenabled = true\nignore = [\"src/generated/**\"]\n");
    let path = repo.write_file(
        "src/generated/foo.rs",
        "// ==========================\nfn f() {}\n",
    );
    let outcome = main(&repo.payload_for(&path));
    check!(outcome.exit_code == 0);
    check!(outcome.rules.is_empty());
}

#[test]
fn comment_lint_enabled_still_blocks_files_outside_ignore() {
    let repo = ScratchRepo::new("enabled");
    repo.write_config("[comment-lint]\nenabled = true\nignore = [\"src/generated/**\"]\n");
    let path = repo.write_file("src/main.rs", "// ==========================\nfn f() {}\n");
    let outcome = main(&repo.payload_for(&path));
    check!(outcome.exit_code == 2);
    check!(!outcome.rules.is_empty());
}
