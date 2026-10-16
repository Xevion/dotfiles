pub mod matching;
pub mod scan;
pub mod state;

use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, PoisonError};
use std::time::SystemTime;

use rayon::prelude::*;
use serde::Deserialize;

use crate::event::Closure;
use scan::{FileScan, ScanCache, Track};
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
    pub closure: Closure,
    /// The run's own first recorded cwd. Per-run, not per-session: the
    /// session-wide list can point at a stale project path after a rename.
    pub own_cwd: Option<PathBuf>,
    /// Shared across every run in the same session. `Arc`, not `Rc`: built
    /// on rayon worker threads and sent across a channel.
    pub session_cwds: Arc<Vec<PathBuf>>,
}

impl Run {
    #[must_use]
    pub fn matches_cwd(&self, cwd: &Path, generic_roots: &[PathBuf]) -> bool {
        Self::session_matches(&self.session_cwds, cwd, generic_roots)
    }

    /// Derived on read, not stored: a run settles from active to done as
    /// time passes, with nothing on disk changing.
    #[must_use]
    pub fn state(&self, now: SystemTime) -> RunState {
        RunState::classify(self.closure, self.mtime, now)
    }
}

/// Scans transcripts under `~/.claude/projects`, remembering what it has
/// read so repeated scans only touch what changed.
#[derive(Default)]
pub struct Indexer {
    cache: ScanCache,
    /// Written once when an agent is spawned, so read once.
    metas: Mutex<HashMap<PathBuf, AgentMeta>>,
}

impl Indexer {
    /// Calls `on_batch` with one session's runs as soon as that session is
    /// scanned, newest sessions first, and returns once everything has been.
    /// Batches arrive from worker threads, in no guaranteed order.
    pub fn scan(&self, claude_projects_root: &Path, on_batch: impl Fn(Vec<Run>) + Sync) {
        // `par_bridge` pulls items in iteration order, so the newest sessions
        // start first and the earliest batches are the most relevant ones.
        SessionEntry::discover(claude_projects_root)
            .into_iter()
            .par_bridge()
            .for_each(|session| {
                let runs = self.scan_session(&session);
                if !runs.is_empty() {
                    on_batch(runs);
                }
            });
    }

    /// Blocks until every session is scanned; newest run first.
    #[must_use]
    pub fn collect(&self, claude_projects_root: &Path) -> Vec<Run> {
        let runs = Mutex::new(Vec::new());
        self.scan(claude_projects_root, |batch| {
            runs.lock()
                .unwrap_or_else(PoisonError::into_inner)
                .extend(batch);
        });
        let mut runs = runs.into_inner().unwrap_or_else(PoisonError::into_inner);
        runs.sort_by_key(|r| std::cmp::Reverse(r.mtime));
        runs
    }

    /// Unreadable or half-written files give defaults and are retried.
    fn meta(&self, path: &Path) -> AgentMeta {
        let lock = || self.metas.lock().unwrap_or_else(PoisonError::into_inner);
        if let Some(meta) = lock().get(path) {
            return meta.clone();
        }
        let Some(meta) = AgentMeta::read(path) else {
            return AgentMeta::default();
        };
        lock().insert(path.to_path_buf(), meta.clone());
        meta
    }

    fn scan_session(&self, session: &SessionEntry) -> Vec<Run> {
        // The main transcript is often the largest file, so it runs
        // alongside the agent transcripts rather than ahead of them.
        let (main, scans): (Option<Arc<FileScan>>, Vec<Arc<FileScan>>) = rayon::join(
            || {
                session
                    .main_transcript
                    .as_deref()
                    .map(|p| self.cache.scan(p, Track::CwdsOnly))
            },
            || {
                session
                    .agents
                    .par_iter()
                    .map(|agent| self.cache.scan(&agent.path, Track::CwdsAndTurns))
                    .collect()
            },
        );

        let mut session_cwds: Vec<PathBuf> = main
            .iter()
            .chain(&scans)
            .flat_map(|s| s.cwds.iter().cloned())
            .collect();
        session_cwds.sort();
        session_cwds.dedup();
        let session_cwds = Arc::new(session_cwds);

        session
            .agents
            .iter()
            .zip(&scans)
            .map(|(agent, scan)| {
                let meta_path = session
                    .subagents_dir
                    .join(format!("agent-{}.meta.json", agent.agent_id));
                Run {
                    agent_id: agent.agent_id.clone(),
                    path: agent.path.clone(),
                    project_dir: session.project_dir.clone(),
                    meta: self.meta(&meta_path),
                    mtime: agent.mtime,
                    closure: scan.closure,
                    own_cwd: scan.cwds.first().cloned(),
                    session_cwds: Arc::clone(&session_cwds),
                }
            })
            .collect()
    }
}

/// The most recently written subagent transcript, found without reading any
/// transcript contents.
#[must_use]
pub fn newest_transcript(claude_projects_root: &Path) -> Option<PathBuf> {
    SessionEntry::discover(claude_projects_root)
        .into_iter()
        .flat_map(|s| s.agents)
        .max_by_key(|a| a.mtime)
        .map(|a| a.path)
}

struct AgentFile {
    path: PathBuf,
    agent_id: String,
    mtime: SystemTime,
}

