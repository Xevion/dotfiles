use super::*;
use assert2::{assert, check};
use rstest::rstest;

fn parse(language: Language, source: &str) -> Tree {
    let mut parser = Parser::new();
    parser
        .set_language(&language.ts_language())
        .expect("set language");
    parser.parse(source, None).expect("parse")
}

#[rstest]
#[case::rust("main.rs", Some(Language::Rust))]
#[case::ts("index.ts", Some(Language::TypeScript))]
#[case::tsx("app.tsx", Some(Language::Tsx))]
#[case::js("script.js", Some(Language::JavaScript))]
#[case::jsx("comp.jsx", Some(Language::JavaScript))]
#[case::mjs("mod.mjs", Some(Language::JavaScript))]
#[case::cjs("mod.cjs", Some(Language::JavaScript))]
#[case::go("main.go", Some(Language::Go))]
#[case::py("script.py", Some(Language::Python))]
#[case::c("main.c", Some(Language::C))]
#[case::h("header.h", Some(Language::C))]
#[case::cpp("main.cpp", Some(Language::Cpp))]
#[case::cc("main.cc", Some(Language::Cpp))]
#[case::hpp("header.hpp", Some(Language::Cpp))]
#[case::java("Main.java", Some(Language::Java))]
#[case::cs("Program.cs", Some(Language::CSharp))]
#[case::sh("script.sh", Some(Language::Bash))]
#[case::bash("script.bash", Some(Language::Bash))]
#[case::css("style.css", Some(Language::Css))]
#[case::kt("Main.kt", Some(Language::Kotlin))]
#[case::kts("build.kts", Some(Language::Kotlin))]
#[case::svelte("App.svelte", Some(Language::Svelte))]
#[case::sql("query.sql", Some(Language::Sql))]
#[case::unknown("README.md", None)]
#[case::no_extension("Makefile", None)]
fn detects_language_from_extension(#[case] name: &str, #[case] expected: Option<Language>) {
    assert!(Language::from_path(Path::new(name)) == expected);
}

#[rstest]
#[case::rust(Language::Rust, "// plain\nfn f() {}\n", &["// plain"])]
#[case::go(Language::Go, "// plain\nfunc f() {}\n", &["// plain"])]
#[case::python(Language::Python, "# plain\nx = 1\n", &["# plain"])]
#[case::c(Language::C, "// plain\nint main() {}\n", &["// plain"])]
#[case::cpp(Language::Cpp, "// plain\nint main() {}\n", &["// plain"])]
#[case::java(Language::Java, "// plain\nclass A {}\n", &["// plain"])]
#[case::csharp(Language::CSharp, "// plain\nclass A {}\n", &["// plain"])]
#[case::bash(Language::Bash, "# plain\necho hi\n", &["# plain"])]
#[case::css(Language::Css, "/* plain */\na { color: red; }\n", &["/* plain */"])]
#[case::kotlin(Language::Kotlin, "// plain\nfun f() {}\n", &["// plain"])]
#[case::sql(Language::Sql, "-- plain\nSELECT 1;\n", &["-- plain"])]
#[case::typescript(Language::TypeScript, "// plain\nfunction f() {}\n", &["// plain"])]
#[case::tsx(Language::Tsx, "// plain\nfunction f() { return <div/>; }\n", &["// plain"])]
#[case::javascript(Language::JavaScript, "// plain\nfunction f() {}\n", &["// plain"])]
fn extracts_comments_per_language(
    #[case] language: Language,
    #[case] source: &str,
    #[case] expected: &[&str],
) {
    let tree = parse(language, source);
    let comments = extract_comments(language, &tree, source);
    let texts: Vec<&str> = comments.iter().map(|c| c.text.as_str()).collect();
    assert!(texts == expected);
}

#[rstest]
#[case::outer_line("/// outer doc\nfn f() {}\n", CommentKind::Doc)]
#[case::inner_line("//! inner doc\nfn f() {}\n", CommentKind::Doc)]
#[case::plain_line("// plain\nfn f() {}\n", CommentKind::Plain)]
#[case::outer_block("/** outer doc */\nfn f() {}\n", CommentKind::Doc)]
#[case::plain_block("/* plain */\nfn f() {}\n", CommentKind::Plain)]
#[case::four_slashes_is_not_doc("//// divider\nfn f() {}\n", CommentKind::Plain)]
fn rust_doc_classification_is_structural(#[case] source: &str, #[case] expected: CommentKind) {
    let tree = parse(Language::Rust, source);
    let comments = extract_comments(Language::Rust, &tree, source);
    assert!(comments.len() == 1);
    check!(comments[0].kind == expected);
}

