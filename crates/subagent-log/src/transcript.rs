//! Full-transcript parse, only ever run for the transcript the user has
//! actually selected (see `index` for why the startup scan stays shallow).
//!
//! A [`Transcript`] is the ordered conversation; a [`Report`] is what the
//! run handed back, derived from it. A run can hand back several times, and
//! usually says something afterwards, so a report is a list of sections
//! rather than a single message.

use std::collections::HashMap;
use std::fmt::{self, Write as _};
use std::fs::File;
use std::io::{self, BufRead, BufReader};
use std::path::Path;

use serde_json::Value;

use crate::event::{self, ContentBlock, HANDBACK_TOOL, Line, SEND_MESSAGE_TOOL};

const RESULT_MAX_LINES: usize = 4;
const RESULT_MAX_CHARS: usize = 400;
const SUMMARY_MAX_CHARS: usize = 160;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Delivery {
    Delivered,
    /// The handback tool refused, so the caller never received it.
    Failed,
    /// No result was recorded (still in flight, or the transcript was cut).
    Unconfirmed,
}

#[derive(Debug, PartialEq, Eq)]
pub enum Entry {
    /// The task prompt, or a later message from the caller.
    User(String),
    Text(String),
    ToolCall {
        name: String,
        summary: String,
    },
    /// Clipped to a few lines: transcripts hold multi-megabyte results.
    ToolResult {
        text: String,
        is_error: bool,
    },
    Handback {
        text: String,
        delivery: Delivery,
    },
    SendMessage {
        to: String,
        text: String,
    },
}

#[derive(Debug, Default)]
pub struct Transcript {
    pub entries: Vec<Entry>,
}

impl Transcript {
    /// Unreadable lines (partially written, or invalid UTF-8) are skipped
    /// rather than failing the whole transcript.
    pub fn load(path: &Path) -> io::Result<Self> {
        Ok(Self::parse(BufReader::new(File::open(path)?)))
    }

    #[must_use]
    pub fn parse(mut reader: impl BufRead) -> Self {
        let mut entries = Vec::new();
        // Handback tool-use id -> index of its entry, awaiting a result.
        let mut awaiting_result: HashMap<String, usize> = HashMap::new();
        let mut buf = Vec::new();

        loop {
            buf.clear();
            match reader.read_until(b'\n', &mut buf) {
                Ok(0) | Err(_) => break,
                Ok(_) => {}
            }
            match event::parse_line(&buf) {
                Some(Line::Assistant(message)) => {
                    for block in message.content {
                        Self::push_assistant_block(&mut entries, &mut awaiting_result, block);
                    }
                }
                Some(Line::User(message)) => {
                    for block in message.content {
                        Self::push_user_block(&mut entries, &mut awaiting_result, block);
                    }
                }
                None => {}
            }
        }

        Self { entries }
    }

    fn push_assistant_block(
        entries: &mut Vec<Entry>,
        awaiting_result: &mut HashMap<String, usize>,
        block: ContentBlock,
    ) {
        match block {
            ContentBlock::Text { text } => entries.push(Entry::Text(text)),
            ContentBlock::ToolUse { id, name, input } if name == HANDBACK_TOOL => {
                awaiting_result.insert(id, entries.len());
                entries.push(Entry::Handback {
                    text: str_field(&input, "message").unwrap_or_default(),
                    delivery: Delivery::Unconfirmed,
                });
            }
            ContentBlock::ToolUse { name, input, .. } if name == SEND_MESSAGE_TOOL => {
                match (str_field(&input, "to"), str_field(&input, "message")) {
                    (to, Some(text)) => entries.push(Entry::SendMessage {
                        to: to.unwrap_or_default(),
                        text,
                    }),
                    (_, None) => entries.push(Entry::ToolCall {
                        summary: summarize_input(&input),
                        name,
                    }),
                }
            }
            ContentBlock::ToolUse { name, input, .. } => entries.push(Entry::ToolCall {
                summary: summarize_input(&input),
                name,
            }),
            ContentBlock::ToolResult { .. } | ContentBlock::Other => {}
        }
    }

