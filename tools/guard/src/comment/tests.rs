use super::*;
use crate::lang;
use assert2::check;
use rstest::rstest;
use std::path::Path;

fn analyze(ext: &str, source: &str) -> Analysis {
    let name = format!("test.{ext}");
    lang::analyze(Path::new(&name), source).expect("recognized extension parses")
}

fn only_categories(findings: &[Finding]) -> Vec<Category> {
    findings.iter().map(|f| f.category).collect()
}

#[test]
fn banner_divider_comment_fires() {
    let analysis = analyze("rs", "// ==========================\nfn f() {}\n");
    let report = evaluate(&analysis);
    check!(only_categories(&report.categorical) == vec![Category::Banner]);
    check!(report.categorical[0].line == 0);
}

#[test]
fn bare_label_comment_fires() {
    let analysis = analyze("rs", "// Handlers\nfn f() {}\n");
    let report = evaluate(&analysis);
    check!(only_categories(&report.categorical) == vec![Category::BareLabel]);
}

#[test]
fn two_word_bare_label_comment_fires() {
    let analysis = analyze("rs", "// Config Types\nfn f() {}\n");
    let report = evaluate(&analysis);
    check!(only_categories(&report.categorical) == vec![Category::BareLabel]);
}

#[test]
fn history_comment_fires() {
    let analysis = analyze("rs", "// Previously used a different approach\nfn f() {}\n");
    let report = evaluate(&analysis);
    check!(only_categories(&report.categorical) == vec![Category::History]);
}

#[rstest]
#[case::refactored("// Refactored from the old client")]
#[case::used_to("// Used to retry three times")]
#[case::removed("// Removed the cache layer")]
fn history_alternatives_stay_unnarrowed(#[case] line: &str) {
    let source = format!("{line}\nfn f() {{}}\n");
    let analysis = analyze("rs", &source);
    let report = evaluate(&analysis);
    check!(only_categories(&report.categorical) == vec![Category::History]);
}

#[test]
fn ordinary_comment_does_not_fire() {
    let analysis = analyze("rs", "// this explains a real, specific decision\nfn f() {}\n");
    let report = evaluate(&analysis);
    check!(report.categorical.is_empty());
}

#[test]
fn divider_inside_a_string_literal_does_not_fire() {
    let analysis = analyze("rs", "fn f() {\n    let s = \"----------\";\n}\n");
    check!(analysis.comments.is_empty());
    let report = evaluate(&analysis);
    check!(report.categorical.is_empty());
}

#[test]
fn divider_inside_a_regex_literal_does_not_fire() {
    let analysis = analyze("js", "const re = /^-----$/;\n");
    check!(analysis.comments.is_empty());
    let report = evaluate(&analysis);
    check!(report.categorical.is_empty());
}

#[test]
fn banner_line_number_accounts_for_preceding_blank_lines() {
    let source = "fn a() {}\n\n\n// ---------------------------\nfn b() {}\n";
    let analysis = analyze("rs", source);
    let report = evaluate(&analysis);
    check!(report.categorical.len() == 1);
    check!(report.categorical[0].line == 3);
}

#[test]
fn python_encoding_declaration_does_not_fire() {
    let analysis = analyze("py", "# -*- coding: utf-8 -*-\nx = 1\n");
    let report = evaluate(&analysis);
    check!(report.categorical.is_empty());
}

const PADDING: &str = "fn pad1() {}\nfn pad2() {}\nfn pad3() {}\nfn pad4() {}\nfn pad5() {}\nfn pad6() {}\n";

#[test]
fn long_prose_three_lines_does_not_fire() {
    let source = format!("{PADDING}// line one\n// line two\n// line three\nfn f() {{}}\n");
    let analysis = analyze("rs", &source);
    let report = evaluate(&analysis);
    check!(report.nudges.is_empty());
}

#[test]
fn long_prose_four_lines_fires() {
    let source = format!(
        "{PADDING}// line one\n// line two\n// line three\n// line four\nfn f() {{}}\n"
    );
    let analysis = analyze("rs", &source);
    let report = evaluate(&analysis);
    check!(only_categories(&report.nudges) == vec![Category::LongProse]);
    check!(report.nudges[0].line == 6);
}

#[test]
fn long_prose_exempt_within_first_five_lines() {
    let source = "// line one\n// line two\n// line three\n// line four\nfn f() {}\n";
    let analysis = analyze("rs", source);
    let report = evaluate(&analysis);
    check!(report.nudges.is_empty());
}

