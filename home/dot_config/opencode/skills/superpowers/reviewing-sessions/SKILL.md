---
name: reviewing-sessions
description: "Use when asked to review, analyze, or query OpenCode session history — troubleshooting tool failures, identifying repeated instructions, analyzing token costs, or finding workflow improvement opportunities across sessions."
---

# Reviewing OpenCode Sessions

## Overview

Query OpenCode's SQLite database to analyze session history for workflow improvements. Use this to find problematic tools, repeated patterns, cost outliers, and subagent effectiveness.

## Database Location & Access

**DB path**: `~/.local/share/opencode/opencode.db` (SQLite, WAL mode)

**Two access methods:**

| Method | Command | Best for |
|--------|---------|----------|
| **OpenCode CLI** | `opencode db "<SQL>" --format json` | Simple queries, handles locking, portable |
| **Direct SQLite** | `sqlite3 "$HOME/.local/share/opencode/opencode.db" "<SQL>"` | Complex queries, when opencode binary unavailable |

**Output formats** for `opencode db`: `--format json` (structured, pipe to `jq`), `--format tsv` (tabular, human-readable).

**Timestamps** are Unix milliseconds. Convert: `datetime(time_created/1000, 'unixepoch', 'localtime')`.

**JSON extraction**: Message and part `data` columns are JSON. Use `json_extract(data, '$.field')`.

## Schema Quick Reference

```
project (id, worktree, name)
  └─ session (id, project_id, parent_id, slug, title, directory, version,
              summary_additions, summary_deletions, summary_files,
              time_created, time_updated)
       ├─ message (id, session_id, data JSON)
       │    │  data: { role, modelID, providerID, agent, mode,
       │    │          tokens: {input, output}, cost, time: {created, completed} }
       │    └─ part (id, message_id, session_id, data JSON)
       │         data: { type, tool, state: {status, input, output}, content }
       │         types: text, tool, reasoning, step-start, step-finish,
       │                patch, file, compaction, agent, subtask
       └─ todo (session_id, content, status, priority, position)
```

**Part types for tool calls**: `json_extract(data,'$.type') = 'tool'`. Status values: `completed`, `error`, `running`, `pending`.

**Permission signals** are stored in `json_extract(data, '$.state.error')` on error-status tool parts:

| Error pattern | Meaning |
|---------------|---------|
| `Tool execution aborted` | User cancelled a running tool (Ctrl+C or UI abort) |
| `The user rejected permission%` | User explicitly denied the permission prompt |
| `The user has specified a rule which prevents%` | Automatic rule-based denial (permission config) |
| `The user dismissed this question` | User dismissed a `question` tool prompt |

## Analysis Cookbook

See [queries.md](queries.md) for the full query reference organized by goal:

1. **Session browsing** — list, search, filter by project/date/title
2. **Tool analysis** — usage frequency, error rates, failure patterns
3. **Token & cost analysis** — per-session, per-model, cost outliers
4. **Session lineage** — fork chains, subagent depth, parent tracing
5. **Workflow patterns** — compaction events, todo completion, version history
6. **Permissions** — user rejections, rule-blocked calls, aborted executions

## Workflow

When asked to analyze sessions:

1. **Scope the question** — global (all projects) or specific project directory?
2. **Pick the right query** from the cookbook, or compose from the schema
3. **Use `--format json`** and pipe through `jq` for structured output
4. **Cross-reference** — tool errors often correlate with high token usage; check both
5. **Summarize findings** with actionable suggestions (config changes, skill gaps, tool issues)

## Tips

- **Filter by project**: `WHERE directory = '/path/to/project'` or `WHERE directory LIKE '%project-name%'`
- **Recent sessions**: `WHERE time_created > strftime('%s','now','-7 days') * 1000`
- **Truncated tool names** (e.g., `bas_`, `edi_`, `gre_`, `webfetc`, `websearc`) appear from cancelled/interrupted calls or compacted sessions. To exclude them: `AND length(json_extract(data,'$.tool')) > 3 AND json_extract(data,'$.tool') NOT LIKE '%\_'` (escape the underscore). Or group them with their full-name counterparts for accurate totals
- **Tool name prefixes**: MCP tools use `server_toolname` format. Known prefixes: `cclsp_` (Claude Code LSP bridge), `lsp_` (native OpenCode LSP), `ast_grep_` (AST structural search), `context7_`, `linear_`, `gh_grep_`, `grep_app_`. Filter with `LIKE 'prefix%'`
- **`opencode export <session_id>`** dumps a full session as structured JSON for deep inspection
- **File diffs**: `~/.local/share/opencode/storage/session_diff/<session_id>.json` has before/after file content
