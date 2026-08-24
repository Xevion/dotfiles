use super::*;
use assert2::check;
use rstest::*;
use std::sync::atomic::{AtomicU32, Ordering};

static COUNTER: AtomicU32 = AtomicU32::new(0);

fn scratch_dir(label: &str) -> PathBuf {
    let n = COUNTER.fetch_add(1, Ordering::Relaxed);
    let path = std::env::temp_dir().join(format!(
        "guard-logging-test-{label}-{}-{n}",
        std::process::id()
    ));
    std::fs::create_dir_all(&path).expect("create scratch dir");
    path
}

fn sample_payload(cwd: &str) -> Payload {
    Payload {
        hook_event_name: "PreToolUse".into(),
        tool_name: "Bash".into(),
        session_id: Some("sess-1".into()),
        cwd: Some(cwd.into()),
        tool_input: crate::payload::ToolInput {
            command: Some("ls -la".into()),
            file_path: None,
        },
    }
}

#[rstest]
#[case::allow(0, "allow")]
#[case::block(2, "block")]
#[case::other_nonzero(127, "error")]
fn decision_label_maps_exit_codes(#[case] code: i32, #[case] expected: &str) {
    check!(decision_label(code) == expected);
}

#[rstest]
#[case::alphanumeric("my-project", "my-project")]
#[case::spaces_become_underscores("my project", "my_project")]
#[case::slashes_become_underscores("a/b", "a_b")]
#[case::preserves_dots("v1.2", "v1.2")]
fn sanitize_replaces_unsafe_characters(#[case] input: &str, #[case] expected: &str) {
    check!(sanitize(input) == expected);
}

#[test]
fn same_basename_different_paths_produce_different_slugs() {
    let a = project_slug(Path::new("/home/xevion/projects/foo"));
    let b = project_slug(Path::new("/home/other/workspace/foo"));
    check!(a != b);
    check!(a.starts_with("foo-"));
    check!(b.starts_with("foo-"));
}

#[test]
fn same_path_always_produces_the_same_slug() {
    let a = project_slug(Path::new("/home/xevion/projects/foo"));
    let b = project_slug(Path::new("/home/xevion/projects/foo"));
    check!(a == b);
}

#[test]
fn write_record_produces_a_valid_jsonl_line() {
    let home = scratch_dir("home");
    let project_dir = scratch_dir("project");
    let payload = sample_payload(project_dir.to_str().unwrap());

    write_record(&home, &payload, 0, Duration::from_millis(42)).expect("write record");

    let now = jiff::Zoned::now();
    let month = format!("{:04}-{:02}", now.year(), now.month());
    let project = project_slug(&project_dir);
    let log_path = home
        .join(".claude/hooks/logs")
        .join(&project)
        .join(format!("{month}.jsonl"));
    let contents = std::fs::read_to_string(&log_path).expect("read log file");
    let line = contents.lines().next().expect("one line");

    let value: serde_json::Value = serde_json::from_str(line).expect("valid json");
    check!(value["session_id"] == "sess-1");
    check!(value["event"] == "PreToolUse");
    check!(value["tool"] == "Bash");
    check!(value["decision"] == "allow");
    check!(value["duration_ms"] == 42);
    check!(value["rules"].as_array().unwrap().is_empty());
    check!(value["file"].is_null());

    let _ = std::fs::remove_dir_all(&home);
    let _ = std::fs::remove_dir_all(&project_dir);
}

#[test]
fn write_record_appends_multiple_lines() {
    let home = scratch_dir("home-append");
    let project_dir = scratch_dir("project-append");
    let payload = sample_payload(project_dir.to_str().unwrap());

    write_record(&home, &payload, 0, Duration::from_millis(1)).expect("first write");
    write_record(&home, &payload, 2, Duration::from_millis(2)).expect("second write");

    let now = jiff::Zoned::now();
    let month = format!("{:04}-{:02}", now.year(), now.month());
    let project = project_slug(&project_dir);
    let log_path = home
        .join(".claude/hooks/logs")
        .join(&project)
        .join(format!("{month}.jsonl"));
    let contents = std::fs::read_to_string(&log_path).expect("read log file");
    check!(contents.lines().count() == 2);

    let _ = std::fs::remove_dir_all(&home);
    let _ = std::fs::remove_dir_all(&project_dir);
}
