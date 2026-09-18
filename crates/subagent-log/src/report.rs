//! Full-transcript report extraction. Only ever called for the transcript
//! the user has actually selected. See `Run::index` for why this stays out
//! of the startup scan.

use std::fs;
use std::path::Path;

use crate::event;

#[derive(Debug, PartialEq, Eq)]
pub enum Report {
    /// The transcript closed with a normal assistant reply.
    Clean(String),
    /// Ended on a `SendMessage` call instead of closing text.
    SendMessage(String),
    /// Neither applies: shows what the transcript was last doing.
    Inconclusive { last_seen: String },
}

impl Report {
    #[must_use]
    pub fn extract(path: &Path) -> Self {
        let Ok(raw) = fs::read_to_string(path) else {
            return Self::Inconclusive {
                last_seen: "(could not read transcript file)".to_string(),
            };
        };

        let mut last_assistant: Option<event::AssistantMessage> = None;
        for line in raw.lines() {
            if line.is_empty() {
                continue;
            }
            if let Some(message) = event::parse_assistant(line) {
                last_assistant = Some(message);
            }
        }

        let Some(message) = last_assistant else {
            return Self::Inconclusive {
                last_seen: "(no assistant turns found in this transcript)".to_string(),
            };
        };

        if message.closed_with_text() {
            return Self::Clean(message.text().unwrap_or_default().to_string());
        }

        if let Some(sent) = message.last_send_message() {
            return Self::SendMessage(sent.to_string());
        }

        Self::Inconclusive {
            last_seen: message.describe_trailing_action(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;
    use std::io::Write as _;

    fn fixture(lines: &[&str]) -> tempfile::NamedTempFile {
        let mut f = tempfile::NamedTempFile::new().unwrap();
        for line in lines {
            writeln!(f, "{line}").unwrap();
        }
        f
    }

    #[test]
    fn clean_report_extracts_final_text() {
        let f = fixture(&[
            r#"{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"first"}]}}"#,
            r###"{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"## Summary\nDone."}]}}"###,
        ]);
        let report = Report::extract(f.path());
        check!(report == Report::Clean("## Summary\nDone.".to_string()));
    }

    #[test]
    fn send_message_terminal_turn_falls_back_to_its_content() {
        let f = fixture(&[
            r#"{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"SendMessage","input":{"to":"peer","message":"the actual finding"}}]}}"#,
        ]);
        let report = Report::extract(f.path());
        check!(report == Report::SendMessage("the actual finding".to_string()));
    }

    #[test]
    fn stuck_wait_loop_is_inconclusive_not_a_stray_earlier_line() {
        let f = fixture(&[
            r#"{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Still waiting."}]}}"#,
            r#"{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_2","name":"Bash","input":{"command":"true","description":"Wait for status-check responses"}}]}}"#,
        ]);
        let report = Report::extract(f.path());
        check!(
            report
                == Report::Inconclusive {
                    last_seen: "(interrupted mid tool call Bash: Wait for status-check responses)"
                        .to_string()
                }
        );
    }

    #[test]
    fn empty_transcript_is_inconclusive() {
        let f = fixture(&[]);
        let report = Report::extract(f.path());
        check!(
            report
                == Report::Inconclusive {
                    last_seen: "(no assistant turns found in this transcript)".to_string()
                }
        );
    }
}
