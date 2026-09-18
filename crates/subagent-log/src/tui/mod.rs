mod view;

use std::io::BufRead as _;
use std::path::{Path, PathBuf};
use std::time::SystemTime;

use crossbeam_channel::{Receiver, Sender};
use crossterm::event::{Event, KeyCode, KeyEventKind};
use ratatui::widgets::ListState;
use snafu::ResultExt as _;

use crate::error::{AppError, DrawSnafu, WatchSnafu};
use crate::event;
use crate::index::Run;
use crate::paths::ShortenExt as _;
use crate::report::Report;
use crate::watch::Watch;

pub enum Mode {
    Browsing,
    Viewing,
    ProjectJump,
}

pub enum Filter {
    CurrentProject,
    Project(PathBuf),
    AllProjects,
}

pub struct ProjectSummary {
    pub project_dir: PathBuf,
    pub display_path: String,
    pub count: usize,
    pub most_recent: SystemTime,
}

pub struct Preview {
    pub agent_id: String,
    pub title: String,
    pub report: Report,
}

pub struct App {
    pub home: PathBuf,
    pub cwd: PathBuf,
    pub generic_roots: Vec<PathBuf>,

    pub all_runs: Vec<Run>,
    /// Per-run, not session-wide: `Run::session_cwds` can point at a stale
    /// project path after a rename.
    pub display_paths: Vec<Option<PathBuf>>,
    pub filter: Filter,
    pub visible: Vec<usize>,
    pub list_state: ListState,

    pub mode: Mode,
    pub project_summaries: Vec<ProjectSummary>,
    pub project_jump_state: ListState,

    pub preview: Option<Preview>,
    pub scroll: u16,
    pub should_quit: bool,
    pub loading: bool,
}

enum AppEvent {
    Input(Event),
    Refresh,
    IndexReady(Vec<Run>, Vec<Option<PathBuf>>),
}

impl App {
    fn new(home: PathBuf, cwd: PathBuf, all: bool) -> Self {
        let generic_roots = Run::generic_roots(&home);
        Self {
            home,
            cwd,
            generic_roots,
            all_runs: Vec::new(),
            display_paths: Vec::new(),
            filter: if all {
                Filter::AllProjects
            } else {
                Filter::CurrentProject
            },
            visible: Vec::new(),
            list_state: ListState::default(),
            mode: Mode::Browsing,
            project_summaries: Vec::new(),
            project_jump_state: ListState::default(),
            preview: None,
            scroll: 0,
            should_quit: false,
            loading: true,
        }
    }

    pub fn run(
        claude_projects_root: &Path,
        home: PathBuf,
        cwd: PathBuf,
        all: bool,
    ) -> Result<(), AppError> {
        let (tx, rx): (Sender<AppEvent>, Receiver<AppEvent>) = crossbeam_channel::unbounded();

        let input_tx = tx.clone();
        std::thread::spawn(move || {
            while let Ok(ev) = crossterm::event::read() {
                if input_tx.send(AppEvent::Input(ev)).is_err() {
                    break;
                }
            }
        });

        let (watch_tx, watch_rx) = crossbeam_channel::unbounded();
        let _watch = Watch::spawn(claude_projects_root, watch_tx).context(WatchSnafu {
            path: claude_projects_root.display().to_string(),
        })?;
        let refresh_tx = tx.clone();
        std::thread::spawn(move || {
            while watch_rx.recv().is_ok() {
                if refresh_tx.send(AppEvent::Refresh).is_err() {
                    break;
                }
            }
        });

        let mut app = Self::new(home, cwd, all);
        spawn_scan(claude_projects_root.to_path_buf(), tx.clone());

        let mut terminal = ratatui::init();
        let outcome = (|| -> Result<(), AppError> {
            loop {
                terminal
                    .draw(|frame| app.render(frame))
                    .context(DrawSnafu)?;
                if app.should_quit {
                    return Ok(());
                }
                match rx.recv() {
                    Ok(AppEvent::Input(Event::Key(key))) if key.kind == KeyEventKind::Press => {
                        app.handle_key(key.code);
                    }
                    Ok(AppEvent::Refresh) => {
                        spawn_scan(claude_projects_root.to_path_buf(), tx.clone());
                    }
                    Ok(AppEvent::IndexReady(runs, display_paths)) => {
                        app.apply_index(runs, display_paths);
                    }
                    Ok(_) => {}
                    Err(_) => return Ok(()),
                }
            }
        })();
        ratatui::restore();
        outcome
    }

