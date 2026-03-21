# AI Config File Strategy

## Decision: `CLAUDE.md` is the canonical rules file

All project rules live in `CLAUDE.md`. When a tool only reads `AGENTS.md`, add a one-line pointer:

```markdown
@CLAUDE.md
```

This gives every tool access to the same rules from a single source of truth.

## Why not `AGENTS.md` as the canonical file?

`AGENTS.md` has broader *native* support, but `CLAUDE.md` has sufficient coverage for the tools actually in use — and Claude Code (the primary tool) cannot read `AGENTS.md` at all. There is no `@` import syntax in `AGENTS.md` to bridge the gap, so making `AGENTS.md` canonical forces `CLAUDE.md` to be the pointer. Since Claude Code is the primary tool, it makes more sense for `CLAUDE.md` to hold the content.

## Cross-tool compatibility (as of March 2026)

| Tool | `CLAUDE.md` | `AGENTS.md` | Notes |
|------|:-----------:|:-----------:|-------|
| Claude Code | native | -- | No AGENTS.md support ([FR #6235](https://github.com/anthropics/claude-code/issues/6235)) |
| OpenCode | fallback | native | Reads CLAUDE.md when no AGENTS.md exists |
| Cursor | opt-in toggle | native | "Include CLAUDE.md in context" setting; also imports `.claude/commands/` |
| GitHub Copilot (coding agent) | native | native | Both supported since Aug 2025 |
| Codex CLI | configurable | native | Add to `project_doc_fallback_filenames` in config.toml |
| Gemini CLI | configurable | -- | Uses `GEMINI.md` by default; add to `contextFileName` in settings.json |
| Windsurf | likely | native | Unconfirmed for CLAUDE.md |
| Aider | configurable | configurable | File-agnostic; requires `--read` flag or `.aider.conf.yml` |

## Per-project setup

**Standard project (Claude Code + OpenCode):**
```
project/
└── CLAUDE.md          # All rules here
```

OpenCode reads `CLAUDE.md` as fallback when no `AGENTS.md` exists. No pointer needed.

**Project needing AGENTS.md (Codex, Windsurf, other tools):**
```
project/
├── CLAUDE.md          # All rules here
└── AGENTS.md          # Contains only: @CLAUDE.md
```

## Global config

- `~/.claude/CLAUDE.md` — Claude Code global rules (chezmoi-managed via `home/dot_claude/CLAUDE.md.tmpl`)
- `~/.config/opencode/AGENTS.md` — OpenCode global rules (chezmoi-managed via `home/dot_config/opencode/AGENTS.md.tmpl`)
- Both files share rules by including `home/.chezmoitemplates/common-rules.md.tmpl` via Go template
- OpenCode's `AGENTS.md.tmpl` has its own preamble (Question tool character limits, multiple-select defaults) before the shared rules

## History

1. **Original**: `AGENTS.md` as source of truth, `CLAUDE.md` containing `@AGENTS.md`
2. **March 2026**: Reversed — `CLAUDE.md` became canonical since Claude Code (primary tool) can't read `AGENTS.md`
3. **March 2026 (later)**: Migrated to template-include model — shared rules in `common-rules.md.tmpl`, both `CLAUDE.md.tmpl` and `AGENTS.md.tmpl` include it with tool-specific preambles. Agents and skills consolidated under `home/dot_claude/` (previously split across `home/dot_config/opencode/agent/` and `home/dot_config/opencode/skills/`).
