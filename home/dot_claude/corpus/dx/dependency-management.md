---
name: dependency-management
category: dx
last_audited: 2026-04-03
exemplars:
  - repo: local/inkwell
    path: renovate.json
    note: "Ecosystem grouping by manager and namespace, dependencyDashboard: false"
  - repo: Xevion/Pac-Man
    path: .github/renovate.json
    note: "allowedVersions blocks with inline rationale, minimumReleaseAge, pinGitHubActionDigests"
  - repo: Xevion/rustdoc-mcp
    path: deny.toml
    note: "cargo-deny with 4 target triples, license allowlist, yanked=deny, unknown-registry=deny"
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

### Dependabot Alternative

For projects where Renovate's advanced features (allowedVersions with rationale, minimumReleaseAge) are not needed, GitHub Dependabot is a simpler alternative. Minimum config for a Rust project covers `cargo` + `github-actions` ecosystems. Use `commit-message: { prefix: "chore", include: "scope" }` to produce conventional commits. Dependabot lacks Renovate's grouping and release-age guards — use Renovate when those guards are needed. Note: even with Dependabot, GitHub Actions should reference pinned SHA digests — Dependabot's `github-actions` ecosystem will maintain SHA pins once added.

### Baseline Renovate config for multi-language repos

For multi-language monorepos (Go + TypeScript, Rust + TypeScript), four baseline conventions: (1) ecosystem grouping by manager and tightly-coupled namespaces, (2) `minimumReleaseAge` of 3 days, (3) `:semanticCommits` for conventional commit consistency, (4) `helpers:pinGitHubActionDigests` for supply-chain safety. A bare `{ "dependencyDashboard": true }` config is insufficient.

## Open Questions

- Renovate vs Dependabot feature parity and selection criteria