    fn apply_index(&mut self, runs: Vec<Run>, display_paths: Vec<Option<PathBuf>>) {
        self.all_runs = runs;
        self.display_paths = display_paths;
        self.loading = false;
        self.recompute_visible();
    }

    fn recompute_visible(&mut self) {
        self.visible = self
            .all_runs
            .iter()
            .enumerate()
            .filter(|(_, r)| match &self.filter {
                Filter::CurrentProject => r.matches_cwd(&self.cwd, &self.generic_roots),
                Filter::Project(dir) => &r.project_dir == dir,
                Filter::AllProjects => true,
            })
            .map(|(i, _)| i)
            .collect();
        let selected = self
            .list_state
            .selected()
            .unwrap_or(0)
            .min(self.visible.len().saturating_sub(1));
        self.list_state.select(if self.visible.is_empty() {
            None
        } else {
            Some(selected)
        });
        self.sync_preview();
    }

    fn selected_run(&self) -> Option<&Run> {
        let idx = self.list_state.selected()?;
        self.visible.get(idx).map(|&i| &self.all_runs[i])
    }

    pub(super) fn selected_run_agent_id(&self) -> Option<&str> {
        self.selected_run().map(|r| r.agent_id.as_str())
    }

    pub(super) fn selected_has_parent(&self) -> bool {
        self.selected_run()
            .is_some_and(|r| r.meta.parent_agent_id.is_some())
    }

    pub(super) fn display_path(&self, index: usize) -> String {
        self.display_paths[index].as_deref().map_or_else(
            || {
                self.all_runs[index].session_cwds.first().map_or_else(
                    || self.all_runs[index].project_dir.display().to_string(),
                    |p| p.shorten(&self.home),
                )
            },
            |p| p.shorten(&self.home),
        )
    }

    fn sync_preview(&mut self) {
        let Some(run) = self.selected_run() else {
            self.preview = None;
            self.scroll = 0;
            return;
        };
        if self
            .preview
            .as_ref()
            .is_some_and(|p| p.agent_id == run.agent_id)
        {
            return;
        }
        self.preview = Some(Preview {
            agent_id: run.agent_id.clone(),
            title: format!("{}: {}", run.meta.agent_type, run.meta.description),
            report: Report::extract(&run.path),
        });
        self.scroll = 0;
    }

    fn move_cursor(&mut self, delta: isize) {
        if self.visible.is_empty() {
            return;
        }
        let len = self.visible.len();
        let current = self.list_state.selected().unwrap_or(0);
        let steps = delta.unsigned_abs() % len;
        let next = if delta.is_negative() {
            (current + len - steps) % len
        } else {
            (current + steps) % len
        };
        self.list_state.select(Some(next));
        self.sync_preview();
    }

    fn scroll_by(&mut self, delta: i16) {
        self.scroll = self.scroll.saturating_add_signed(delta);
    }

    fn jump_to_parent(&mut self) {
        let Some(run) = self.selected_run() else {
            return;
        };
        let Some(parent_id) = run.meta.parent_agent_id.clone() else {
            return;
        };

        if let Some(idx) = self
            .visible
            .iter()
            .position(|&i| self.all_runs[i].agent_id == parent_id)
        {
            self.list_state.select(Some(idx));
            self.sync_preview();
            return;
        }

        if let Some(i) = self.all_runs.iter().position(|r| r.agent_id == parent_id) {
            self.filter = Filter::Project(self.all_runs[i].project_dir.clone());
            self.recompute_visible();
            if let Some(idx) = self.visible.iter().position(|&v| v == i) {
                self.list_state.select(Some(idx));
                self.sync_preview();
            }
        }
    }

    fn open_project_jump(&mut self) {
        let mut by_project: Vec<(PathBuf, usize, SystemTime)> = Vec::new();
        for run in &self.all_runs {
            match by_project.iter_mut().find(|(d, ..)| *d == run.project_dir) {
                Some((_, count, most_recent)) => {
                    *count += 1;
                    *most_recent = (*most_recent).max(run.mtime);
                }
                None => by_project.push((run.project_dir.clone(), 1, run.mtime)),
            }
        }
        by_project.sort_by_key(|(_, _, most_recent)| std::cmp::Reverse(*most_recent));
        self.project_summaries = by_project
            .into_iter()
            .map(|(project_dir, count, most_recent)| {
                let display_path = self
                    .all_runs
                    .iter()
                    .position(|r| r.project_dir == project_dir)
                    .map_or_else(
                        || project_dir.display().to_string(),
                        |i| self.display_path(i),
                    );
                ProjectSummary {
                    project_dir,
                    display_path,
                    count,
                    most_recent,
                }
            })
            .collect();
        self.project_jump_state
            .select(if self.project_summaries.is_empty() {
                None
            } else {
                Some(0)
            });
        self.mode = Mode::ProjectJump;
    }

