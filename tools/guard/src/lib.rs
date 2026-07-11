//! guard — transparent pipeline capture for Claude Code's Bash tool.
//!
//! Two subcommands, one binary. `guard hook` is the PreToolUse logic
//! (discipline rules, approval, pipeline rewriting). `guard run` executes a
//! single pipeline itself: spawns stages, wires pipes, taps the unfiltered
//! source stream, and reports exit/duration/counts in a footer.
//!
//! See `DESIGN.md` for the full rationale.

pub mod approval;
pub mod hook;
pub mod nested;
pub mod parse;
pub mod rules;
pub mod run;
pub mod tap;
