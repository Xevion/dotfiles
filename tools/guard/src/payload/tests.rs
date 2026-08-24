use super::*;
use assert2::{assert, check};

#[test]
fn full_bash_payload_parses() {
    let json = r#"{
        "hook_event_name": "PreToolUse",
        "tool_name": "Bash",
        "session_id": "abc123",
        "cwd": "/home/x/project",
        "tool_input": { "command": "ls -la" }
    }"#;
    let p: Payload = serde_json::from_str(json).expect("parse");
    check!(p.hook_event_name == "PreToolUse");
    check!(p.tool_name == "Bash");
    check!(p.session_id.as_deref() == Some("abc123"));
    check!(p.cwd.as_deref() == Some("/home/x/project"));
    check!(p.tool_input.command.as_deref() == Some("ls -la"));
    check!(p.tool_input.file_path.is_none());
}

#[test]
fn missing_optional_fields_default() {
    let json = r#"{ "hook_event_name": "PostToolUse", "tool_name": "Edit" }"#;
    let p: Payload = serde_json::from_str(json).expect("parse");
    check!(p.session_id.is_none());
    check!(p.cwd.is_none());
    check!(p.tool_input.command.is_none());
    check!(p.tool_input.file_path.is_none());
}

#[test]
fn edit_payload_carries_file_path() {
    let json = r#"{
        "hook_event_name": "PostToolUse",
        "tool_name": "Edit",
        "tool_input": { "file_path": "/home/x/project/src/main.rs" }
    }"#;
    let p: Payload = serde_json::from_str(json).expect("parse");
    assert!(p.tool_input.file_path.as_deref() == Some("/home/x/project/src/main.rs"));
}

#[test]
fn absent_event_name_parses_to_empty_string() {
    let json = r#"{ "tool_name": "Bash" }"#;
    let p: Payload = serde_json::from_str(json).expect("parse");
    check!(p.hook_event_name.is_empty());
    check!(p.tool_name == "Bash");
}

#[test]
fn empty_object_parses_to_defaults() {
    let p: Payload = serde_json::from_str("{}").expect("parse");
    check!(p.hook_event_name.is_empty());
    check!(p.tool_name.is_empty());
}
