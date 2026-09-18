use clap::{Parser, Subcommand};

#[derive(Parser)]
#[command(about = "Browse Claude Code subagent transcripts", version)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Option<Command>,

    /// Show every project instead of just the current one
    #[arg(long, global = true)]
    pub all: bool,
}

#[derive(Subcommand)]
pub enum Command {
    /// Plain-text listing, newest first (non-interactive)
    List,
    /// Print the most recent run's report to stdout
    Latest,
    /// Look up a run by agent-id prefix, or description substring
    Show { query: String },
}
