---
name: cli-tool-design
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/tempo
    path: src/cli.ts
    note: cleye-based CLI with global flag pre-pass, prefix matching, automatic TTY/CI detection, partial-staging safety
  - repo: local/bose-re
    path: crates/bose-cli/src/main.rs
    note: "clap global = true for -v flag, three-tier log level override, OutputBuffer for async output"
---

# CLI Tool Design

## Philosophy

Progressive disclosure: simple cases need no flags, advanced cases are discoverable via `--help`. Sensible defaults mean the tool does the right thing without configuration. Output adapts to the environment automatically — humans get formatted output, scripts get plain or JSON. Errors are specific and actionable, not generic. Fail fast: validate arguments before doing work.

## Conventions

### Global flag pre-pass

Extract meta flags (`-v`/`--verbose`, `-q`/`--quiet`, `--log-file`, `--config`) from `process.argv` before subcommand parsing. Mutate or shadow argv to strip them so the downstream subcommand parser sees clean input. This ensures global flags work regardless of where they appear on the command line and avoids conflicts with subcommand-local flags of the same name.

```ts
const globalFlags = extractGlobalFlags(process.argv.slice(2));
// globalFlags.verbose, globalFlags.quiet, etc. are available globally
const remainingArgv = globalFlags._remaining; // passed to subcommand parser
```

### Prefix-matching enum resolution

For subcommand names and enum-valued flags, implement three-step resolution: exact match first, then unambiguous prefix match, then error listing ambiguous candidates. This lets users abbreviate naturally (`run fix-f` → resolves to `fix-first`) without requiring exact spelling.

```ts
function resolvePrefix(input: string, candidates: string[]): string {
  const exact = candidates.find(c => c === input);
  if (exact) return exact;
  const matches = candidates.filter(c => c.startsWith(input));
  if (matches.length === 1) return matches[0];
  if (matches.length > 1) throw new Error(`Ambiguous: "${input}" matches ${matches.join(", ")}`);
  throw new Error(`Unknown value: "${input}". Valid: ${candidates.join(", ")}`);
}
```

### Automatic TTY/CI output switching

Detect the output environment at startup and switch output modes without requiring manual flags. Check `process.stdout.isTTY` for interactive terminals, `process.env.CI` for generic CI, and known provider variables (`GITHUB_ACTIONS`, `GITLAB_CI`, `CIRCLECI`) for provider-specific formatting. In GitHub Actions, use `::group::` and `::endgroup::` for collapsible sections. In TTY, use spinners and ANSI color. In plain piped output, strip all decoration.

```ts
const env = detectEnv(); // { isTTY, isCI, provider: "github" | "gitlab" | null }
const logger = env.isCI && env.provider === "github"
  ? new GitHubActionsLogger()
  : env.isTTY
  ? new SpinnerLogger()
  : new PlainLogger();
```

## Language-Specific

### Rust

Use `clap` derive macros for argument parsing — the derive API keeps argument definitions colocated with the struct fields that receive them. Use `colored` or `owo-colors` for terminal output (prefer `owo-colors` for zero-allocation formatting). Use `indicatif` for progress bars and spinners. Use `anyhow` for error propagation in binaries; `thiserror` for library error types that callers might match on.

- **Global flags via clap `global = true`**: declare global flags (`--verbose`, `--quiet`, `--config`) with `global = true` on the top-level `Cli` struct. Clap handles positional extraction automatically — the idiomatic Rust equivalent of the TypeScript global flag pre-pass. Use `ArgAction::Count` for `-v`/-vv`/-vvv` verbosity patterns

### TypeScript

Use `cleye` or `Commander` for command/argument parsing. `cleye` has a minimal API and good TypeScript inference; Commander is more full-featured with a larger ecosystem. Use `ansis` for ANSI color (smaller than chalk, ESM-native). Use `LogTape` for structured logging when the CLI needs log levels, sinks, or contextual fields. Distribute via Bun's `bun build --compile` for a single self-contained binary.

### Go

Use `cobra` for command structure — it is the de facto standard and integrates well with `pflag` for POSIX-style flags. Use `lipgloss` for styled terminal output. Use `bubbletea` when the CLI needs interactive TUI elements (selection menus, forms, live-updating views). For simple spinners, `briandowns/spinner` is sufficient without pulling in a full TUI framework.

## Anti-Patterns

- **Interactive prompts without `--yes`** — any prompt that blocks automation must have a bypass flag. Scripts and CI pipelines cannot answer questions.
- **Unclear error messages** — "operation failed" is not an error message. Include what was attempted, what went wrong, and what the user can do next.
- **No `--quiet` for scripting** — if a command prints progress output by default, it must support `--quiet` or `-q` to suppress it. Noisy output breaks shell pipelines that capture stdout.
- **Color codes in non-TTY output** — ANSI escapes in piped output corrupt downstream consumers. Always check `isTTY` before emitting color.
- **Exit code 0 on failure** — any non-success outcome must exit non-zero. Scripts use exit codes for control flow; lying about success silently breaks them.

## Open Questions

- **TUI framework maturity** — bubbletea (Go), Ratatui (Rust), and Ink (TypeScript/React) are all viable but each has ergonomic gaps. Worth tracking as the space matures.
- **Cross-platform terminal compatibility** — Windows Terminal has improved significantly but edge cases remain with ANSI sequences, cursor movement, and Unicode rendering. Test on Windows CMD and PowerShell separately from Windows Terminal.