    fn handle_key(&mut self, code: KeyCode) {
        match self.mode {
            Mode::Browsing => self.handle_browsing_key(code),
            Mode::Viewing => self.handle_viewing_key(code),
            Mode::ProjectJump => self.handle_project_jump_key(code),
        }
    }

    fn handle_browsing_key(&mut self, code: KeyCode) {
        match code {
            KeyCode::Char('q') | KeyCode::Esc => self.should_quit = true,
            KeyCode::Up | KeyCode::Char('k') => self.move_cursor(-1),
            KeyCode::Down | KeyCode::Char('j') => self.move_cursor(1),
            KeyCode::PageUp => self.scroll_by(-10),
            KeyCode::PageDown => self.scroll_by(10),
            KeyCode::Enter => self.mode = Mode::Viewing,
            KeyCode::Char('a') => self.open_project_jump(),
            KeyCode::Char('A') => {
                self.filter = match self.filter {
                    Filter::AllProjects => Filter::CurrentProject,
                    Filter::CurrentProject | Filter::Project(_) => Filter::AllProjects,
                };
                self.recompute_visible();
            }
            KeyCode::Char('p') => self.jump_to_parent(),
            _ => {}
        }
    }

    fn handle_viewing_key(&mut self, code: KeyCode) {
        match code {
            KeyCode::Char('q') | KeyCode::Esc | KeyCode::Enter => self.mode = Mode::Browsing,
            KeyCode::Up | KeyCode::Char('k') => self.scroll_by(-1),
            KeyCode::Down | KeyCode::Char('j') => self.scroll_by(1),
            KeyCode::PageUp => self.scroll_by(-15),
            KeyCode::PageDown => self.scroll_by(15),
            KeyCode::Home => self.scroll = 0,
            KeyCode::End => self.scroll = u16::MAX,
            _ => {}
        }
    }

    fn handle_project_jump_key(&mut self, code: KeyCode) {
        match code {
            KeyCode::Char('q') | KeyCode::Esc => self.mode = Mode::Browsing,
            KeyCode::Up | KeyCode::Char('k') => {
                if !self.project_summaries.is_empty() {
                    let len = self.project_summaries.len();
                    let cur = self.project_jump_state.selected().unwrap_or(0);
                    self.project_jump_state.select(Some((cur + len - 1) % len));
                }
            }
            KeyCode::Down | KeyCode::Char('j') => {
                if !self.project_summaries.is_empty() {
                    let len = self.project_summaries.len();
                    let cur = self.project_jump_state.selected().unwrap_or(0);
                    self.project_jump_state.select(Some((cur + 1) % len));
                }
            }
            KeyCode::Enter => {
                if let Some(idx) = self.project_jump_state.selected()
                    && let Some(summary) = self.project_summaries.get(idx)
                {
                    self.filter = Filter::Project(summary.project_dir.clone());
                    self.recompute_visible();
                    self.mode = Mode::Browsing;
                }
            }
            _ => {}
        }
    }
}

/// Scans off the render/event thread so a reload never blocks input or
/// drawing, sending the result back as an `IndexReady` event.
fn spawn_scan(claude_projects_root: PathBuf, tx: Sender<AppEvent>) {
    std::thread::spawn(move || {
        let runs = Run::index(&claude_projects_root);
        let display_paths = runs.iter().map(|r| own_cwd(&r.path)).collect();
        let _ = tx.send(AppEvent::IndexReady(runs, display_paths));
    });
}

/// Bounded to ~80 lines: every event line carries its own `cwd`, so this
/// finds one without reading a large transcript in full.
fn own_cwd(transcript_path: &Path) -> Option<PathBuf> {
    let file = std::fs::File::open(transcript_path).ok()?;
    let reader = std::io::BufReader::new(file);
    reader
        .lines()
        .take(80)
        .map_while(Result::ok)
        .find_map(|line| event::extract_cwd(&line).map(PathBuf::from))
}
