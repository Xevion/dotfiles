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
- `~/.config/opencode/AGENTS.md` — OpenCode global rules (symlinked to `~/.claude/CLAUDE.md`)

## `setup-ai-configs` script

The `setup-ai-configs` script (`~/.local/bin/setup-ai-configs`) creates symlinks for tools that need files in non-standard locations (e.g., `.cursorrules`, `.antigravity/rules.md`). It should point these to `CLAUDE.md`, not `AGENTS.md`.

## History

The original pattern was `AGENTS.md` as source of truth with `CLAUDE.md` containing `@AGENTS.md`. This was reversed in March 2026 after confirming that `CLAUDE.md` has sufficient cross-tool support and is required by the primary tool (Claude Code).