    fn push_user_block(
        entries: &mut Vec<Entry>,
        awaiting_result: &mut HashMap<String, usize>,
        block: ContentBlock,
    ) {
        match block {
            ContentBlock::Text { text } => entries.push(Entry::User(text)),
            ContentBlock::ToolResult {
                tool_use_id,
                content,
                is_error,
            } => {
                let text = event::flatten_result(&content);
                if let Some(index) = awaiting_result.remove(&tool_use_id) {
                    if let Some(Entry::Handback { delivery, .. }) = entries.get_mut(index) {
                        *delivery = if is_error || event::handback_result_failed(&text) {
                            Delivery::Failed
                        } else {
                            Delivery::Delivered
                        };
                    }
                } else {
                    entries.push(Entry::ToolResult {
                        text: clip(&text),
                        is_error,
                    });
                }
            }
            ContentBlock::ToolUse { .. } | ContentBlock::Other => {}
        }
    }

    #[must_use]
    pub fn report(&self) -> Report {
        let handbacks: Vec<(&str, Delivery)> = self
            .entries
            .iter()
            .filter_map(|e| match e {
                Entry::Handback { text, delivery } => Some((text.as_str(), *delivery)),
                _ => None,
            })
            .collect();
        let total = handbacks.len();

        let mut sections: Vec<Section> = handbacks
            .into_iter()
            .enumerate()
            .map(|(i, (text, delivery))| Section::Handback {
                number: i + 1,
                total,
                text: text.to_string(),
                delivery,
            })
            .collect();

        if let Some(tail) = self.tail_section() {
            sections.push(tail);
        }
        Report { sections }
    }

    /// What the run said or did after its last handback (or in place of one):
    /// closing text, a final `SendMessage`, or a note on where it stopped.
    fn tail_section(&self) -> Option<Section> {
        // Results and caller messages (including "[Request interrupted by
        // user]") say nothing about what the run itself last did.
        let last = self
            .entries
            .iter()
            .rposition(|e| !matches!(e, Entry::ToolResult { .. } | Entry::User(_)));

        let Some(last) = last else {
            return Some(Section::Inconclusive(NO_TURNS.to_string()));
        };

        match &self.entries[last] {
            Entry::Text(_) => {
                let start = self.entries[..last]
                    .iter()
                    .rposition(|e| !matches!(e, Entry::Text(_)))
                    .map_or(0, |i| i + 1);
                let text = self.entries[start..=last]
                    .iter()
                    .filter_map(|e| match e {
                        Entry::Text(t) => Some(t.as_str()),
                        _ => None,
                    })
                    .collect::<Vec<_>>()
                    .join("\n\n");
                Some(Section::Closing(text))
            }
            Entry::SendMessage { text, .. } => Some(Section::SendMessage(text.clone())),
            Entry::Handback { .. } | Entry::ToolResult { .. } | Entry::User(_) => None,
            Entry::ToolCall { name, summary } => {
                Some(Section::Inconclusive(if summary.is_empty() {
                    format!("(interrupted mid tool call: {name})")
                } else {
                    format!("(interrupted mid tool call {name}: {summary})")
                }))
            }
        }
    }
}

const NO_TURNS: &str = "(no assistant turns found in this transcript)";

#[derive(Debug, PartialEq, Eq)]
pub enum Section {
    Handback {
        number: usize,
        total: usize,
        text: String,
        delivery: Delivery,
    },
    /// Ended on a `SendMessage` call instead of closing text.
    SendMessage(String),
    /// What the run said last, after any handbacks.
    Closing(String),
    /// Where the transcript stopped when it never closed normally.
    Inconclusive(String),
}

