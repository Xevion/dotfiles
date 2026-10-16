mod render;
mod view;
mod wrap;

use std::cell::{Cell, RefCell};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime};

use crossbeam_channel::{Receiver, RecvTimeoutError, Sender};
use crossterm::event::{Event, KeyCode, KeyEventKind};
use ratatui::style::Stylize as _;
use ratatui::text::{Line, Text};
use ratatui::widgets::ListState;
use snafu::ResultExt as _;

use crate::error::{AppError, DrawSnafu, WatchSnafu};
use crate::index::{Indexer, Run};
use crate::paths::ShortenExt as _;
use crate::transcript::Transcript;
use crate::watch::Watch;
use wrap::WrapIndex;

/// Redraw cadence while idle, so relative timestamps and run states keep
/// moving without any filesystem event.
const TICK: Duration = Duration::from_secs(1);

/// A live run's transcript is reparsed in full on every reload, so
/// reloading the open one is rate limited.
const RELOAD_INTERVAL: Duration = Duration::from_secs(2);

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

#[derive(Clone, Copy, PartialEq, Eq)]
pub enum PreviewKind {
    /// What the run handed back.
    Report,
    /// The whole conversation.
    Transcript,
}

impl PreviewKind {
    fn toggled(self) -> Self {
        match self {
            Self::Report => Self::Transcript,
            Self::Transcript => Self::Report,
        }
    }

    fn label(self) -> &'static str {
        match self {
            Self::Report => "report",
            Self::Transcript => "transcript",
        }
    }
}

pub struct ProjectSummary {
    pub project_dir: PathBuf,
    pub display_path: String,
    pub count: usize,
    pub most_recent: SystemTime,
}

/// A transcript parsed and styled for display. `mtime` is the run's mtime
/// when it was read, so a changed file can be told from a current preview.
pub struct Preview {
    pub agent_id: String,
    pub mtime: SystemTime,
    pub title: String,
    pub report: Text<'static>,
    pub transcript: Text<'static>,
}

/// Scans never overlap: one asked for mid-scan runs right after it.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
enum ScanState {
    Idle,
    Running,
    RunningThenRescan,
}

struct PreviewRequest {
    agent_id: String,
    mtime: SystemTime,
    path: PathBuf,
    title: String,
}

struct WrapCache {
    revision: u64,
    width: u16,
    index: WrapIndex,
}

pub struct App {
    pub home: PathBuf,
    pub cwd: PathBuf,
    pub generic_roots: Vec<PathBuf>,

    pub all_runs: Vec<Run>,
    /// Runs of a refresh in progress. The list keeps showing `all_runs`
    /// until the refresh completes, so rows never vanish and reappear.
    staging: Vec<Run>,
    pub filter: Filter,
    pub visible: Vec<usize>,
    pub list_state: ListState,
    /// First list row on screen; set while drawing.
    list_offset: Cell<usize>,
    /// Until the cursor first moves it stays on the newest run, however
    /// many more arrive. After, it follows its run by id.
    cursor_moved: bool,

    pub mode: Mode,
    pub project_summaries: Vec<ProjectSummary>,
    pub project_jump_state: ListState,

    pub preview: Option<Preview>,
    pub preview_kind: PreviewKind,
    /// The (run, mtime) a preview was last asked for, loaded or in flight.
    requested: Option<(String, SystemTime)>,
    last_request: Instant,
    /// Bumped whenever the displayed text changes, invalidating `wrap_cache`.
    preview_revision: u64,
    wrap_cache: RefCell<Option<WrapCache>>,
    pub scroll: usize,
    max_scroll: Cell<usize>,

    pub should_quit: bool,
    /// True until the first scan completes.
    pub loading: bool,
    scan: ScanState,
    scan_generation: u64,

    claude_projects_root: PathBuf,
    indexer: Arc<Indexer>,
    events: Sender<AppEvent>,
    preview_requests: Sender<PreviewRequest>,
}

enum AppEvent {
    Input(Event),
    Refresh,
    /// One session's runs, tagged so a superseded scan's stragglers are dropped.
    ScanBatch {
        generation: u64,
        runs: Vec<Run>,
    },
    ScanDone {
        generation: u64,
    },
    PreviewReady(Preview),
}

