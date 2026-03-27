---
name: dependency-management
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: local/inkwell
    path: renovate.json
    note: "Ecosystem grouping by manager and namespace, dependencyDashboard: false"
  - repo: Xevion/Pac-Man
    path: .github/renovate.json
    note: "allowedVersions blocks with inline rationale, minimumReleaseAge, pinGitHubActionDigests"
---

# Dependency Management

## Philosophy

Lock files always committed. Conservative updates. Audit discipline. Minimal dependency count — don't add deps for trivial functions.

## Conventions

- **Renovate ecosystem grouping**: group by manager (gomod) and by tightly-coupled package namespaces (`@sveltejs/`, `@pandacss/`, `@biomejs/`) to reduce PR noise while keeping related updates atomic. For multi-language monorepos this is essential
- **`allowedVersions` blocks with inline rationale**: when a major version upgrade requires migration work, block it explicitly in Renovate config with a `description` field explaining the blocker. This is a first-class pattern, not a workaround
- **`minimumReleaseAge`**: set a minimum release age (e.g., 3 days) to avoid rushing newly-published packages that may be yanked
- **`helpers:pinGitHubActionDigests`**: pin GitHub Actions to full SHA digests for supply-chain security. Include via Renovate `extends`
- **`:semanticCommits`**: enable semantic commit messages on Renovate PRs for conventional-commit consistency

## Anti-Patterns

- Pinning to exact versions without automated updates
- Ignoring audit warnings
- Adding deps for trivial functions
- Blocking major version upgrades without documenting the reason

## Open Questions

- Renovate vs Dependabot feature parity and selection criteria
