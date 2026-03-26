---
name: project-automation
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: scripts/
    note: Bun-based build orchestration with command registry, staleness detection, and smart auto-fix
---

# Project Automation

## Philosophy

Build scripts are code — write them in a typed language, not shell. Centralize command definitions so every script agrees on how to invoke tools. Detect staleness automatically rather than requiring manual regeneration steps.

## Conventions

- **Command registry**: map `(subsystem, action)` tuples to command definitions in a single module. All scripts (check, format, lint, test, pre-commit) import from this registry — no duplicated command strings

```typescript
// Pattern: centralized command definitions
const REGISTRY: Record<Subsystem, Record<Action, CommandDef>> = {
  frontend: {
    'format-check': { cmd: ['bun', 'run', '--cwd', 'web', 'format:check'] },
    lint: { cmd: ['bun', 'run', '--cwd', 'web', 'lint'] },
  },
  backend: {
    'format-check': { cmd: ['cargo', 'fmt', '--all', '--', '--check'] },
    lint: { cmd: ['cargo', 'clippy', '--all-features', '--', '-D', 'warnings'] },
  },
};
```

- **Mtime-based staleness detection**: before running checks, compare source mtime vs generated artifact mtime. Auto-regenerate code-gen outputs (type bindings, query metadata) when sources are newer
- **Smart auto-fix**: when running parallel checks, auto-format only if formatting is the sole failure (all peer checks passed). If other checks also fail, report errors without reformatting to avoid masking issues
- **Pre-commit partial-staging safety**: pre-commit hooks that auto-format must detect partially-staged files and refuse to re-stage when both staged and unstaged changes exist in the same file

## Anti-Patterns

- Duplicating command strings across multiple scripts
- Requiring manual regeneration of generated artifacts
- Auto-formatting during check runs that also have lint/type errors
- Pre-commit hooks that silently include unstaged changes

## Open Questions

- Turborepo/Nx-style task caching for monorepo builds
- Watcher-based vs poll-based staleness detection trade-offs