#[rstest]
#[case::jsdoc(
    Language::TypeScript,
    "/**\n * doc\n */\nfunction f() {}\n",
    CommentKind::Doc
)]
#[case::jsdoc_plain(
    Language::TypeScript,
    "// plain\nfunction f() {}\n",
    CommentKind::Plain
)]
#[case::javadoc(Language::Java, "/**\n * doc\n */\nclass A {}\n", CommentKind::Doc)]
#[case::kdoc(Language::Kotlin, "/**\n * doc\n */\nfun f() {}\n", CommentKind::Doc)]
#[case::csharp_xmldoc(
    Language::CSharp,
    "/// <summary>doc</summary>\nclass A {}\n",
    CommentKind::Doc
)]
#[case::csharp_plain(Language::CSharp, "// plain\nclass A {}\n", CommentKind::Plain)]
fn prefix_based_doc_classification(
    #[case] language: Language,
    #[case] source: &str,
    #[case] expected: CommentKind,
) {
    let tree = parse(language, source);
    let comments = extract_comments(language, &tree, source);
    assert!(comments.len() == 1);
    check!(comments[0].kind == expected);
}

#[test]
fn go_comment_adjacent_to_declaration_is_doc() {
    let source = "// Doer does things.\nfunc Doer() {}\n";
    let tree = parse(Language::Go, source);
    let comments = extract_comments(Language::Go, &tree, source);
    assert!(comments.len() == 1);
    check!(comments[0].kind == CommentKind::Doc);
}

#[test]
fn go_comment_separated_by_blank_line_is_not_doc() {
    let source = "// Not adjacent.\n\nfunc Doer() {}\n";
    let tree = parse(Language::Go, source);
    let comments = extract_comments(Language::Go, &tree, source);
    assert!(comments.len() == 1);
    check!(comments[0].kind == CommentKind::Plain);
}

#[test]
fn go_trailing_comment_is_not_doc() {
    let source = "func Doer() {} // trailing\n";
    let tree = parse(Language::Go, source);
    let comments = extract_comments(Language::Go, &tree, source);
    assert!(comments.len() == 1);
    check!(comments[0].kind == CommentKind::Plain);
}

#[test]
fn go_consecutive_lines_merge_before_adjacency_test() {
    let source = "// Line one.\n// Line two.\nfunc Doer() {}\n";
    let tree = parse(Language::Go, source);
    let comments = extract_comments(Language::Go, &tree, source);
    assert!(comments.len() == 2);
    check!(comments.iter().all(|c| c.kind == CommentKind::Doc));
}

#[test]
fn merge_comments_groups_consecutive_standalone_lines() {
    let source = "// one\n// two\n\n// three\nfn f() {}\n";
    let tree = parse(Language::Rust, source);
    let comments = extract_comments(Language::Rust, &tree, source);
    let blocks = merge_comments(&comments);
    assert!(blocks.len() == 2);
    check!(blocks[0].comments.len() == 2);
    check!(blocks[1].comments.len() == 1);
}

#[test]
fn merge_comments_does_not_merge_block_style_with_line_style() {
    let source = "/* block */\n// line\nfn f() {}\n";
    let tree = parse(Language::Rust, source);
    let comments = extract_comments(Language::Rust, &tree, source);
    let blocks = merge_comments(&comments);
    assert!(blocks.len() == 2);
    check!(blocks[0].comments.len() == 1);
    check!(blocks[1].comments.len() == 1);
}

#[test]
fn merge_comments_does_not_merge_trailing_comments() {
    let source = "let a = 1; // one\nlet b = 2; // two\n";
    let tree = parse(Language::Rust, source);
    let comments = extract_comments(Language::Rust, &tree, source);
    let blocks = merge_comments(&comments);
    assert!(blocks.len() == 2);
}

#[rstest]
#[case::standalone("// standalone\nfn f() {}\n", Placement::Standalone)]
#[case::trailing("fn f() {} // trailing\n", Placement::Trailing)]
#[case::indented_standalone("    // indented\n    fn f() {}\n", Placement::Standalone)]
fn placement_detects_standalone_vs_trailing(#[case] source: &str, #[case] expected: Placement) {
    let tree = parse(Language::Rust, source);
    let comments = extract_comments(Language::Rust, &tree, source);
    assert!(comments.len() == 1);
    check!(comments[0].placement == expected);
}

