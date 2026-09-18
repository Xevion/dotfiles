use std::collections::HashSet;
use std::time::SystemTime;

use ratatui::Frame;
use ratatui::layout::{Alignment, Constraint, Layout, Rect};
use ratatui::style::{Color, Modifier, Style, Stylize as _};
use ratatui::text::{Line, Span, Text};
use ratatui::widgets::{Block, Borders, List, ListItem, Paragraph, Wrap};

use super::{App, Mode};
use crate::index::Run;
use crate::index::state::RunState;
use crate::paths::ShortenExt as _;
use crate::report::Report;
use crate::time_fmt::TimestampExt as _;

const BADGE_WIDTH: usize = 6;
const TIME_WIDTH: usize = 32;
const PATH_WIDTH: usize = 26;

impl App {
    pub(super) fn render(&self, frame: &mut Frame) {
        let [header, body, footer] = Layout::vertical([
            Constraint::Length(1),
            Constraint::Min(0),
            Constraint::Length(1),
        ])
        .areas(frame.area());

        self.render_header(frame, header);

        match self.mode {
            Mode::Browsing => {
                let [list_area, preview_area] =
                    Layout::horizontal([Constraint::Percentage(45), Constraint::Percentage(55)])
                        .areas(body);
                self.render_list(frame, list_area);
                self.render_preview(frame, preview_area);
            }
            Mode::Viewing => self.render_preview(frame, body),
            Mode::ProjectJump => {
                let [list_area, preview_area] =
                    Layout::horizontal([Constraint::Percentage(45), Constraint::Percentage(55)])
                        .areas(body);
                self.render_list(frame, list_area);
                self.render_preview(frame, preview_area);
                self.render_project_jump(frame, body);
            }
        }

        self.render_footer(frame, footer);
    }

    fn render_header(&self, frame: &mut Frame, area: Rect) {
        let scope = match &self.filter {
            super::Filter::CurrentProject => self.cwd.shorten(&self.home),
            super::Filter::Project(dir) => self
                .all_runs
                .iter()
                .position(|r| &r.project_dir == dir)
                .map_or_else(|| dir.display().to_string(), |i| self.display_path(i)),
            super::Filter::AllProjects => "all projects".to_string(),
        };
        let text = if self.loading {
            format!(" subagent-log: {scope} (scanning\u{2026})")
        } else {
            format!(" subagent-log: {scope} ({} runs)", self.visible.len())
        };
        frame.render_widget(Line::from(text).bold(), area);
    }

    fn render_footer(&self, frame: &mut Frame, area: Rect) {
        let hint = match self.mode {
            Mode::Browsing => {
                let all_label = if matches!(self.filter, super::Filter::AllProjects) {
                    "this project"
                } else {
                    "all projects"
                };
                let parent_hint = if self.selected_has_parent() {
                    "  p: parent"
                } else {
                    ""
                };
                format!(
                    "enter: fullscreen  \u{2191}/\u{2193}: move  pgup/pgdn: scroll preview  a: jump to project  A: {all_label}{parent_hint}  q: quit"
                )
            }
            Mode::Viewing => {
                "enter/esc/q: back to split  \u{2191}/\u{2193}: scroll  pgup/pgdn: page  home/end: top/bottom"
                    .to_string()
            }
            Mode::ProjectJump => "enter: select  esc: cancel".to_string(),
        };
        frame.render_widget(Line::from(hint).dim(), area);
    }

    fn render_list(&self, frame: &mut Frame, area: Rect) {
        let family = self
            .selected_run_agent_id()
            .map(|id| self.family_ids(id))
            .unwrap_or_default();

        let now = SystemTime::now();
        let items: Vec<ListItem> = self
            .visible
            .iter()
            .map(|&i| {
                let run = &self.all_runs[i];
                let linked = family.contains(&run.agent_id);
                let path = self.display_path(i);
                list_row(run, &path, now, linked)
            })
            .collect();

        let list = List::new(items)
            .block(Block::default().borders(Borders::NONE))
            .highlight_style(Style::default().add_modifier(Modifier::REVERSED));

        let mut state = self.list_state;
        frame.render_stateful_widget(list, area, &mut state);
    }

    fn family_ids(&self, agent_id: &str) -> HashSet<String> {
        let mut ids = HashSet::new();
        ids.insert(agent_id.to_string());
        if let Some(run) = self.all_runs.iter().find(|r| r.agent_id == agent_id)
            && let Some(parent) = &run.meta.parent_agent_id
        {
            ids.insert(parent.clone());
        }
        for run in &self.all_runs {
            if run.meta.parent_agent_id.as_deref() == Some(agent_id) {
                ids.insert(run.agent_id.clone());
            }
        }
        ids
    }

    fn render_preview(&self, frame: &mut Frame, area: Rect) {
        let block =
            Block::default()
                .borders(Borders::ALL)
                .title(self.preview.as_ref().map_or_else(
                    || " (nothing selected) ".to_string(),
                    |p| format!(" {} ", p.title),
                ));

        let Some(preview) = &self.preview else {
            frame.render_widget(block, area);
            return;
        };

        let inner = block.inner(area);
        frame.render_widget(block, area);

        let body = match &preview.report {
            Report::Clean(markdown) => tui_markdown::from_str(markdown),
            Report::SendMessage(message) => {
                let mut t = Text::from(
                    Line::from("ended on a SendMessage call, not closing text:")
                        .italic()
                        .dim(),
                );
                t.lines.push(Line::from(""));
                t.extend(tui_markdown::from_str(message));
                t
            }
            Report::Inconclusive { last_seen } => Text::from(vec![
                Line::from("inconclusive: did not close normally")
                    .red()
                    .bold(),
                Line::from(""),
                Line::from(last_seen.clone()).dim(),
            ]),
        };

        let paragraph = Paragraph::new(body)
            .wrap(Wrap { trim: false })
            .scroll((self.scroll, 0));
        frame.render_widget(paragraph, inner);
    }