struct SessionEntry {
    project_dir: PathBuf,
    subagents_dir: PathBuf,
    main_transcript: Option<PathBuf>,
    agents: Vec<AgentFile>,
}

impl SessionEntry {
    /// Directory listing and `stat` only, newest session first.
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
                let subagents_dir = session_entry.path().join("subagents");
                let agents = Self::agent_files(&subagents_dir);
                if agents.is_empty() {
                    continue;
                }
                let session_id = session_entry.file_name().to_string_lossy().into_owned();
                let main_transcript = project_dir.join(format!("{session_id}.jsonl"));
                let main_transcript = main_transcript.is_file().then_some(main_transcript);

                sessions.push(Self {
                    project_dir: project_dir.clone(),
                    subagents_dir,
                    main_transcript,
                    agents,
                });
            }
        }

        sessions.sort_by_key(|s| std::cmp::Reverse(s.agents.iter().map(|a| a.mtime).max()));
        sessions
    }

    fn agent_files(subagents_dir: &Path) -> Vec<AgentFile> {
        let Ok(entries) = fs::read_dir(subagents_dir) else {
            return Vec::new();
        };
        entries
            .flatten()
            .filter_map(|entry| {
                let path = entry.path();
                let agent_id = path
                    .file_name()?
                    .to_str()?
                    .strip_prefix("agent-")?
                    .strip_suffix(".jsonl")?
                    .to_string();
                let mtime = entry
                    .metadata()
                    .and_then(|m| m.modified())
                    .unwrap_or(SystemTime::UNIX_EPOCH);
                Some(AgentFile {
                    path,
                    agent_id,
                    mtime,
                })
            })
            .collect()
    }
}

fn is_dir(entry: &fs::DirEntry) -> bool {
    entry.file_type().is_ok_and(|t| t.is_dir())
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;
    use std::io::Write as _;

    struct Corpus {
        root: tempfile::TempDir,
    }

    impl Corpus {
        fn new() -> Self {
            Self {
                root: tempfile::tempdir().unwrap(),
            }
        }

        fn agent(&self, project: &str, session: &str, agent: &str, lines: &[&str]) -> PathBuf {
            let subagents = self
                .root
                .path()
                .join(project)
                .join(session)
                .join("subagents");
            fs::create_dir_all(&subagents).unwrap();
            let path = subagents.join(format!("agent-{agent}.jsonl"));
            let mut f = fs::File::create(&path).unwrap();
            for line in lines {
                writeln!(f, "{line}").unwrap();
            }
            fs::write(
                subagents.join(format!("agent-{agent}.meta.json")),
                r#"{"agentType":"general-purpose","description":"a task"}"#,
            )
            .unwrap();
            path
        }
    }

    const PROMPT: &str = r#"{"type":"user","cwd":"/work/a","message":{"content":"go"}}"#;
    const HANDBACK: &str = r#"{"type":"assistant","message":{"content":[{"type":"tool_use","id":"h","name":"SubagentHandback","input":{"message":"r"}}]}}"#;
    const WORKING: &str = r#"{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t","name":"Bash","input":{}}]}}"#;

    #[test]
    fn collect_finds_runs_with_meta_and_closure() {
        let corpus = Corpus::new();
        corpus.agent("proj", "s1", "aaa", &[PROMPT, HANDBACK]);
        corpus.agent("proj", "s1", "bbb", &[PROMPT, WORKING]);

        let runs = Indexer::default().collect(corpus.root.path());
        check!(runs.len() == 2);

        let done = runs.iter().find(|r| r.agent_id == "aaa").unwrap();
        check!(done.closure == Closure::HandedBack);
        check!(done.meta.description == "a task");
        check!(done.own_cwd == Some(PathBuf::from("/work/a")));

        let open = runs.iter().find(|r| r.agent_id == "bbb").unwrap();
        check!(open.closure == Closure::Open);
    }

    #[test]
    fn scan_streams_one_batch_per_session() {
        let corpus = Corpus::new();
        corpus.agent("proj", "s1", "aaa", &[PROMPT]);
        corpus.agent("proj", "s2", "bbb", &[PROMPT]);
        corpus.agent("other", "s3", "ccc", &[PROMPT]);

        let batches = Mutex::new(Vec::new());
        Indexer::default().scan(corpus.root.path(), |batch| {
            batches.lock().unwrap().push(batch.len());
        });
        check!(batches.into_inner().unwrap() == [1, 1, 1]);
    }

    #[test]
    fn newest_transcript_ignores_contents() {
        let corpus = Corpus::new();
        let older = corpus.agent("proj", "s1", "old", &[PROMPT]);
        std::thread::sleep(std::time::Duration::from_millis(20));
        let newer = corpus.agent("proj", "s2", "new", &[PROMPT]);

        check!(newest_transcript(corpus.root.path()) == Some(newer));
        check!(older.exists());
    }

    #[test]
    fn sessions_without_subagents_are_skipped() {
        let corpus = Corpus::new();
        fs::create_dir_all(corpus.root.path().join("proj").join("s1")).unwrap();
        check!(Indexer::default().collect(corpus.root.path()).is_empty());
    }
}
