//! guard — a Claude Code hook program, plus its own internal re-entry point.
//!
//! Bare invocation (and its `guard hook` alias) reads one hook JSON payload
//! from stdin and dispatches by event and tool: `PreToolUse`/`Bash` runs
//! discipline rules, approval, and pipeline rewriting; `PostToolUse` on
//! `Write`/`Edit` runs post-edit rules. `guard run` is the internal re-entry
//! point a rewritten pipeline calls back into: it executes a single pipeline
//! itself, spawns stages, wires pipes, taps the unfiltered source stream, and
//! reports exit/duration/counts in a footer.
//!
//! See `DESIGN.md` for the full rationale.

pub mod approval;
pub mod check;
pub mod comment;
pub mod config;
pub mod dispatch;
pub mod git;
pub mod hook;
pub mod lang;
pub mod logging;
pub mod nested;
pub mod outcome;
pub mod parse;
pub mod payload;
pub mod post_edit;
pub mod rm_policy;
pub mod rules;
pub mod run;
pub mod tap;
