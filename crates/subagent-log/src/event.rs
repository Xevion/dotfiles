//! Cheap-first parsing of a single transcript (`.jsonl`) line: `cwd`
//! extraction is a raw substring scan, and assistant-turn parsing is gated
//! behind a substring check before a real deserialize (`#[serde(flatten)]`
//! forces serde_json to buffer the whole object instead of streaming it).

use serde::Deserialize;

/// `cwd` values are paths, never containing a literal `"`, so a scan to the
/// next quote is exact.
#[must_use]
pub fn extract_cwd(raw: &str) -> Option<&str> {
    let after_key = raw.split_once(r#""cwd":""#)?.1;
    let end = after_key.find('"')?;
    Some(&after_key[..end])
}

#[derive(Debug, Deserialize)]
pub struct AssistantMessage {
    #[serde(default)]
    pub content: Vec<ContentBlock>,
}

#[derive(Debug, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ContentBlock {
    Text {
        text: String,
    },
    ToolUse {
        name: String,
        input: serde_json::Value,
    },
    #[serde(other)]
    Other,
}

impl AssistantMessage {
    /// Ended by saying something, not trailing off mid tool-call.
    #[must_use]
    pub fn closed_with_text(&self) -> bool {
        matches!(self.content.last(), Some(ContentBlock::Text { .. }))
    }

    #[must_use]
    pub fn last_send_message(&self) -> Option<&str> {
        self.content.iter().rev().find_map(|block| match block {
            ContentBlock::ToolUse { name, input, .. } if name == "SendMessage" => {
                input.get("message").and_then(serde_json::Value::as_str)
            }
            _ => None,
        })
    }

    #[must_use]
    pub fn text(&self) -> Option<&str> {
        self.content.iter().find_map(|b| match b {
            ContentBlock::Text { text } => Some(text.as_str()),
            _ => None,
        })
    }

    #[must_use]
    pub fn describe_trailing_action(&self) -> String {
        match self.content.last() {
            Some(ContentBlock::ToolUse { name, input, .. }) => {
                let detail = input
                    .get("description")
                    .and_then(serde_json::Value::as_str)
                    .or_else(|| input.get("command").and_then(serde_json::Value::as_str))
                    .unwrap_or("");
                if detail.is_empty() {
                    format!("(interrupted mid tool call: {name})")
                } else {
                    format!("(interrupted mid tool call {name}: {detail})")
                }
            }
            _ => "(interrupted with no closing text)".to_string(),
        }
    }
}

#[derive(Debug, Deserialize)]
struct AssistantLine {
    message: AssistantMessage,
}

/// Tolerates malformed or truncated lines by returning `None`.
#[must_use]
pub fn parse_assistant(raw: &str) -> Option<AssistantMessage> {
    if !raw.contains(r#""type":"assistant""#) {
        return None;
    }
    serde_json::from_str::<AssistantLine>(raw)
        .ok()
        .map(|l| l.message)
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;

    #[test]
    fn extract_cwd_finds_the_value() {
        let line = r#"{"cwd":"/home/xevion/projects/foo","type":"user"}"#;
        check!(extract_cwd(line) == Some("/home/xevion/projects/foo"));
    }

    #[test]
    fn extract_cwd_absent_returns_none() {
        check!(extract_cwd(r#"{"type":"user"}"#) == None);
    }

    #[test]
    fn parse_assistant_skips_non_assistant_lines_without_deserializing() {
        check!(parse_assistant(r#"{"type":"user","message":{}}"#).is_none());
    }

    #[test]
    fn parse_assistant_parses_matching_lines() {
        let line = r#"{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}"#;
        let message = parse_assistant(line).unwrap();
        check!(message.closed_with_text());
    }
}