impl App {
    fn new(
        home: PathBuf,
        cwd: PathBuf,
        all: bool,
        claude_projects_root: PathBuf,
        events: Sender<AppEvent>,
        preview_requests: Sender<PreviewRequest>,
    ) -> Self {
        let generic_roots = Run::generic_roots(&home);
        Self {
            home,
            cwd,
            generic_roots,
            all_runs: Vec::new(),
            staging: Vec::new(),
            filter: if all {
                Filter::AllProjects
            } else {
                Filter::CurrentProject
            },
            visible: Vec::new(),
            list_state: ListState::default(),
            list_offset: Cell::new(0),
            cursor_moved: false,
            mode: Mode::Browsing,
            project_summaries: Vec::new(),
            project_jump_state: ListState::default(),
            preview: None,
            preview_kind: PreviewKind::Report,
            requested: None,
            last_request: Instant::now(),
            preview_revision: 0,
            wrap_cache: RefCell::new(None),
            scroll: 0,
            max_scroll: Cell::new(0),
            should_quit: false,
            loading: true,
            scan: ScanState::Idle,
            scan_generation: 0,
            claude_projects_root,
            indexer: Arc::new(Indexer::default()),
            events,
            preview_requests,
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

        let (preview_tx, preview_rx) = crossbeam_channel::unbounded();
        spawn_preview_worker(preview_rx, tx.clone());

        let mut app = Self::new(
            home,
            cwd,
            all,
            claude_projects_root.to_path_buf(),
            tx,
            preview_tx,
        );
        app.start_scan();

        let mut terminal = ratatui::init();
        let outcome = (|| -> Result<(), AppError> {
            loop {
                terminal
                    .draw(|frame| app.render(frame))
                    .context(DrawSnafu)?;
                if app.should_quit {
                    return Ok(());
                }
                match rx.recv_timeout(TICK) {
                    Ok(event) => {
                        app.handle_event(event);
                        // Fold in everything already queued so a burst of
                        // scan batches costs one redraw, not one each.
                        while let Ok(event) = rx.try_recv() {
                            app.handle_event(event);
                        }
                    }
                    Err(RecvTimeoutError::Timeout) => app.sync_preview(),
                    Err(RecvTimeoutError::Disconnected) => return Ok(()),
                }
            }
        })();
        ratatui::restore();
        outcome
    }

    fn handle_event(&mut self, event: AppEvent) {
        match event {
            AppEvent::Input(Event::Key(key)) if key.kind == KeyEventKind::Press => {
                self.handle_key(key.code);
            }
            AppEvent::Input(_) => {}
            AppEvent::Refresh => self.request_scan(),
            AppEvent::ScanBatch { generation, runs } => self.apply_batch(generation, runs),
            AppEvent::ScanDone { generation } => self.finish_scan(generation),
            AppEvent::PreviewReady(preview) => self.apply_preview(preview),
        }
    }

    fn request_scan(&mut self) {
        match self.scan {
            ScanState::Idle => self.start_scan(),
            ScanState::Running | ScanState::RunningThenRescan => {
                self.scan = ScanState::RunningThenRescan;
            }
        }
    }

    fn start_scan(&mut self) {
        self.scan_generation += 1;
        self.scan = ScanState::Running;
        self.staging.clear();

        let generation = self.scan_generation;
        let indexer = Arc::clone(&self.indexer);
        let root = self.claude_projects_root.clone();
        let events = self.events.clone();
        std::thread::spawn(move || {
            indexer.scan(&root, |runs| {
                let _ = events.send(AppEvent::ScanBatch { generation, runs });
            });
            let _ = events.send(AppEvent::ScanDone { generation });
        });
    }

    fn apply_batch(&mut self, generation: u64, runs: Vec<Run>) {
        if generation != self.scan_generation {
            return;
        }
        if self.loading {
            // First scan: show runs as they arrive.
            let keep = self.cursor_anchor();
            self.all_runs.extend(runs);
            self.all_runs.sort_by_key(|r| std::cmp::Reverse(r.mtime));
            self.recompute_visible(keep);
        } else {
            self.staging.extend(runs);
        }
    }

    fn finish_scan(&mut self, generation: u64) {
        if generation != self.scan_generation {
            return;
        }
        let keep = self.cursor_anchor();
        if !self.loading {
            self.all_runs = std::mem::take(&mut self.staging);
            self.all_runs.sort_by_key(|r| std::cmp::Reverse(r.mtime));
        }
        self.loading = false;
        let rescan = self.scan == ScanState::RunningThenRescan;
        self.scan = ScanState::Idle;
        self.recompute_visible(keep);
        if rescan {
            self.start_scan();
        }
    }

    /// The run the cursor should stay on across a list change, if the user
    /// has taken control of it.
    fn cursor_anchor(&self) -> Option<String> {
        if self.cursor_moved {
            self.selected_run_agent_id().map(str::to_string)
        } else {
            None
        }
    }

    fn recompute_visible(&mut self, keep: Option<String>) {
        let previous_index = self.list_state.selected().unwrap_or(0);
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

        let kept = keep.and_then(|id| {
            self.visible
                .iter()
                .position(|&i| self.all_runs[i].agent_id == id)
        });
        let selected = kept.unwrap_or(if self.cursor_moved {
            previous_index.min(self.visible.len().saturating_sub(1))
        } else {
            0
        });
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
        let run = &self.all_runs[index];
        run.own_cwd
            .as_deref()
            .or_else(|| run.session_cwds.first().map(PathBuf::as_path))
            .map_or_else(
                || run.project_dir.display().to_string(),
                |p| p.shorten(&self.home),
            )
    }

    /// Asks the worker for the selected run's transcript when the preview
    /// is missing or out of date.
    fn sync_preview(&mut self) {
        let Some(run) = self.selected_run() else {
            self.clear_preview();
            return;
        };
        let wanted = (run.agent_id.clone(), run.mtime);
        if self.requested.as_ref() == Some(&wanted) {
            return;
        }

        let same_run = self
            .requested
            .as_ref()
            .is_some_and(|(id, _)| *id == wanted.0);
        if same_run && self.last_request.elapsed() < RELOAD_INTERVAL {
            return;
        }

        let request = PreviewRequest {
            agent_id: wanted.0.clone(),
            mtime: wanted.1,
            path: run.path.clone(),
            title: format!("{}: {}", run.meta.agent_type, run.meta.description),
        };
        if !same_run {
            // Another run's text must not sit under this one's title.
            self.preview = None;
            self.scroll = 0;
            self.bump_revision();
        }
        self.requested = Some(wanted);
        self.last_request = Instant::now();
        let _ = self.preview_requests.send(request);
    }

    fn clear_preview(&mut self) {
        self.preview = None;
        self.requested = None;
        self.scroll = 0;
        self.bump_revision();
    }

    fn apply_preview(&mut self, preview: Preview) {
        // A result for the selected run is shown even if the file has
        // changed again since it was requested; insisting on the latest
        // mtime would starve a busy run's preview forever. Only a result
        // older than what is already displayed is dropped.
        let selected = self
            .requested
            .as_ref()
            .is_some_and(|(id, _)| *id == preview.agent_id);
        let newer = self
            .preview
            .as_ref()
            .is_none_or(|p| p.agent_id != preview.agent_id || p.mtime <= preview.mtime);
        if !(selected && newer) {
            return;
        }
        self.preview = Some(preview);
        self.bump_revision();
    }

    fn bump_revision(&mut self) {
        self.preview_revision += 1;
    }

    fn move_cursor(&mut self, delta: isize) {
        if self.visible.is_empty() {
            return;
        }
        self.cursor_moved = true;
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

    fn scroll_by(&mut self, delta: isize) {
        self.scroll = self
            .scroll
            .saturating_add_signed(delta)
            .min(self.max_scroll.get());
    }

    fn toggle_preview_kind(&mut self) {
        self.preview_kind = self.preview_kind.toggled();
        self.scroll = 0;
        self.bump_revision();
    }

    fn jump_to_parent(&mut self) {
        let Some(run) = self.selected_run() else {
            return;
        };
        let Some(parent_id) = run.meta.parent_agent_id.clone() else {
            return;
        };
        self.cursor_moved = true;

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
            self.recompute_visible(Some(parent_id));
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
            KeyCode::Char('t') => self.toggle_preview_kind(),
            KeyCode::Char('a') => self.open_project_jump(),
            KeyCode::Char('A') => {
                let keep = self.cursor_anchor();
                self.filter = match self.filter {
                    Filter::AllProjects => Filter::CurrentProject,
                    Filter::CurrentProject | Filter::Project(_) => Filter::AllProjects,
                };
                self.recompute_visible(keep);
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
            KeyCode::End => self.scroll = self.max_scroll.get(),
            KeyCode::Char('t') => self.toggle_preview_kind(),
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
                    self.recompute_visible(None);
                    self.mode = Mode::Browsing;
                }
            }
            _ => {}
        }
    }
}

