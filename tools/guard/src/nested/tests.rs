use super::*;
use assert2::{assert, check};
use proptest::prelude::*;
use rstest::*;

fn local(argv: &[&str]) -> Option<String> {
    match nested_payload(&argv.iter().map(|s| s.to_string()).collect::<Vec<_>>()) {
        Some(Nested::Local(p)) => Some(p),
        _ => None,
    }
}

fn remote(argv: &[&str]) -> Option<String> {
    match nested_payload(&argv.iter().map(|s| s.to_string()).collect::<Vec<_>>()) {
        Some(Nested::Remote(p)) => Some(p),
        _ => None,
    }
}

#[rstest]
#[case::bash_c(&["bash", "-c", "'git stash'"], "git stash")]
#[case::sh_c(&["sh", "-c", "'rm -rf x'"], "rm -rf x")]
#[case::dash_c(&["dash", "-c", "'echo hi'"], "echo hi")]
#[case::zsh_c(&["zsh", "-c", "'ls'"], "ls")]
#[case::fish_c(&["fish", "-c", "'echo hi'"], "echo hi")]
#[case::ksh_c(&["ksh", "-c", "'true'"], "true")]
// Bundled flags: login/interactive shells still carry -c.
#[case::bash_lc(&["bash", "-lc", "'sudo x'"], "sudo x")]
#[case::bash_ic(&["bash", "-ic", "'x'"], "x")]
// Absolute path to the shell resolves by basename.
#[case::abs_path(&["/bin/bash", "-c", "'x'"], "x")]
#[case::usr_bin(&["/usr/bin/sh", "-c", "'ls -la'"], "ls -la")]
// Double-quoted payload unquotes too.
#[case::double_quoted(&["bash", "-c", "\"echo hi\""], "echo hi")]
// Trailing operands after the script ($0, $1) are ignored.
#[case::trailing_args(&["bash", "-c", "'echo $0'", "argzero"], "echo $0")]
// Transparent prefixes and env assignments are skipped to reach the shell.
#[case::env_prefix(&["env", "bash", "-c", "'x'"], "x")]
#[case::nohup_prefix(&["nohup", "bash", "-c", "'x'"], "x")]
#[case::assignment_prefix(&["env", "FOO=1", "bash", "-c", "'x'"], "x")]
#[case::bare_assignment_prefix(&["FOO=bar", "bash", "-c", "'x'"], "x")]
// Arg-taking bash options do not shadow the -c that follows them.
#[case::shopt_upper_o(&["bash", "-O", "extglob", "-c", "'git stash'"], "git stash")]
#[case::set_lower_o(&["bash", "-o", "posix", "-c", "'x'"], "x")]
#[case::plus_upper_o(&["bash", "+O", "histexpand", "-c", "'ls'"], "ls")]
// Long boolean options before -c are skipped without shadowing it.
#[case::norc(&["bash", "--norc", "-c", "'git stash'"], "git stash")]
#[case::noprofile_norc(&["bash", "--noprofile", "--norc", "-c", "'x'"], "x")]
fn local_shell_payloads(#[case] tokens: &[&str], #[case] expected: &str) {
    check!(local(tokens).as_deref() == Some(expected));
}

#[rstest]
// No -c: a script file, not an inline command.
#[case::script_file(&["bash", "script.sh"])]
// -c present but no following token.
#[case::dangling_c(&["bash", "-c"])]
// Not a shell.
#[case::not_a_shell(&["python", "-c", "'print(1)'"])]
#[case::plain_command(&["ls", "-la"])]
// Payload with an unexpanded parameter: value unknowable, fail open.
#[case::dollar_var(&["bash", "-c", "\"rm $TARGET\""])]
#[case::command_subst(&["bash", "-c", "\"$(evil)\""])]
fn local_none(#[case] tokens: &[&str]) {
    check!(local(tokens) == None);
}

#[rstest]
// Single-token remote command.
#[case::single(&["ssh", "roman", "'uptime'"], "uptime")]
// ssh flattens split operands with spaces (verified against the real ssh).
#[case::split_operands(&["ssh", "roman", "rm", "-rf", "/tmp/x"], "rm -rf /tmp/x")]
// user@host host form.
#[case::user_at_host(&["ssh", "user@roman", "'ls -la'"], "ls -la")]
// Flags that consume the next token (port, identity, option) are skipped.
#[case::port_flag(&["ssh", "-p", "2222", "roman", "'ls'"], "ls")]
#[case::identity_flag(&["ssh", "-i", "key.pem", "roman", "'ls'"], "ls")]
#[case::option_flag(&["ssh", "-o", "StrictHostKeyChecking=no", "roman", "'ls'"], "ls")]
// Boolean flags do not consume a token.
#[case::verbose(&["ssh", "-v", "roman", "'ls'"], "ls")]
#[case::tt(&["ssh", "-tt", "roman", "'ls'"], "ls")]
// Boolean cluster then arg-taking flag then value.
#[case::mixed_cluster(&["ssh", "-4p", "2222", "roman", "'ls'"], "ls")]
// Glued arg form: -p2222.
#[case::glued_port(&["ssh", "-p2222", "roman", "'ls'"], "ls")]
// slogin is an ssh alias.
#[case::slogin(&["slogin", "roman", "'ls'"], "ls")]
// tailscale ssh wrapper.
#[case::tailscale(&["tailscale", "ssh", "roman", "'uptime'"], "uptime")]
#[case::tailscale_flags(&["tailscale", "ssh", "-p", "22", "user@roman", "df", "-h"], "df -h")]
// `--` ends option parsing; next token is the host.
#[case::dashdash(&["ssh", "--", "roman", "'ls'"], "ls")]
fn remote_ssh_payloads(#[case] tokens: &[&str], #[case] expected: &str) {
    check!(remote(tokens).as_deref() == Some(expected));
}