impl Section {
    #[must_use]
    pub fn title(&self) -> String {
        match self {
            Self::Handback {
                number,
                total,
                delivery,
                ..
            } => {
                let suffix = match delivery {
                    Delivery::Delivered => "",
                    Delivery::Failed => " (not delivered)",
                    Delivery::Unconfirmed => " (delivery unconfirmed)",
                };
                if *total > 1 {
                    format!("handback {number}/{total}{suffix}")
                } else {
                    format!("handback{suffix}")
                }
            }
            Self::SendMessage(_) => "ended on a SendMessage call".to_string(),
            Self::Closing(_) => "closing message".to_string(),
            Self::Inconclusive(_) => "inconclusive".to_string(),
        }
    }

    #[must_use]
    pub fn body(&self) -> &str {
        match self {
            Self::Handback { text, .. }
            | Self::SendMessage(text)
            | Self::Closing(text)
            | Self::Inconclusive(text) => text,
        }
    }
}

#[derive(Debug, PartialEq, Eq)]
pub struct Report {
    pub sections: Vec<Section>,
}

/// Plain-text form for the non-interactive commands. A lone section prints
/// bare; several get a `== title ==` line each.
impl fmt::Display for Report {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.sections.as_slice() {
            [Section::Inconclusive(last_seen)] => write!(
                f,
                "(inconclusive: this transcript did not close normally)\n{last_seen}"
            ),
            [only] => write!(f, "{}", only.body()),
            sections => {
                let rendered: Vec<String> = sections
                    .iter()
                    .map(|s| format!("== {} ==\n{}", s.title(), s.body()))
                    .collect();
                write!(f, "{}", rendered.join("\n\n"))
            }
        }
    }
}

fn str_field(input: &Value, key: &str) -> Option<String> {
    input.get(key).and_then(Value::as_str).map(str::to_string)
}

/// The most telling single field of a tool call's input, on one line.
fn summarize_input(input: &Value) -> String {
    const KEYS: [&str; 8] = [
        "description",
        "command",
        "file_path",
        "path",
        "pattern",
        "query",
        "url",
        "prompt",
    ];
    KEYS.iter()
        .find_map(|key| input.get(key).and_then(Value::as_str))
        .map(|s| truncate_chars(s.lines().next().unwrap_or_default(), SUMMARY_MAX_CHARS))
        .unwrap_or_default()
}

fn truncate_chars(s: &str, max: usize) -> String {
    if s.chars().count() <= max {
        return s.to_string();
    }
    let mut out: String = s.chars().take(max.saturating_sub(1)).collect();
    out.push('\u{2026}');
    out
}