/// Parses and styles transcripts off the UI thread, newest request wins:
/// moving the cursor quickly skips straight to where it landed instead of
/// parsing every run it passed.
fn spawn_preview_worker(requests: Receiver<PreviewRequest>, events: Sender<AppEvent>) {
    std::thread::spawn(move || {
        while let Ok(mut request) = requests.recv() {
            while let Ok(newer) = requests.try_recv() {
                request = newer;
            }
            let (report, transcript) = match Transcript::load(&request.path) {
                Ok(transcript) => (
                    render::report_text(&transcript.report()),
                    render::transcript_text(&transcript),
                ),
                Err(e) => {
                    let notice =
                        Text::from(Line::from(format!("(could not read transcript: {e})")).red());
                    (notice.clone(), notice)
                }
            };
            let preview = Preview {
                agent_id: request.agent_id,
                mtime: request.mtime,
                title: request.title,
                report,
                transcript,
            };
            if events.send(AppEvent::PreviewReady(preview)).is_err() {
                break;
            }
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::event::Closure;
    use crate::index::AgentMeta;
    use assert2::check;

    fn run(id: &str, age_secs: u64) -> Run {
        Run {
            agent_id: id.to_string(),
            path: PathBuf::from(format!("/nonexistent/agent-{id}.jsonl")),
            project_dir: PathBuf::from("/projects/p"),
            meta: AgentMeta::default(),
            mtime: SystemTime::now() - Duration::from_secs(age_secs),
            closure: Closure::Text,
            own_cwd: Some(PathBuf::from("/home/u/proj")),
            session_cwds: Arc::new(vec![PathBuf::from("/home/u/proj")]),
        }
    }

    fn app() -> App {
        let (events, _) = crossbeam_channel::unbounded();
        let (previews, _) = crossbeam_channel::unbounded();
        let mut app = App::new(
            PathBuf::from("/home/u"),
            PathBuf::from("/home/u/proj"),
            true,
            PathBuf::from("/nonexistent"),
            events,
            previews,
        );
        app.scan_generation = 1;
        app.scan = ScanState::Running;
        app
    }

    fn ids(app: &App) -> Vec<&str> {
        app.visible
            .iter()
            .map(|&i| app.all_runs[i].agent_id.as_str())
            .collect()
    }

    #[test]
    fn first_scan_shows_runs_newest_first_as_they_arrive() {
        let mut app = app();
        app.apply_batch(1, vec![run("old", 100)]);
        check!(ids(&app) == ["old"]);
        app.apply_batch(1, vec![run("new", 1)]);
        check!(ids(&app) == ["new", "old"]);
        check!(app.loading);
    }

    #[test]
    fn untouched_cursor_stays_on_the_newest_run() {
        let mut app = app();
        app.apply_batch(1, vec![run("old", 100)]);
        app.apply_batch(1, vec![run("new", 1)]);
        check!(app.selected_run_agent_id() == Some("new"));
    }

    #[test]
    fn moved_cursor_follows_its_run_as_newer_ones_arrive() {
        let mut app = app();
        app.apply_batch(1, vec![run("b", 50), run("c", 100)]);
        app.move_cursor(1);
        check!(app.selected_run_agent_id() == Some("c"));

        app.apply_batch(1, vec![run("a", 1)]);
        check!(ids(&app) == ["a", "b", "c"]);
        check!(app.selected_run_agent_id() == Some("c"));
    }

    #[test]
    fn stale_generation_batches_are_dropped() {
        let mut app = app();
        app.apply_batch(0, vec![run("ghost", 1)]);
        check!(app.all_runs.is_empty());
        app.finish_scan(0);
        check!(app.loading);
    }

    #[test]
    fn refresh_swaps_in_only_when_complete() {
        let mut app = app();
        app.apply_batch(1, vec![run("a", 10)]);
        app.finish_scan(1);
        check!(!app.loading);

        app.scan_generation = 2;
        app.scan = ScanState::Running;
        app.apply_batch(2, vec![run("a", 5), run("b", 1)]);
        check!(ids(&app) == ["a"]);

        app.finish_scan(2);
        check!(ids(&app) == ["b", "a"]);
        check!(app.scan == ScanState::Idle);
    }

    #[test]
    fn refresh_drops_runs_that_disappeared() {
        let mut app = app();
        app.apply_batch(1, vec![run("a", 10), run("gone", 20)]);
        app.finish_scan(1);

        app.scan_generation = 2;
        app.apply_batch(2, vec![run("a", 5)]);
        app.finish_scan(2);
        check!(ids(&app) == ["a"]);
    }

    #[test]
    fn scan_requested_mid_scan_is_queued_not_overlapped() {
        let mut app = app();
        app.request_scan();
        check!(app.scan == ScanState::RunningThenRescan);
        check!(app.scan_generation == 1);
    }

    #[test]
    fn queued_scan_starts_when_the_running_one_finishes() {
        let mut app = app();
        app.request_scan();
        app.finish_scan(1);
        check!(app.scan_generation == 2);
        check!(app.scan == ScanState::Running);
    }

    #[test]
    fn scrolling_is_clamped_to_the_content() {
        let mut app = app();
        app.max_scroll.set(5);
        app.scroll_by(100);
        check!(app.scroll == 5);
        app.scroll_by(-100);
        check!(app.scroll == 0);
    }

    #[test]
    fn toggling_the_preview_kind_resets_scroll() {
        let mut app = app();
        app.scroll = 7;
        app.toggle_preview_kind();
        check!(app.preview_kind == PreviewKind::Transcript);
        check!(app.scroll == 0);
    }

    fn preview_of(id: &str, mtime: SystemTime) -> Preview {
        Preview {
            agent_id: id.to_string(),
            mtime,
            title: String::new(),
            report: Text::default(),
            transcript: Text::default(),
        }
    }

    #[test]
    fn preview_older_than_the_requested_mtime_is_still_shown_for_the_selected_run() {
        let mut app = app();
        let t1 = SystemTime::UNIX_EPOCH + Duration::from_secs(1);
        let t2 = SystemTime::UNIX_EPOCH + Duration::from_secs(2);
        app.requested = Some(("a".to_string(), t2));
        app.apply_preview(preview_of("a", t1));
        check!(app.preview.as_ref().map(|p| p.mtime) == Some(t1));
    }

    #[test]
    fn preview_older_than_the_displayed_one_is_dropped() {
        let mut app = app();
        let t1 = SystemTime::UNIX_EPOCH + Duration::from_secs(1);
        let t2 = SystemTime::UNIX_EPOCH + Duration::from_secs(2);
        app.requested = Some(("a".to_string(), t2));
        app.apply_preview(preview_of("a", t2));
        app.apply_preview(preview_of("a", t1));
        check!(app.preview.as_ref().map(|p| p.mtime) == Some(t2));
    }

    #[test]
    fn preview_for_a_run_no_longer_selected_is_ignored() {
        let mut app = app();
        app.requested = Some(("wanted".to_string(), SystemTime::UNIX_EPOCH));
        app.apply_preview(Preview {
            agent_id: "other".to_string(),
            mtime: SystemTime::UNIX_EPOCH,
            title: String::new(),
            report: Text::default(),
            transcript: Text::default(),
        });
        check!(app.preview.is_none());
    }
}