#[rstest]
// Interactive session, no remote command.
#[case::interactive(&["ssh", "roman"])]
#[case::interactive_flags(&["ssh", "-t", "roman"])]
// All options, no host.
#[case::only_flags(&["ssh", "-v"])]
// tailscale without the ssh subcommand.
#[case::tailscale_status(&["tailscale", "status"])]
// Remote command with an unexpanded parameter: fail open.
#[case::remote_dollar(&["ssh", "roman", "\"rm $X\""])]
fn remote_none(#[case] tokens: &[&str]) {
    check!(remote(tokens) == None);
}

#[rstest]
#[case::dash_c("-c", true)]
#[case::lc("-lc", true)]
#[case::ic("-ic", true)]
#[case::xec("-xec", true)]
#[case::no_c("-l", false)]
#[case::long_rcfile("--rcfile", false)]
#[case::long_command("--command", false)]
#[case::empty("-", false)]
#[case::with_digit("-c2", false)]
fn c_flag_detection(#[case] tok: &str, #[case] expected: bool) {
    check!(is_c_flag(tok) == expected);
}

#[rstest]
#[case::foo("FOO=bar", true)]
#[case::underscore("_X=1", true)]
#[case::empty_value("EMPTY=", true)]
#[case::no_eq("bash", false)]
#[case::leading_digit("1FOO=x", false)]
#[case::flag("-c", false)]
#[case::path("/bin/sh", false)]
fn assignment_detection(#[case] tok: &str, #[case] expected: bool) {
    check!(is_assignment(tok) == expected);
}

/// The ssh double-parse: a quoted multi-word operand is flattened, so it
/// reaches the remote shell as separate words. `nested_payload` reproduces
/// this, matching the behavior verified against the real ssh to `roman`.
#[test]
fn ssh_flattens_quoted_operand() {
    let out = remote(&["ssh", "roman", "echo", "'two three'"]);
    check!(out.as_deref() == Some("echo two three"));
}

proptest! {
    // Never panics and never indexes out of bounds on arbitrary argv.
    #[test]
    fn nested_payload_never_panics(tokens in prop::collection::vec("[-a-zA-Z0-9=@/_. ]{0,12}", 0..8)) {
        let _ = nested_payload(&tokens);
    }

    // ssh_host_index only ever points at a real, non-flag operand.
    #[test]
    fn ssh_host_index_points_at_operand(
        flags in prop::collection::vec(prop_oneof![
            Just("-v".to_string()), Just("-p".to_string()), Just("2222".to_string()),
            Just("-tt".to_string()), Just("-i".to_string()), Just("key".to_string()),
        ], 0..6),
        host in "[a-z][a-z0-9]{2,8}",
    ) {
        let mut args = flags.clone();
        args.push(host.clone());
        args.push("uptime".to_string());
        if let Some(idx) = ssh_host_index(&args) {
            // The chosen index is within bounds and its token does not start
            // with '-' (unless it followed `--`, not generated here).
            prop_assert!(idx < args.len());
            prop_assert!(!args[idx].starts_with('-'));
        }
    }

    // A single-quoted payload with no shell metacharacters round-trips: the
    // extracted string equals the inside of the quotes.
    #[test]
    fn single_quoted_payload_roundtrips(inner in "[a-zA-Z0-9 _./-]{1,20}") {
        let quoted = format!("'{inner}'");
        let tokens = vec!["bash".to_string(), "-c".to_string(), quoted];
        let got = match nested_payload(&tokens) {
            Some(Nested::Local(p)) => Some(p),
            _ => None,
        };
        prop_assert_eq!(got, Some(inner));
    }
}

#[test]
fn multi_level_shape() {
    // One level of extraction returns the next wrapper verbatim; callers drive
    // the recursion. `bash -c 'ssh roman uptime'` -> the ssh line.
    let p = local(&["bash", "-c", "'ssh roman uptime'"]);
    assert!(p.as_deref() == Some("ssh roman uptime"));
}