    fn render_project_jump(&self, frame: &mut Frame, area: Rect) {
        let popup = centered(area, 70, 60);
        frame.render_widget(ratatui::widgets::Clear, popup);

        let now = SystemTime::now();
        let items: Vec<ListItem> = self
            .project_summaries
            .iter()
            .map(|s| {
                let when = s.most_recent.relative(now);
                ListItem::new(Line::from(vec![
                    Span::raw(format!(" {:<32}", s.display_path)),
                    Span::styled(
                        format!("{:>4} runs", s.count),
                        Style::default().fg(Color::DarkGray),
                    ),
                    Span::raw(format!("  {when}")),
                ]))
            })
            .collect();

        let list = List::new(items)
            .block(
                Block::default()
                    .borders(Borders::ALL)
                    .title(" jump to project ")
                    .title_alignment(Alignment::Center),
            )
            .highlight_style(Style::default().add_modifier(Modifier::REVERSED));

        let mut state = self.project_jump_state;
        frame.render_stateful_widget(list, popup, &mut state);
    }
}

fn list_row(run: &Run, path: &str, now: SystemTime, linked: bool) -> ListItem<'static> {
    let (badge, color) = match run.state {
        RunState::Done => ("done", Color::Green),
        RunState::Active => ("active", Color::Cyan),
        RunState::Stalled => ("stall", Color::Yellow),
        RunState::Interrupted => ("dead", Color::Red),
    };
    let when = pad_or_truncate(&run.mtime.display(now), TIME_WIDTH);

    let mut style = Style::default();
    if linked {
        style = style.add_modifier(Modifier::BOLD).fg(Color::Magenta);
    }

    let mut spans = vec![
        Span::styled(
            format!(" {}", pad_or_truncate(badge, BADGE_WIDTH)),
            Style::default().fg(color),
        ),
        Span::raw(format!(" {when}")),
        Span::raw(" "),
    ];
    spans.extend(path_column(path, PATH_WIDTH));
    spans.push(Span::raw(format!(
        " {}{}: {}",
        depth_marker(run),
        run.meta.agent_type,
        run.meta.description
    )));

    ListItem::new(Line::from(spans).patch_style(style))
}

fn depth_marker(run: &Run) -> String {
    let mut marker = if run.meta.spawn_depth > 1 {
        format!(
            "{}\u{2514} ",
            "  ".repeat((run.meta.spawn_depth - 1) as usize)
        )
    } else {
        String::new()
    };
    if run.meta.is_fork {
        marker.push_str("\u{1f500} ");
    }
    marker
}

fn path_column(path: &str, width: usize) -> Vec<Span<'static>> {
    let fitted = fit(path, width);
    let pad = width.saturating_sub(fitted.chars().count());
    let mut spans = match fitted.rfind('/') {
        Some(idx) if idx + 1 < fitted.len() => {
            let (prefix, leaf) = fitted.split_at(idx + 1);
            vec![
                Span::styled(prefix.to_string(), Style::default().fg(Color::DarkGray)),
                Span::styled(
                    leaf.to_string(),
                    Style::default()
                        .fg(project_color(leaf))
                        .add_modifier(Modifier::BOLD),
                ),
            ]
        }
        _ => vec![Span::styled(fitted, Style::default().fg(Color::DarkGray))],
    };
    if pad > 0 {
        spans.push(Span::raw(" ".repeat(pad)));
    }
    spans
}

const PROJECT_PALETTE: [Color; 6] = [
    Color::Cyan,
    Color::LightGreen,
    Color::LightYellow,
    Color::LightBlue,
    Color::LightMagenta,
    Color::LightCyan,
];

fn project_color(name: &str) -> Color {
    let hash = name.bytes().fold(0u32, |acc, b| {
        acc.wrapping_mul(31).wrapping_add(u32::from(b))
    });
    PROJECT_PALETTE[(hash as usize) % PROJECT_PALETTE.len()]
}

/// `{:<width}` only pads, never truncates.
fn fit(s: &str, width: usize) -> String {
    let char_count = s.chars().count();
    if char_count <= width {
        return s.to_string();
    }
    if width == 0 {
        return String::new();
    }
    let mut truncated: String = s.chars().take(width.saturating_sub(1)).collect();
    truncated.push('\u{2026}');
    truncated
}

fn pad_or_truncate(s: &str, width: usize) -> String {
    let fitted = fit(s, width);
    let pad = width.saturating_sub(fitted.chars().count());
    if pad > 0 {
        format!("{fitted}{}", " ".repeat(pad))
    } else {
        fitted
    }
}

fn centered(area: Rect, percent_x: u16, percent_y: u16) -> Rect {
    let [_, vertical, _] = Layout::vertical([
        Constraint::Percentage((100 - percent_y) / 2),
        Constraint::Percentage(percent_y),
        Constraint::Percentage((100 - percent_y) / 2),
    ])
    .areas(area);
    let [_, horizontal, _] = Layout::horizontal([
        Constraint::Percentage((100 - percent_x) / 2),
        Constraint::Percentage(percent_x),
        Constraint::Percentage((100 - percent_x) / 2),
    ])
    .areas(vertical);
    horizontal
}
