---
name: cli-tool-design
category: dx
last_audited: 2026-03-26
exemplars: []
---

# CLI Tool Design

## Philosophy

<!-- Progressive disclosure, sensible defaults, scriptable output, fail fast with clear messages -->

## Conventions

<!-- Subcommand structure, --help everywhere, JSON output option, exit codes with meaning, color in TTY only -->

## Language-Specific

### Rust
<!-- clap derive macros, colored/owo-colors for output, indicatif for progress -->

### TypeScript
<!-- Commander/yargs, ora for spinners, chalk for colors, Bun for distribution -->

### Go
<!-- cobra for commands, lipgloss for TUI styling, bubbletea for interactive -->

## Anti-Patterns

<!-- Interactive prompts without --yes flag, unclear error messages, no --quiet option for scripting -->

## Open Questions

<!-- TUI framework maturity, cross-platform terminal compatibility -->
