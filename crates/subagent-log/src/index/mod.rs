pub mod matching;
pub mod state;

use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::SystemTime;

use crossbeam_channel::Receiver;
use rayon::prelude::*;
use serde::Deserialize;

use crate::event;
use state::RunState;

#[derive(Debug, Deserialize, Default, Clone)]
pub struct AgentMeta {
    #[serde(rename = "agentType", default)]
    pub agent_type: String,
    #[serde(default)]
    pub description: String,
    #[serde(rename = "parentAgentId", default)]
    pub parent_agent_id: Option<String>,
    #[serde(rename = "spawnDepth", default)]
    pub spawn_depth: u32,
    #[serde(rename = "isFork", default)]
    pub is_fork: bool,
}

impl AgentMeta {
    fn read(path: &Path) -> Option<Self> {
        let raw = fs::read_to_string(path).ok()?;
        serde_json::from_str(&raw).ok()
    }
}

pub struct Run {
    pub agent_id: String,
    pub path: PathBuf,
    pub project_dir: PathBuf,
    pub meta: AgentMeta,
    pub mtime: SystemTime,
    pub state: RunState,
    /// Shared across every run in the same session. `Arc`, not `Rc`: built
    /// on rayon worker threads and sent across a channel.
    pub session_cwds: Arc<Vec<PathBuf>>,
}

impl Run {
    #[must_use]
    pub fn matches_cwd(&self, cwd: &Path, generic_roots: &[PathBuf]) -> bool {
        Self::session_matches(&self.session_cwds, cwd, generic_roots)
    }

    /// Blocks until every session is scanned. Thin wrapper over
    /// [`Self::index_streaming`], restoring mtime-descending order.
    #[must_use]
    pub fn index(claude_projects_root: &Path) -> Vec<Self> {
        let mut runs: Vec<Self> = Self::index_streaming(claude_projects_root)
            .into_iter()
            .flatten()
            .collect();
        runs.sort_by_key(|r| std::cmp::Reverse(r.mtime));
        runs
    }

    /// Pushes one `Vec<Run>` batch per session as it finishes scanning,
    /// rather than blocking on the whole corpus. Batches are not
    /// mtime-sorted; iterate or poll `try_recv` until the channel closes.
    #[must_use]
    pub fn index_streaming(claude_projects_root: &Path) -> Receiver<Vec<Self>> {
        let sessions = SessionEntry::discover(claude_projects_root);
        let (tx, rx) = crossbeam_channel::unbounded();
        let now = SystemTime::now();

        rayon::spawn(move || {
            sessions.into_par_iter().for_each(|session| {
                let runs = Self::scan_session(&session, now);
                if !runs.is_empty() {
                    let _ = tx.send(runs);
                }
            });
        });

        rx
    }

    fn scan_session(session: &SessionEntry, now: SystemTime) -> Vec<Self> {
        let main_scan = session.main_transcript.as_deref().map(Scan::of);
        let scans: Vec<Scan> = session
            .agent_paths
            .par_iter()
            .map(|p| Scan::of(p))
            .collect();

        let mut session_cwds = Vec::new();
        if let Some(s) = &main_scan {
            session_cwds.extend(s.cwds.iter().cloned());
        }
        for s in &scans {
            session_cwds.extend(s.cwds.iter().cloned());
        }
        session_cwds.sort();
        session_cwds.dedup();
        let session_cwds = Arc::new(session_cwds);

        session
            .agent_paths
            .iter()
            .zip(&scans)
            .filter_map(|(path, s)| {
                let agent_id = Self::agent_id_from_path(path)?;
                let meta_path = session
                    .subagents_dir
                    .join(format!("agent-{agent_id}.meta.json"));
                let meta = AgentMeta::read(&meta_path).unwrap_or_default();
                let mtime = fs::metadata(path)
                    .and_then(|m| m.modified())
                    .unwrap_or(SystemTime::UNIX_EPOCH);
                let state = RunState::classify(s.last_turn_closed, mtime, now);
                Some(Self {
                    agent_id,
                    path: path.clone(),
                    project_dir: session.project_dir.clone(),
                    meta,
                    mtime,
                    state,
                    session_cwds: Arc::clone(&session_cwds),
                })
            })
            .collect()
    }

    fn agent_id_from_path(path: &Path) -> Option<String> {
        let name = path.file_stem()?.to_str()?;
        name.strip_prefix("agent-").map(str::to_string)
    }
}

struct SessionEntry {
    project_dir: PathBuf,
    subagents_dir: PathBuf,
    main_transcript: Option<PathBuf>,
    agent_paths: Vec<PathBuf>,
}

impl SessionEntry {
    fn discover(claude_projects_root: &Path) -> Vec<Self> {
        let mut sessions = Vec::new();
        let Ok(project_dirs) = fs::read_dir(claude_projects_root) else {
            return sessions;
        };

        for project_entry in project_dirs.flatten() {
            if !is_dir(&project_entry) {
                continue;
            }
            let project_dir = project_entry.path();
            let Ok(session_dirs) = fs::read_dir(&project_dir) else {
                continue;
            };

            for session_entry in session_dirs.flatten() {
                if !is_dir(&session_entry) {
                    continue;
                }
                let session_dir = session_entry.path();
                let subagents_dir = session_dir.join("subagents");
                if !subagents_dir.is_dir() {
                    continue;
                }
                let session_id = session_entry.file_name().to_string_lossy().into_owned();
                let agent_paths = Self::agent_transcript_paths(&subagents_dir);
                let main_transcript = project_dir.join(format!("{session_id}.jsonl"));
                let main_transcript = main_transcript.is_file().then_some(main_transcript);

                sessions.push(Self {
                    project_dir: project_dir.clone(),
                    subagents_dir,
                    main_transcript,
                    agent_paths,
                });
            }
        }

        sessions
    }

    fn agent_transcript_paths(subagents_dir: &Path) -> Vec<PathBuf> {
        let Ok(entries) = fs::read_dir(subagents_dir) else {
            return Vec::new();
        };
        entries
            .flatten()
            .map(|e| e.path())
            .filter(|p| {
                p.file_name()
                    .and_then(|n| n.to_str())
                    .is_some_and(|n| n.starts_with("agent-"))
                    && p.extension()
                        .is_some_and(|ext| ext.eq_ignore_ascii_case("jsonl"))
            })
            .collect()
    }
}

fn is_dir(entry: &fs::DirEntry) -> bool {
    entry.file_type().is_ok_and(|t| t.is_dir())
}

struct Scan {
    cwds: Vec<PathBuf>,
    last_turn_closed: bool,
}

impl Scan {
    fn of(path: &Path) -> Self {
        let mut cwds = Vec::new();
        let mut last_turn_closed = false;

        let Ok(raw) = fs::read_to_string(path) else {
            return Self {
                cwds,
                last_turn_closed,
            };
        };

        for line in raw.lines() {
            if line.is_empty() {
                continue;
            }
            if let Some(cwd) = event::extract_cwd(line) {
                cwds.push(PathBuf::from(cwd));
            }
            if let Some(message) = event::parse_assistant(line) {
                last_turn_closed = message.closed_with_text();
            }
        }

        Self {
            cwds,
            last_turn_closed,
        }
    }
}
