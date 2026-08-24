//! The JSON payload Claude Code writes to stdin for every hook invocation.
//! Shared by the dispatch layer and every per-event handler so stdin is
//! parsed exactly once per invocation.

use serde::Deserialize;

/// Both `hook_event_name` and `tool_name` default rather than being required:
/// a payload missing either must still route the way it did before dispatch
/// existed, not silently parse-fail into a no-op.
#[derive(Debug, Clone, Default, Deserialize)]
pub struct Payload {
    #[serde(default)]
    pub hook_event_name: String,
    #[serde(default)]
    pub tool_name: String,
    #[serde(default)]
    pub session_id: Option<String>,
    #[serde(default)]
    pub cwd: Option<String>,
    #[serde(default)]
    pub tool_input: ToolInput,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct ToolInput {
    #[serde(default)]
    pub command: Option<String>,
    #[serde(default)]
    pub file_path: Option<String>,
}

#[cfg(test)]
mod tests;