fn clip(text: &str) -> String {
    let total_lines = text.lines().count();
    let head = text
        .lines()
        .take(RESULT_MAX_LINES)
        .collect::<Vec<_>>()
        .join("\n");
    let mut out = truncate_chars(&head, RESULT_MAX_CHARS);
    if total_lines > RESULT_MAX_LINES {
        let _ = write!(
            out,
            "\n\u{2026} (+{} more lines)",
            total_lines - RESULT_MAX_LINES
        );
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::{assert, check};

    fn parse(lines: &[&str]) -> Transcript {
        Transcript::parse(lines.join("\n").as_bytes())
    }

    fn text(t: &str) -> String {
        format!(
            r#"{{"type":"assistant","message":{{"role":"assistant","content":[{{"type":"text","text":"{t}"}}]}}}}"#
        )
    }

    fn tool(id: &str, name: &str, input: &str) -> String {
        format!(
            r#"{{"type":"assistant","message":{{"role":"assistant","content":[{{"type":"tool_use","id":"{id}","name":"{name}","input":{input}}}]}}}}"#
        )
    }

    fn handback(id: &str, message: &str) -> String {
        tool(
            id,
            "SubagentHandback",
            &format!(r#"{{"message":"{message}"}}"#),
        )
    }

    fn result(id: &str, body: &str) -> String {
        let escaped = body.replace('"', "\\\"");
        format!(
            r#"{{"type":"user","message":{{"role":"user","content":[{{"type":"tool_result","tool_use_id":"{id}","content":[{{"type":"text","text":"{escaped}"}}]}}]}}}}"#
        )
    }

    fn ok(id: &str) -> String {
        result(
            id,
            r#"{"success":true,"message":"Report delivered to your caller."}"#,
        )
    }

    fn refused(id: &str) -> String {
        result(
            id,
            r#"{"success":false,"message":"Nothing was sent: SubagentHandback is not active for this agent."}"#,
        )
    }

    fn sections(t: &Transcript) -> Vec<Section> {
        t.report().sections
    }

    fn delivered_handback(number: usize, total: usize, text: &str) -> Section {
        Section::Handback {
            number,
            total,
            text: text.to_string(),
            delivery: Delivery::Delivered,
        }
    }

    #[test]
    fn closing_text_alone_is_the_report() {
        let t = parse(&[&text("first"), &text("## Summary\\nDone.")]);
        check!(sections(&t) == vec![Section::Closing("first\n\n## Summary\nDone.".to_string())]);
    }

    #[test]
    fn closing_text_spans_only_the_trailing_run_of_text() {
        let t = parse(&[
            &text("early"),
            &tool("t1", "Bash", r#"{"command":"true"}"#),
            &result("t1", "ok"),
            &text("late"),
        ]);
        check!(sections(&t) == vec![Section::Closing("late".to_string())]);
    }

    #[test]
    fn handback_as_the_last_line_is_a_report_not_an_interruption() {
        let t = parse(&[&handback("h1", "the finding"), &ok("h1")]);
        check!(sections(&t) == vec![delivered_handback(1, 1, "the finding")]);
    }

    #[test]
    fn trailing_chatter_after_a_handback_does_not_replace_it() {
        let t = parse(&[
            &handback("h1", "the finding"),
            &ok("h1"),
            &text("Report delivered."),
        ]);
        check!(
            sections(&t)
                == vec![
                    delivered_handback(1, 1, "the finding"),
                    Section::Closing("Report delivered.".to_string()),
                ]
        );
    }

    #[test]
    fn repeated_handbacks_are_all_kept_in_order() {
        let t = parse(&[
            &handback("h1", "first"),
            &ok("h1"),
            &tool("t1", "Bash", r#"{"command":"true"}"#),
            &result("t1", "ok"),
            &handback("h2", "second"),
            &ok("h2"),
        ]);
        check!(
            sections(&t)
                == vec![
                    delivered_handback(1, 2, "first"),
                    delivered_handback(2, 2, "second")
                ]
        );
    }

    #[test]
    fn refused_handback_is_flagged_and_the_plain_text_report_follows() {
        let t = parse(&[
            &handback("h1", "stuck in the tool"),
            &refused("h1"),
            &text("the real report"),
        ]);
        check!(
            sections(&t)
                == vec![
                    Section::Handback {
                        number: 1,
                        total: 1,
                        text: "stuck in the tool".to_string(),
                        delivery: Delivery::Failed,
                    },
                    Section::Closing("the real report".to_string()),
                ]
        );
    }

    #[test]
    fn handback_without_a_result_is_unconfirmed() {
        let t = parse(&[&handback("h1", "cut off")]);
        let found = sections(&t);
        assert!(let [Section::Handback { delivery, .. }] = found.as_slice());
        check!(*delivery == Delivery::Unconfirmed);
    }

    #[test]
    fn handback_results_do_not_show_up_as_tool_results() {
        let t = parse(&[&handback("h1", "x"), &ok("h1")]);
        check!(
            !t.entries
                .iter()
                .any(|e| matches!(e, Entry::ToolResult { .. }))
        );
    }

    #[test]
    fn send_message_terminal_turn_is_the_report() {
        let t = parse(&[&tool(
            "s1",
            "SendMessage",
            r#"{"to":"peer","message":"the actual finding"}"#,
        )]);
        check!(sections(&t) == vec![Section::SendMessage("the actual finding".to_string())]);
    }

    #[test]
    fn stuck_wait_loop_is_inconclusive_not_a_stray_earlier_line() {
        let t = parse(&[
            &text("Still waiting."),
            &tool(
                "t2",
                "Bash",
                r#"{"command":"true","description":"Wait for status-check responses"}"#,
            ),
        ]);
        check!(
            sections(&t)
                == vec![Section::Inconclusive(
                    "(interrupted mid tool call Bash: Wait for status-check responses)".to_string()
                )]
        );
    }

    #[test]
    fn died_after_handing_back_keeps_the_handback_and_notes_where_it_stopped() {
        let t = parse(&[
            &handback("h1", "done"),
            &ok("h1"),
            &tool("t2", "Bash", r#"{"command":"sleep 99"}"#),
        ]);
        check!(
            sections(&t)
                == vec![
                    delivered_handback(1, 1, "done"),
                    Section::Inconclusive("(interrupted mid tool call Bash: sleep 99)".to_string()),
                ]
        );
    }

    #[test]
    fn denied_handback_is_flagged_as_not_delivered() {
        let denied = r#"{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"h1","is_error":true,"content":"Only the auto-mode classifier can allow SubagentHandback"}]}}"#;
        let t = parse(&[&handback("h1", "never arrived"), denied]);
        let report = t.report();
        assert!(let [Section::Handback { delivery, .. }] = report.sections.as_slice());
        check!(*delivery == Delivery::Failed);
    }

    #[test]
    fn caller_interrupt_after_a_tool_call_still_reports_that_call() {
        let interrupt = r#"{"type":"user","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user for tool use]"}]}}"#;
        let t = parse(&[&tool("t1", "Bash", r#"{"command":"sleep 99"}"#), interrupt]);
        check!(
            sections(&t)
                == vec![Section::Inconclusive(
                    "(interrupted mid tool call Bash: sleep 99)".to_string()
                )]
        );
    }

    #[test]
    fn prompt_only_transcript_has_no_assistant_turns() {
        let prompt = r#"{"type":"user","message":{"role":"user","content":"go"}}"#;
        check!(sections(&parse(&[prompt])) == vec![Section::Inconclusive(NO_TURNS.to_string())]);
    }

    #[test]
    fn empty_transcript_is_inconclusive() {
        check!(sections(&parse(&[])) == vec![Section::Inconclusive(NO_TURNS.to_string())]);
    }

    #[test]
    fn malformed_lines_are_skipped() {
        let t = parse(&[
            "not json",
            &text("survived"),
            r#"{"type":"assistant","message":{"con"#,
        ]);
        check!(sections(&t) == vec![Section::Closing("survived".to_string())]);
    }

    #[test]
    fn long_tool_results_are_clipped() {
        let body = (1..=10)
            .map(|n| format!("line {n}"))
            .collect::<Vec<_>>()
            .join("\\n");
        let t = parse(&[&tool("t1", "Bash", "{}"), &result("t1", &body)]);
        assert!(let Some(Entry::ToolResult { text, .. }) = t.entries.last());
        check!(text.lines().count() == RESULT_MAX_LINES + 1);
        check!(text.ends_with("(+6 more lines)"));
    }

    #[test]
    fn display_labels_sections_only_when_there_are_several() {
        let lone = Report {
            sections: vec![Section::Closing("just this".to_string())],
        };
        check!(lone.to_string() == "just this");

        let several = Report {
            sections: vec![
                delivered_handback(1, 1, "report"),
                Section::Closing("bye".to_string()),
            ],
        };
        check!(several.to_string() == "== handback ==\nreport\n\n== closing message ==\nbye");
    }
}
