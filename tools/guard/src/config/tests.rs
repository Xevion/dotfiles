use super::*;
use assert2::check;
use rstest::*;
use std::sync::atomic::{AtomicU32, Ordering};

static COUNTER: AtomicU32 = AtomicU32::new(0);

struct Scratch {
    path: std::path::PathBuf,
}

impl Scratch {
    fn new() -> Self {
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        let path =
            std::env::temp_dir().join(format!("guard-config-test-{}-{n}", std::process::id()));
        std::fs::create_dir_all(path.join(".git/info")).expect("create scratch repo");
        Scratch { path }
    }

    fn write_config(&self, contents: &str) {
        std::fs::write(self.path.join(".git/info/guard.toml"), contents).unwrap();
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.path);
    }
}

#[test]
fn missing_config_file_falls_back_to_defaults() {
    let s = Scratch::new();
    let cfg = Config::load(&s.path);
    check!(cfg.hooks.disabled.is_empty());
    check!(cfg.comment_lint.enabled);
}

#[test]
fn malformed_toml_falls_back_to_defaults() {
    let s = Scratch::new();
    s.write_config("this is not [ valid toml");
    let cfg = Config::load(&s.path);
    check!(cfg.hooks.disabled.is_empty());
    check!(cfg.comment_lint.enabled);
}

#[test]
fn no_repo_at_all_falls_back_to_defaults() {
    let outside =
        std::env::temp_dir().join(format!("guard-config-test-no-repo-{}", std::process::id()));
    std::fs::create_dir_all(&outside).unwrap();
    let cfg = Config::load(&outside);
    std::fs::remove_dir_all(&outside).unwrap();
    check!(cfg.hooks.disabled.is_empty());
}

#[test]
fn valid_config_parses_hooks_and_comment_lint() {
    let s = Scratch::new();
    s.write_config(
        r#"
        [hooks]
        disabled = ["comment-lint"]

        [comment-lint]
        enabled = true
        ignore = ["src/generated/**", "*.gen.ts"]
        "#,
    );
    let cfg = Config::load(&s.path);
    check!(cfg.hooks.disabled == vec!["comment-lint".to_string()]);
    check!(cfg.comment_lint.enabled);
    check!(cfg.comment_lint.ignore.len() == 2);
}

#[rstest]
#[case::exact_dir_match("src/generated/foo.rs", true)]
#[case::nested_under_dir("src/generated/nested/bar.rs", true)]
#[case::suffix_glob_match("web/component.gen.ts", true)]
#[case::unrelated_file("src/main.rs", false)]
fn ignore_glob_matching(#[case] file: &str, #[case] expect_ignored: bool) {
    let cfg = CommentLintConfig {
        enabled: true,
        ignore: vec!["src/generated/**".into(), "*.gen.ts".into()],
    };
    check!(cfg.is_ignored(Some(file)) == expect_ignored);
}

#[test]
fn no_file_given_never_counts_as_ignored() {
    let cfg = CommentLintConfig {
        enabled: true,
        ignore: vec!["src/generated/**".into()],
    };
    check!(!cfg.is_ignored(None));
}

#[test]
fn disabled_hook_is_never_enabled_regardless_of_ignore() {
    let cfg = Config {
        hooks: HooksConfig {
            disabled: vec!["comment-lint".into()],
        },
        comment_lint: CommentLintConfig {
            enabled: true,
            ignore: Vec::new(),
        },
    };
    check!(!cfg.hook_enabled("comment-lint", Some("src/main.rs")));
}

#[test]
fn ignored_file_disables_comment_lint_even_when_hook_not_globally_disabled() {
    let cfg = Config {
        hooks: HooksConfig::default(),
        comment_lint: CommentLintConfig {
            enabled: true,
            ignore: vec!["src/generated/**".into()],
        },
    };
    check!(!cfg.hook_enabled("comment-lint", Some("src/generated/foo.rs")));
    check!(cfg.hook_enabled("comment-lint", Some("src/main.rs")));
}

#[test]
fn unknown_hook_id_defaults_to_enabled() {
    let cfg = Config::default();
    check!(cfg.hook_enabled("some-future-hook", None));
}