#[test]
fn long_prose_exempt_for_doc_comments() {
    let source = format!(
        "{PADDING}/// line one\n/// line two\n/// line three\n/// line four\nfn f() {{}}\n"
    );
    let analysis = analyze("rs", &source);
    let report = evaluate(&analysis);
    check!(report.nudges.is_empty());
}

#[test]
fn trailing_comment_at_ten_words_does_not_fire() {
    let source = "let x = 1; // one two three four five six seven eight nine ten\n";
    let analysis = analyze("rs", source);
    let report = evaluate(&analysis);
    check!(report.nudges.is_empty());
}

#[test]
fn trailing_comment_at_eleven_words_fires() {
    let source = "let x = 1; // one two three four five six seven eight nine ten eleven\n";
    let analysis = analyze("rs", source);
    let report = evaluate(&analysis);
    check!(only_categories(&report.nudges) == vec![Category::VerboseTrailing]);
}

#[test]
fn evaluate_trusted_abstains_on_untrusted_parse() {
    let source =
        "@#$%^&*(((((]]]]}}}}}{{{{{{{{ )))) nonsense !!! ??? garbage tokens here".repeat(20);
    let analysis = analyze("rs", &source);
    check!(analysis.quality == lang::ParseQuality::Untrusted);
    check!(evaluate_trusted(&analysis).is_none());
}

#[test]
fn evaluate_trusted_runs_normally_on_clean_parse() {
    let analysis = analyze("rs", "// Handlers\nfn f() {}\n");
    let report = evaluate_trusted(&analysis).expect("clean parse is trusted");
    check!(!report.categorical.is_empty());
}

#[rstest]
#[case::readme("README.md", true)]
#[case::lockfile("Cargo.lock", true)]
#[case::svg("logo.svg", true)]
#[case::snapshot("output.snap", true)]
#[case::node_modules("web/node_modules/pkg/index.ts", true)]
#[case::migrations("db/migrations/0001_init.sql", true)]
#[case::minified("dist/app.min.js", true)]
#[case::vendor("vendor/lib/thing.go", true)]
#[case::generated_dir("src/generated/api.ts", true)]
#[case::gen_suffix("src/schema.gen.ts", true)]
#[case::generated_suffix("src/schema.generated.ts", true)]
#[case::target_dir("target/debug/build/foo.rs", true)]
#[case::storybook_static("web/storybook-static/index.html", true)]
#[case::svelte_kit(".svelte-kit/generated/root.svelte", true)]
#[case::next_dir(".next/server/app/page.js", true)]
#[case::nuxt_dir(".nuxt/dist/client/app.js", true)]
#[case::out_dir("out/bin/app.js", true)]
#[case::site_packages("venv/lib/site-packages/pkg/mod.py", true)]
#[case::third_party("third_party/lib/thing.go", true)]
#[case::external_dir("external/lib/thing.go", true)]
#[case::vendored_install_include("spikes/glibcwall/install/include/stdio.h", true)]
#[case::ordinary_source("src/main.rs", false)]
#[case::sveltekit_routes_not_out("web/src/routes/+page.svelte", false)]
fn is_excluded_path_matches_ported_and_added_rules(#[case] path: &str, #[case] expected: bool) {
    check!(is_excluded_path(Path::new(path)) == expected);
}

#[test]
fn format_block_report_caps_examples_and_reports_overflow() {
    let mut source = String::new();
    for i in 0..7 {
        source.push_str(&format!("// {}\n", "=".repeat(10 + i)));
    }
    source.push_str("fn f() {}\n");
    let analysis = analyze("rs", &source);
    let report = evaluate(&analysis);
    check!(report.categorical.len() == 7);
    let text = format_block_report("src/main.rs", &report);
    check!(text.contains("... and 2 more"));
    check!(text.matches("  src/main.rs:").count() == 5);
}

#[test]
fn format_block_report_includes_nudges_after_categorical() {
    let source = format!(
        "// ==========================\n{PADDING}// line one\n// line two\n// line three\n\
         // line four\nfn f() {{}}\n"
    );
    let analysis = analyze("rs", &source);
    let report = evaluate(&analysis);
    check!(!report.categorical.is_empty());
    check!(!report.nudges.is_empty());
    let text = format_block_report("src/main.rs", &report);
    let banner_pos = text.find("Remove decorative dividers").expect("banner section present");
    let prose_pos = text
        .find("Standalone comment block is unusually long")
        .expect("long-prose section present");
    check!(banner_pos < prose_pos);
}
