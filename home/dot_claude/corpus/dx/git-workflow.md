---
name: git-workflow
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: scripts/pre-commit.ts
    note: Pre-commit hook with partial-staging detection and auto-format safety
---

# Git Workflow

## Philosophy

Conventional commits. `master` as default branch. Rebase-merge for clean history. Atomic commits — each commit is a self-contained logical change.

## Conventions

- **Scoped conventional commits**: `feat(backend):`, `fix(frontend):`, `refactor(scraper):` — scope maps to subsystem. Release-please (or similar) auto-generates changelogs from these prefixes
- **Pre-commit auto-format with staging safety**: pre-commit hooks that auto-format must detect partially-staged files and refuse to re-stage when both staged and unstaged changes exist. Staging isolation is non-negotiable
- **GPG commit signing**: sign commits with GPG keys. WSL environments bridge to Windows GPG for native pinentry
- **Short-lived branches**: `feature/*`, `fix/*` — merge quickly, delete after merge

## Anti-Patterns

- Merge commits for single-commit PRs (use rebase-merge)
- WIP commits left in history (squash or interactive rebase before merge)
- Force-pushing shared branches
- Pre-commit hooks that silently include unstaged changes during auto-formatting

## Open Questions

- Trunk-based development adoption for solo vs team projects
- SSH key signing vs GPG signing trade-offs
