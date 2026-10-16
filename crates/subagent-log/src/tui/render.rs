//! Turns a parsed transcript into styled text. Runs on the preview worker
//! thread so the UI thread only ever draws finished lines.

use ratatui::style::{Color, Modifier, Style, Stylize as _};
use ratatui::text::{Line, Span, Text};

use crate::transcript::{Delivery, Entry, Report, Section, Transcript};

const RULE: &str = "\u{2500}\u{2500}";

pub fn report_text(report: &Report) -> Text<'static> {
    // A lone closing message is the whole report; anything else says how
    // it was delivered.
    let labelled = !matches!(report.sections.as_slice(), [Section::Closing(_)]);
    let mut lines: Vec<Line<'static>> = Vec::new();

    for (i, section) in report.sections.iter().enumerate() {
        if i > 0 {
            lines.push(Line::default());
        }
        if labelled {
            lines.push(section_header(section, report.sections.len() == 1));
            lines.push(Line::default());
        }
        match section {
            Section::Inconclusive(last_seen) => {
                lines.push(Line::from(last_seen.clone()).dim());
            }
            Section::Handback { text, .. }
            | Section::SendMessage(text)
            | Section::Closing(text) => lines.extend(markdown(text)),
        }
    }
    Text::from(lines)
}

fn section_header(section: &Section, alone: bool) -> Line<'static> {
    let (title, style) = match section {
        Section::Handback { delivery, .. } => (
            section.title(),
            match delivery {
                Delivery::Delivered => Style::new().fg(Color::Green),
                Delivery::Failed => Style::new().fg(Color::Red),
                Delivery::Unconfirmed => Style::new().fg(Color::Yellow),
            },
        ),
        Section::SendMessage(_) => (section.title(), Style::new().fg(Color::Yellow)),
        Section::Inconclusive(_) if alone => (
            "inconclusive: did not close normally".to_string(),
            Style::new().fg(Color::Red),
        ),
        Section::Closing(_) | Section::Inconclusive(_) => {
            (section.title(), Style::new().add_modifier(Modifier::DIM))
        }
    };
    Line::from(format!("{RULE} {title} {RULE}")).style(style.add_modifier(Modifier::BOLD))
}

pub fn transcript_text(transcript: &Transcript) -> Text<'static> {
    let mut lines: Vec<Line<'static>> = Vec::new();
    let mut seen_prompt = false;

    for entry in &transcript.entries {
        match entry {
            Entry::User(text) => {
                let title = if seen_prompt {
                    "message from caller"
                } else {
                    "prompt"
                };
                seen_prompt = true;
                block_header(&mut lines, title, Color::Blue);
                lines.extend(text.lines().map(|l| Line::from(l.to_string()).dim()));
            }
            Entry::Text(text) => {
                block_header(&mut lines, "assistant", Color::Cyan);
                lines.extend(markdown(text));
            }
            Entry::ToolCall { name, summary } => {
                let mut spans = vec![
                    Span::styled("  \u{203a} ", Style::new().fg(Color::DarkGray)),
                    Span::styled(name.clone(), Style::new().fg(Color::Yellow)),
                ];
                if !summary.is_empty() {
                    spans.push(Span::styled(
                        format!("  {summary}"),
                        Style::new().fg(Color::DarkGray),
                    ));
                }
                lines.push(Line::from(spans));
            }
            Entry::ToolResult { text, is_error } => {
                let style = if *is_error {
                    Style::new().fg(Color::Red)
                } else {
                    Style::new().fg(Color::DarkGray)
                };
                lines.extend(
                    text.lines()
                        .map(|l| Line::from(Span::styled(format!("    {l}"), style))),
                );
            }
            Entry::Handback { text, delivery } => {
                let (label, color) = match delivery {
                    Delivery::Delivered => ("handback", Color::Green),
                    Delivery::Failed => ("handback (not delivered)", Color::Red),
                    Delivery::Unconfirmed => ("handback (delivery unconfirmed)", Color::Yellow),
                };
                block_header(&mut lines, label, color);
                lines.extend(markdown(text));
            }
            Entry::SendMessage { to, text } => {
                let label = if to.is_empty() {
                    "SendMessage".to_string()
                } else {
                    format!("SendMessage to {to}")
                };
                block_header(&mut lines, &label, Color::Yellow);
                lines.extend(markdown(text));
            }
        }
    }

    if lines.is_empty() {
        lines.push(Line::from("(empty transcript)").dim());
    }
    Text::from(lines)
}

/// A blank separator (except at the very top) and a bold coloured label.
fn block_header(lines: &mut Vec<Line<'static>>, title: &str, color: Color) {
    if !lines.is_empty() {
        lines.push(Line::default());
    }
    lines.push(Line::from(format!("{RULE} {title} {RULE}")).style(Style::new().fg(color).bold()));
}

fn markdown(source: &str) -> Vec<Line<'static>> {
    tui_markdown::from_str(source)
        .lines
        .into_iter()
        .map(|line| Line {
            style: line.style,
            alignment: line.alignment,
            spans: line
                .spans
                .into_iter()
                .map(|span| Span::styled(span.content.into_owned(), span.style))
                .collect(),
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;

    fn plain(text: &Text<'_>) -> String {
        text.lines
            .iter()
            .map(|l| {
                l.spans
                    .iter()
                    .map(|s| s.content.as_ref())
                    .collect::<String>()
            })
            .collect::<Vec<_>>()
            .join("\n")
    }

    #[test]
    fn lone_closing_message_has_no_header() {
        let report = Report {
            sections: vec![Section::Closing("hello".to_string())],
        };
        check!(plain(&report_text(&report)).trim() == "hello");
    }

    #[test]
    fn several_sections_each_get_a_header() {
        let report = Report {
            sections: vec![
                Section::Handback {
                    number: 1,
                    total: 2,
                    text: "first".to_string(),
                    delivery: Delivery::Delivered,
                },
                Section::Handback {
                    number: 2,
                    total: 2,
                    text: "second".to_string(),
                    delivery: Delivery::Failed,
                },
            ],
        };
        let rendered = plain(&report_text(&report));
        check!(rendered.contains("handback 1/2"));
        check!(rendered.contains("handback 2/2 (not delivered)"));
        check!(rendered.contains("first"));
        check!(rendered.contains("second"));
    }

    #[test]
    fn lone_inconclusive_says_so_in_the_header() {
        let report = Report {
            sections: vec![Section::Inconclusive("(stuck)".to_string())],
        };
        let rendered = plain(&report_text(&report));
        check!(rendered.contains("inconclusive: did not close normally"));
        check!(rendered.contains("(stuck)"));
    }

    #[test]
    fn transcript_labels_the_first_user_entry_as_the_prompt() {
        let transcript = Transcript {
            entries: vec![
                Entry::User("do it".to_string()),
                Entry::User("also this".to_string()),
            ],
        };
        let rendered = plain(&transcript_text(&transcript));
        check!(rendered.contains("prompt"));
        check!(rendered.contains("message from caller"));
    }

    #[test]
    fn empty_transcript_still_renders_something() {
        check!(plain(&transcript_text(&Transcript::default())).contains("empty transcript"));
    }
}
