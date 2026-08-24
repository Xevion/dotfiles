use super::*;
use assert2::check;
use rstest::rstest;
use std::sync::atomic::{AtomicU32, Ordering};

static COUNTER: AtomicU32 = AtomicU32::new(0);

struct ScratchDir {
    root: PathBuf,
}

impl ScratchDir {
    fn new(label: &str) -> Self {
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        let root = std::env::temp_dir().join(format!(
            "guard-check-test-{label}-{}-{n}",
            std::process::id()
        ));
        std::fs::create_dir_all(&root).expect("create scratch dir");
        ScratchDir { root }
    }

    fn write(&self, rel: &str, contents: &str) {
        let path = self.root.join(rel);
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).expect("create parent dirs");
        }
        std::fs::write(&path, contents).expect("write scratch file");
    }
}

impl Drop for ScratchDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

#[rstest]
#[case::hidden(".git", true)]
#[case::hidden_venv(".venv", true)]
#[case::node_modules("node_modules", true)]
#[case::target("target", true)]
#[case::vendor("vendor", true)]
#[case::storybook_static("storybook-static", true)]
#[case::third_party("third_party", true)]
#[case::external("external", true)]
#[case::site_packages("site-packages", true)]
#[case::out("out", true)]
#[case::ordinary_src("src", false)]
#[case::ordinary_docs("docs", false)]
fn should_skip_dir_matches_known_directories(#[case] name: &str, #[case] expected: bool) {
    check!(should_skip_dir(name) == expected);
}

#[test]
fn collect_files_prunes_storybook_static_output() {
    let dir = ScratchDir::new("storybook");
    dir.write("web/src/App.svelte", "<div></div>\n");
    dir.write("web/storybook-static/index.html", "<html></html>\n");

    let mut files = Vec::new();
    collect_files(&dir.root, &mut files);
    let names: Vec<String> = files
        .iter()
        .map(|p| p.strip_prefix(&dir.root).unwrap().to_string_lossy().into_owned())
        .collect();

    check!(names.contains(&"web/src/App.svelte".to_string()));
    check!(!names.iter().any(|n| n.contains("storybook-static")));
}

#[test]
fn collect_files_prunes_skipped_directories() {
    let dir = ScratchDir::new("prune");
    dir.write("src/main.rs", "fn f() {}\n");
    dir.write("node_modules/pkg/index.js", "module.exports = {};\n");
    dir.write(".git/HEAD", "ref: refs/heads/main\n");

    let mut files = Vec::new();
    collect_files(&dir.root, &mut files);
    let names: Vec<String> = files
        .iter()
        .map(|p| p.strip_prefix(&dir.root).unwrap().to_string_lossy().into_owned())
        .collect();

    check!(names.contains(&"src/main.rs".to_string()));
    check!(!names.iter().any(|n| n.starts_with("node_modules")));
    check!(!names.iter().any(|n| n.starts_with(".git")));
}

#[test]
fn collect_files_on_a_single_file_returns_just_that_file() {
    let dir = ScratchDir::new("single-file");
    dir.write("only.rs", "fn f() {}\n");
    let mut files = Vec::new();
    collect_files(&dir.root.join("only.rs"), &mut files);
    check!(files.len() == 1);
}

#[test]
fn scan_file_counts_language_quality_and_findings() {
    let dir = ScratchDir::new("scan");
    dir.write("main.rs", "// ==========================\nfn f() {}\n");
    let mut totals = Totals::default();
    scan_file(&dir.root.join("main.rs"), &mut totals);

    check!(totals.files_scanned == 1);
    check!(totals.by_language.get("rust") == Some(&1));
    check!(totals.quality.clean == 1);
    check!(totals.rule_hits.get("banner").map(|h| h.count) == Some(1));
}

#[test]
fn scan_file_skips_excluded_paths_without_counting() {
    let dir = ScratchDir::new("excluded");
    dir.write("README.md", "# hello\n");
    let mut totals = Totals::default();
    scan_file(&dir.root.join("README.md"), &mut totals);
    check!(totals.files_scanned == 0);
}

#[test]
fn scan_file_skips_unrecognized_extensions_without_counting() {
    let dir = ScratchDir::new("unrecognized");
    dir.write("notes.txt", "whatever\n");
    let mut totals = Totals::default();
    scan_file(&dir.root.join("notes.txt"), &mut totals);
    check!(totals.files_scanned == 0);
}

#[test]
fn scan_file_untrusted_parse_is_counted_but_produces_no_rule_hits() {
    let dir = ScratchDir::new("untrusted");
    let source =
        "@#$%^&*(((((]]]]}}}}}{{{{{{{{ )))) nonsense !!! ??? garbage tokens here".repeat(20);
    dir.write("broken.rs", &source);
    let mut totals = Totals::default();
    scan_file(&dir.root.join("broken.rs"), &mut totals);

    check!(totals.files_scanned == 1);
    check!(totals.quality.untrusted == 1);
    check!(totals.rule_hits.is_empty());
}

#[test]
fn format_finding_includes_block_extent_and_preview_for_long_prose() {
    let source = "fn pad1() {}\nfn pad2() {}\nfn pad3() {}\nfn pad4() {}\nfn pad5() {}\nfn pad6() {}\n\
                  // line one\n// line two\n// line three\n// line four\nfn f() {}\n";
    let analysis = lang::analyze(Path::new("prose.rs"), source).expect("recognized extension");
    let report = comment::evaluate(&analysis);
    check!(report.nudges.len() == 1);

    let text = format_finding("src/prose.rs", &report.nudges[0], &analysis.blocks);
    check!(text.contains("lines 7-10, 4L"));
    check!(text.contains("line one"));
    check!(text.contains("line two"));
    check!(text.contains("line three"));
    check!(!text.contains("line four"));
    check!(text.contains("... (1 more lines)"));
}

#[test]
fn format_finding_is_single_line_for_non_long_prose_categories() {
    let source = "// ==========================\nfn f() {}\n";
    let analysis = lang::analyze(Path::new("banner.rs"), source).expect("recognized extension");
    let report = comment::evaluate(&analysis);
    check!(report.categorical.len() == 1);

    let text = format_finding("src/banner.rs", &report.categorical[0], &analysis.blocks);
    check!(!text.contains('\n'));
}

#[test]
fn main_returns_two_for_missing_path() {
    check!(main("/nonexistent/guard-check-path") == 2);
}

#[test]
fn main_returns_two_for_empty_path() {
    check!(main("") == 2);
}

#[test]
fn main_returns_zero_for_a_real_directory() {
    let dir = ScratchDir::new("main-ok");
    dir.write("main.rs", "fn f() {}\n");
    check!(main(dir.root.to_str().unwrap()) == 0);
}
