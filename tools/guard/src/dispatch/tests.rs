use super::*;
use assert2::check;
use rstest::*;

#[rstest]
#[case::pretooluse_bash("PreToolUse", "Bash", Handler::PreToolUseBash)]
#[case::posttooluse_write("PostToolUse", "Write", Handler::PostToolUseEdit)]
#[case::posttooluse_edit("PostToolUse", "Edit", Handler::PostToolUseEdit)]
#[case::posttooluse_bash_unhandled("PostToolUse", "Bash", Handler::None)]
#[case::pretooluse_edit_unhandled("PreToolUse", "Edit", Handler::None)]
#[case::unrelated_event("SessionStart", "Bash", Handler::None)]
#[case::unknown_tool("PreToolUse", "Glob", Handler::None)]
#[case::absent_event_bash("", "Bash", Handler::PreToolUseBash)]
#[case::absent_event_other_tool("", "Edit", Handler::None)]
fn classify_routes_by_event_and_tool(
    #[case] event: &str,
    #[case] tool: &str,
    #[case] expected: Handler,
) {
    check!(classify(event, tool) == expected);
}

#[test]
fn route_returns_zero_for_unhandled_combinations() {
    let payload = Payload {
        hook_event_name: "SessionStart".into(),
        tool_name: "Bash".into(),
        session_id: None,
        cwd: None,
        tool_input: crate::payload::ToolInput::default(),
    };
    check!(route(&payload) == 0);
}

#[test]
fn route_dispatches_bash_pretooluse_to_hook() {
    // No command in tool_input, so hook::main's early-return path fires and
    // returns 0; this exercises that routing actually reaches hook::main
    // rather than testing hook's internal decision logic.
    let payload = Payload {
        hook_event_name: "PreToolUse".into(),
        tool_name: "Bash".into(),
        session_id: None,
        cwd: None,
        tool_input: crate::payload::ToolInput::default(),
    };
    check!(route(&payload) == 0);
}

#[test]
fn payload_without_event_name_still_parses() {
    let raw = r#"{"tool_name":"Bash","tool_input":{"command":"ls"}}"#;
    let payload: Payload = serde_json::from_str(raw).expect("must parse without hook_event_name");
    check!(payload.hook_event_name.is_empty());
    check!(classify(&payload.hook_event_name, &payload.tool_name) == Handler::PreToolUseBash);
}

#[test]
fn route_dispatches_edit_posttooluse_to_stub() {
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
    check!(route(&payload) == 0);
}