#[test]
fn svelte_script_comment_offsets_shift_into_outer_coordinates() {
    let source = "<div>\n  hello\n</div>\n<script>\n  // inner comment\n  let x = 1;\n</script>\n";
    let tree = parse(Language::Svelte, source);
    let comments = extract_comments(Language::Svelte, &tree, source);
    assert!(comments.len() == 1);
    let comment = &comments[0];
    check!(comment.text == "// inner comment");
    check!(comment.start_line == 4);
    check!(comment.end_line == 4);
    check!(&source[comment.byte_range.clone()] == "// inner comment");
}

#[test]
fn svelte_script_doc_comment_still_classifies_via_prefix() {
    let source = "<script>\n/**\n * doc\n */\nfunction f() {}\n</script>\n";
    let tree = parse(Language::Svelte, source);
    let comments = extract_comments(Language::Svelte, &tree, source);
    assert!(comments.len() == 1);
    check!(comments[0].kind == CommentKind::Doc);
}

#[test]
fn well_formed_source_is_clean() {
    let source = "fn f() -> i32 { 42 }\n";
    let tree = parse(Language::Rust, source);
    let (quality, stats) = classify_quality(tree.root_node(), source.len());
    check!(quality == ParseQuality::Clean);
    check!(stats.error_bytes == 0);
    check!(stats.missing_nodes == 0);
}

#[test]
fn severely_broken_source_is_untrusted() {
    let source =
        "@#$%^&*(((((]]]]}}}}}{{{{{{{{ )))) nonsense !!! ??? garbage tokens here".repeat(20);
    let tree = parse(Language::Rust, &source);
    let (quality, stats) = classify_quality(tree.root_node(), source.len());
    check!(quality == ParseQuality::Untrusted);
    check!(stats.error_bytes > 0);
}

#[test]
fn small_localized_error_is_degraded_not_untrusted() {
    let padding = "fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }\n".repeat(20);
    let source = format!("{padding}fn broken(\n{padding}");
    let tree = parse(Language::Rust, &source);
    let (quality, stats) = classify_quality(tree.root_node(), source.len());
    check!(quality == ParseQuality::Degraded);
    check!(stats.error_bytes > 0);
}

#[test]
fn analyze_returns_none_for_unknown_extension() {
    check!(analyze(Path::new("README.md"), "# hello").is_none());
}

#[test]
fn analyze_extracts_comments_and_blocks_for_known_extension() {
    let source = "// one\n// two\nfn f() {}\n";
    let analysis = analyze(Path::new("lib.rs"), source).expect("known extension");
    check!(analysis.quality == ParseQuality::Clean);
    check!(analysis.comments.len() == 2);
    check!(analysis.blocks.len() == 1);
}

#[test]
fn header_with_cpp_markers_sniffs_as_cpp() {
    let source = "#pragma once\n#include <string>\n\nnamespace facade {\n\
                  void f(std::string s);\n} // namespace facade\n";
    let analysis = analyze(Path::new("bridge.h"), source).expect("recognized extension");
    check!(analysis.language == Language::Cpp);
}

#[test]
fn header_with_no_cpp_markers_stays_c() {
    let source = "#ifndef ABI_H\n#define ABI_H\n\nint init_headless(void);\n\n#endif\n";
    let analysis = analyze(Path::new("abi.h"), source).expect("recognized extension");
    check!(analysis.language == Language::C);
}

#[test]
fn header_cpp_marker_only_inside_a_comment_stays_c() {
    let source = "/* mentions rust::behavior::trycatch in prose only */\n\
                  int init_headless(void);\n";
    let analysis = analyze(Path::new("abi.h"), source).expect("recognized extension");
    check!(analysis.language == Language::C);
}

#[rstest]
#[case::cpp_only_include("#include <memory>\nint f(void);\n")]
#[case::namespace("namespace bridge {\nint f();\n}\n")]
#[case::template("template <class T> T f(T x) { return x; }\n")]
#[case::class_decl("class Widget {\npublic:\n  int x;\n};\n")]
#[case::scope_resolution("int x = std::move(y);\n")]
#[case::extern_cpp("extern \"C++\" {\nint f();\n}\n")]
fn header_cpp_marker_variants_sniff_as_cpp(#[case] source: &str) {
    let analysis = analyze(Path::new("thing.h"), source).expect("recognized extension");
    check!(analysis.language == Language::Cpp);
}

#[test]
fn non_header_c_extension_is_never_sniffed() {
    let source = "#include <string>\nnamespace bridge { void f(); }\n";
    let analysis = analyze(Path::new("thing.c"), source).expect("recognized extension");
    check!(analysis.language == Language::C);
}
