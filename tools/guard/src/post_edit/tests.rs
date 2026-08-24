use super::*;
use assert2::check;

#[test]
fn stub_always_allows() {
    let payload = Payload {
        hook_event_name: "PostToolUse".into(),
        tool_name: "Edit".into(),
        session_id: None,
        cwd: None,
        tool_input: crate::payload::ToolInput {
            command: None,
            file_path: Some("src/main.rs".into()),
        },
    };
    check!(main(&payload) == 0);
}
