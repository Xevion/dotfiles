mod cli;
mod error;
mod event;
mod index;
mod paths;
mod report;
mod time_fmt;
mod tui;
mod watch;

use std::io::IsTerminal as _;
use std::path::{Path, PathBuf};
use std::process::ExitCode;
use std::time::SystemTime;

use clap::Parser as _;

use cli::{Cli, Command};
use index::Run;
use paths::ShortenExt as _;
use report::Report;
use time_fmt::TimestampExt as _;

fn main() -> ExitCode {
    let cli = Cli::parse();

    let Some(home) = std::env::var_os("HOME").map(PathBuf::from) else {
        eprintln!("subagent-log: could not determine home directory ($HOME unset)");
        return ExitCode::FAILURE;
    };
    let claude_projects_root = home.join(".claude").join("projects");
    let cwd = std::env::current_dir().unwrap_or_else(|_| home.clone());

    match cli.command {
        // Not a real terminal: fall back to plain output instead of a TUI
        // that would just panic on init.
        None if !std::io::stdout().is_terminal() => {
            cmd_list(&claude_projects_root, &home, &cwd, cli.all);
            ExitCode::SUCCESS
        }
        None => match tui::App::run(&claude_projects_root, home, cwd, cli.all) {
            Ok(()) => ExitCode::SUCCESS,
            Err(e) => {
                eprintln!("subagent-log: {e}");
                ExitCode::FAILURE
            }
        },
        Some(Command::List) => {
            cmd_list(&claude_projects_root, &home, &cwd, cli.all);
            ExitCode::SUCCESS
        }
        Some(Command::Latest) => cmd_latest(&claude_projects_root),
        Some(Command::Show { query }) => {
            cmd_show(&claude_projects_root, &home, &cwd, cli.all, &query)
        }
    }
}

fn cmd_list(claude_projects_root: &Path, home: &Path, cwd: &Path, all: bool) {
    let runs = current_runs(claude_projects_root, home, cwd, all);
    let now = SystemTime::now();
    for run in &runs {
        println!("{}", format_line(run, home, now));
    }
}

fn cmd_latest(claude_projects_root: &Path) -> ExitCode {
    let runs = Run::index(claude_projects_root);
    let Some(run) = runs.first() else {
        eprintln!(
            "subagent-log: no subagent runs found under {}",
            claude_projects_root.display()
        );
        return ExitCode::FAILURE;
    };
    print_report(run);
    ExitCode::SUCCESS
}

fn cmd_show(
    claude_projects_root: &Path,
    home: &Path,
    cwd: &Path,
    all: bool,
    query: &str,
) -> ExitCode {
    let runs = current_runs(claude_projects_root, home, cwd, all);
    let found = runs
        .iter()
        .find(|r| r.agent_id.starts_with(query))
        .or_else(|| {
            runs.iter().find(|r| {
                r.meta
                    .description
                    .to_lowercase()
                    .contains(&query.to_lowercase())
            })
        });
    let Some(run) = found else {
        eprintln!("subagent-log: no run matching '{query}'");
        return ExitCode::FAILURE;
    };
    print_report(run);
    ExitCode::SUCCESS
}

fn print_report(run: &Run) {
    match Report::extract(&run.path) {
        Report::Clean(text) | Report::SendMessage(text) => println!("{text}"),
        Report::Inconclusive { last_seen } => {
            println!("(inconclusive: this transcript did not close normally)");
            println!("{last_seen}");
        }
    }
}

fn current_runs(claude_projects_root: &Path, home: &Path, cwd: &Path, all: bool) -> Vec<Run> {
    let runs = Run::index(claude_projects_root);
    if all {
        return runs;
    }
    let generic_roots = Run::generic_roots(home);
    runs.into_iter()
        .filter(|r| r.matches_cwd(cwd, &generic_roots))
        .collect()
}

fn format_line(run: &Run, home: &Path, now: SystemTime) -> String {
    let when = run.mtime.relative(now);
    let path = run.session_cwds.first().map_or_else(
        || run.project_dir.display().to_string(),
        |p| p.shorten(home),
    );
    format!(
        "{when:<9} {:<7} {agent_id}  {path}  {agent_type}: {description}",
        format!("{:?}", run.state),
        agent_id = run.agent_id,
        agent_type = run.meta.agent_type,
        description = run.meta.description,
    )
}
