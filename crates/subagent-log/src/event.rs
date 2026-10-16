//! Cheap-first parsing of single transcript (`.jsonl`) lines. Scans work on
//! raw bytes with substring searches (`cwd` extraction, line-kind gating) and
//! only deserialize the few lines that need it; `#[serde(flatten)]` is
//! avoided because it forces serde_json to buffer the whole object.
//!
//! Every transcript line carries exactly one content block, so a "message"
//! here is one line, and a single API turn is spread over several lines.

use std::sync::LazyLock;

use memchr::memmem::Finder;
use serde::Deserialize;
use serde_json::Value;

/// How a subagent returns its result to the agent that spawned it.
pub const HANDBACK_TOOL: &str = "SubagentHandback";
pub const SEND_MESSAGE_TOOL: &str = "SendMessage";

static CWD_KEY: LazyLock<Finder<'static>> = LazyLock::new(|| Finder::new(br#""cwd":""#));
static ASSISTANT_TAG: LazyLock<Finder<'static>> =
    LazyLock::new(|| Finder::new(br#""type":"assistant""#));
static USER_TAG: LazyLock<Finder<'static>> = LazyLock::new(|| Finder::new(br#""type":"user""#));
static HANDBACK_NAME: LazyLock<Finder<'static>> =
    LazyLock::new(|| Finder::new(br#""name":"SubagentHandback""#));

/// `cwd` values are paths, never containing a literal `"`, so a scan to the
/// next quote is exact.
#[must_use]
pub fn extract_cwd(raw: &[u8]) -> Option<&str> {
    let start = CWD_KEY.find(raw)? + CWD_KEY.needle().len();
    let rest = &raw[start..];
    let end = memchr::memchr(b'"', rest)?;
    std::str::from_utf8(&rest[..end]).ok()
}

#[must_use]
pub fn is_assistant_line(raw: &[u8]) -> bool {
    ASSISTANT_TAG.find(raw).is_some()
}

#[must_use]
pub fn is_user_line(raw: &[u8]) -> bool {
    USER_TAG.find(raw).is_some()
}

/// An assistant line that calls the handback tool. A substring test, so a
/// line merely mentioning the tool in prose does not match.
#[must_use]
pub fn mentions_handback_call(raw: &[u8]) -> bool {
    HANDBACK_NAME.find(raw).is_some()
}

#[derive(Debug, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ContentBlock {
    Text {
        text: String,
    },
    ToolUse {
        id: String,
        name: String,
        #[serde(default)]
        input: Value,
    },
    ToolResult {
        tool_use_id: String,
        #[serde(default)]
        content: Value,
        #[serde(default)]
        is_error: bool,
    },
    #[serde(other)]
    Other,
}

#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum RawContent {
    Text(String),
    Blocks(Vec<ContentBlock>),
}

fn deserialize_content<'de, D>(deserializer: D) -> Result<Vec<ContentBlock>, D::Error>
where
    D: serde::Deserializer<'de>,
{
    Ok(match RawContent::deserialize(deserializer)? {
        RawContent::Text(text) => vec![ContentBlock::Text { text }],
        RawContent::Blocks(blocks) => blocks,
    })
}

#[derive(Debug, Deserialize)]
pub struct Message {
    #[serde(default, deserialize_with = "deserialize_content")]
    pub content: Vec<ContentBlock>,
}

#[derive(Debug)]
pub enum Line {
    Assistant(Message),
    User(Message),
}

#[derive(Deserialize)]
struct Envelope {
    #[serde(rename = "type")]
    kind: String,
    message: Message,
}

/// Tolerates malformed or truncated lines (a transcript being written) by
/// returning `None`, as it does for every non-message line kind.
#[must_use]
pub fn parse_line(raw: &[u8]) -> Option<Line> {
    if !is_assistant_line(raw) && !is_user_line(raw) {
        return None;
    }
    let envelope: Envelope = serde_json::from_slice(raw).ok()?;
    match envelope.kind.as_str() {
        "assistant" => Some(Line::Assistant(envelope.message)),
        "user" => Some(Line::User(envelope.message)),
        _ => None,
    }
}

#[must_use]
pub fn parse_assistant(raw: &[u8]) -> Option<Message> {
    match parse_line(raw)? {
        Line::Assistant(message) => Some(message),
        Line::User(_) => None,
    }
}

/// How the last assistant turn left off.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Closure {
    /// Trailing off mid tool-call, or nothing seen yet.
    #[default]
    Open,
    /// Ended by saying something.
    Text,
    /// Ended on a delivered `SubagentHandback` call.
    HandedBack,
}

impl Closure {
    #[must_use]
    pub fn is_closed(self) -> bool {
        self != Self::Open
    }
}

impl Message {
    /// The id of a trailing handback call, if that is how this line ends.
    #[must_use]
    pub fn handback_call_id(&self) -> Option<&str> {
        match self.content.last() {
            Some(ContentBlock::ToolUse { id, name, .. }) if name == HANDBACK_TOOL => {
                Some(id.as_str())
            }
            _ => None,
        }
    }

    /// Classification of this line alone; a handback counts as delivered
    /// until a failed result for it says otherwise.
    #[must_use]
    pub fn closure(&self) -> Closure {
        match self.content.last() {
            Some(ContentBlock::Text { .. }) => Closure::Text,
            _ if self.handback_call_id().is_some() => Closure::HandedBack,
            _ => Closure::Open,
        }
    }

    /// Whether this user line holds a failed result for tool call `id`.
    #[must_use]
    pub fn fails_handback(&self, id: &str) -> bool {
        self.content.iter().any(|block| match block {
            ContentBlock::ToolResult {
                tool_use_id,
                content,
                is_error,
            } => {
                tool_use_id == id && (*is_error || handback_result_failed(&flatten_result(content)))
            }
            _ => false,
        })
    }
}

/// The handback tool answers with `{"success":false,"message":"..."}` when
/// it is not active for the calling agent, in which case nothing was sent.
/// A permission denial arrives instead as a plain-text `is_error` result,
/// which callers must check separately.
#[must_use]
pub fn handback_result_failed(result_text: &str) -> bool {
    serde_json::from_str::<Value>(result_text)
        .ok()
        .and_then(|v| v.get("success").and_then(Value::as_bool))
        .is_some_and(|ok| !ok)
}

/// Tool results are either a bare string or an array of `{type, text}` blocks.
#[must_use]
pub fn flatten_result(content: &Value) -> String {
    match content {
        Value::String(s) => s.clone(),
        Value::Array(blocks) => blocks
            .iter()
            .filter_map(|b| b.get("text").and_then(Value::as_str))
            .collect::<Vec<_>>()
            .join("\n"),
        _ => String::new(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::{assert, check};

    const HANDBACK_LINE: &str = r#"{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_h","name":"SubagentHandback","input":{"message":"the report"}}]}}"#;

    #[test]
    fn extract_cwd_finds_the_value() {
        let line = br#"{"cwd":"/home/xevion/projects/foo","type":"user"}"#;
        check!(extract_cwd(line) == Some("/home/xevion/projects/foo"));
    }

    #[test]
    fn extract_cwd_absent_returns_none() {
        check!(extract_cwd(br#"{"type":"user"}"#) == None);
    }

    #[test]
    fn extract_cwd_truncated_line_returns_none() {
        check!(extract_cwd(br#"{"cwd":"/home/xev"#) == None);
    }

    #[test]
    fn parse_skips_non_message_lines_without_deserializing() {
        check!(parse_line(br#"{"type":"attachment","attachment":{"type":"model"}}"#).is_none());
    }

    #[test]
    fn parse_assistant_parses_matching_lines() {
        let line = br#"{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}"#;
        let message = parse_assistant(line).unwrap();
        check!(message.closure() == Closure::Text);
    }

    #[test]
    fn parse_tolerates_truncated_lines() {
        check!(parse_line(br#"{"type":"assistant","message":{"content":[{"type":"te"#).is_none());
    }

    #[test]
    fn user_content_may_be_a_bare_string() {
        let line = br#"{"type":"user","message":{"role":"user","content":"do the thing"}}"#;
        assert!(let Some(Line::User(message)) = parse_line(line));
        assert!(let [ContentBlock::Text { text }] = message.content.as_slice());
        check!(text == "do the thing");
    }

    #[test]
    fn handback_call_is_recognized_and_closes_the_turn() {
        let message = parse_assistant(HANDBACK_LINE.as_bytes()).unwrap();
        check!(message.handback_call_id() == Some("toolu_h"));
        check!(message.closure() == Closure::HandedBack);
        check!(mentions_handback_call(HANDBACK_LINE.as_bytes()));
    }

    #[test]
    fn other_tool_call_leaves_the_turn_open() {
        let line = br#"{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t","name":"Bash","input":{}}]}}"#;
        let message = parse_assistant(line).unwrap();
        check!(message.closure() == Closure::Open);
        check!(message.handback_call_id() == None);
    }

    #[test]
    fn failed_handback_result_is_detected() {
        let line = br#"{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_h","content":[{"type":"text","text":"{\"success\":false,\"message\":\"Nothing was sent\"}"}]}]}}"#;
        assert!(let Some(Line::User(message)) = parse_line(line));
        check!(message.fails_handback("toolu_h"));
        check!(!message.fails_handback("toolu_other"));
    }

    #[test]
    fn denied_handback_is_a_failure_even_with_a_plain_text_body() {
        let line = br#"{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_h","is_error":true,"content":"Only the auto-mode classifier can allow SubagentHandback"}]}}"#;
        assert!(let Some(Line::User(message)) = parse_line(line));
        check!(message.fails_handback("toolu_h"));
    }

    #[test]
    fn successful_handback_result_is_not_a_failure() {
        let line = br#"{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_h","content":[{"type":"text","text":"{\"success\":true,\"message\":\"Report delivered to your caller.\"}"}]}]}}"#;
        assert!(let Some(Line::User(message)) = parse_line(line));
        check!(!message.fails_handback("toolu_h"));
    }
}
