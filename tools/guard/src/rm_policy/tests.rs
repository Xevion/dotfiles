use super::*;
use assert2::{assert, check};
use rstest::*;

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
#[case::project_source("/home/xevion/projects/foo/src/main.rs")]
#[case::project_root("/home/xevion/projects/foo")]
#[case::tmp_scratch("/tmp/claude-1000/scratch.txt")]
#[case::dotfile_in_home("/home/xevion/.bashrc")]
fn ordinary_paths_are_safe(#[case] path: &str) {
    check!(is_safe_target(path, Path::new(".")));
}

#[rstest]
#[case::root("/")]
#[case::root_trailing_slash("//")]
#[case::home_top("/home")]
#[case::usr("/usr")]
#[case::etc("/etc")]
fn protected_roots_are_unsafe(#[case] path: &str) {
    check!(!is_safe_target(path, Path::new(".")));
}

#[test]
fn home_directory_itself_is_unsafe() {
    let home = std::env::var("HOME").expect("HOME set in test env");
    check!(!is_safe_target(&home, Path::new(".")));
}

#[rstest]
#[case::ssh_key("/home/xevion/.ssh/id_ed25519")]
#[case::ssh_dir_itself("/home/xevion/.ssh")]
#[case::gnupg("/home/xevion/.gnupg/private-keys-v1.d")]
#[case::dot_git("/home/xevion/projects/foo/.git")]
#[case::nested_dot_git("/home/xevion/projects/foo/.git/objects")]
#[case::bare_key_file("/home/xevion/key.txt")]
#[case::pem_anywhere("/home/xevion/projects/foo/cert.pem")]
#[case::id_rsa_anywhere("/home/xevion/projects/foo/id_rsa")]
fn credential_material_is_unsafe(#[case] path: &str) {
    check!(!is_safe_target(path, Path::new(".")));
}

#[test]
fn relative_target_resolves_against_cwd() {
    check!(!is_safe_target(
        ".git",
        Path::new("/home/xevion/projects/foo")
    ));
    check!(is_safe_target(
        "notes.txt",
        Path::new("/home/xevion/projects/foo")
    ));
}

#[test]
fn is_safe_requires_every_operand_safe() {
    let cmd = "rm -rf /home/xevion/projects/foo/build.log /home/xevion/.ssh/id_ed25519";
    check!(!is_safe(&argv(cmd), None));
}

#[test]
fn is_safe_true_when_every_operand_ordinary() {
    let cmd = "rm -rf /home/xevion/projects/foo/build.log /tmp/x";
    check!(is_safe(&argv(cmd), None));
}
