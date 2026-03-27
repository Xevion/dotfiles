---
name: git-workflow
category: dx
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/banner
    path: scripts/pre-commit.ts
    note: Pre-commit hook with partial-staging detection and auto-format safety
  - repo: local/maestro
    path: scripts/pre-commit.ts
    note: Pre-commit hook with partial-staging detection, installed via just install-hooks
---

# Git Workflow

## Philosophy

Conventional commits. `master` as default branch. Rebase-merge for clean history. Atomic commits — each commit is a self-contained logical change.

## Conventions

- **Scoped conventional commits**: `feat(backend):`, `fix(frontend):`, `refactor(scraper):` — scope maps to subsystem. Release-please (or similar) auto-generates changelogs from these prefixes
- **Pre-commit auto-format with staging safety**: pre-commit hooks that auto-format must handle partially-staged files safely (see [project-automation](../dx/project-automation.md) for the detection pattern)
- **GPG commit signing**: sign commits with GPG keys. WSL environments bridge to Windows GPG for native pinentry
- **`just install-hooks` convention**: symlinks pre-commit scripts and makes them executable. Reference in CLAUDE.md to tell AI agents the hook handles formatting so agents don't duplicate that work
- **Commitlint + Husky for enforcement**: commitlint extending `@commitlint/config-conventional` via a Husky `commit-msg` hook. Pre-commit runs `lint-staged` for format/lint on staged files. Note: `lint-staged` handles partial-staging safety automatically — the custom TypeScript pre-commit script with explicit partial-staging detection is needed only when calling formatters directly without `lint-staged`
- **Short-lived branches**: `feature/*`, `fix/*` — merge quickly, delete after merge

## Anti-Patterns

- Merge commits for single-commit PRs (use rebase-merge)
- WIP commits left in history (squash or interactive rebase before merge)
- Force-pushing shared branches
- Pre-commit hooks that silently include unstaged changes during auto-formatting (see [project-automation](../dx/project-automation.md))

## Open Questions

- Trunk-based development adoption for solo vs team projects
- SSH key signing vs GPG signing trade-offs
