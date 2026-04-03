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

### Dependabot Alternative

For projects where Renovate's advanced features (allowedVersions with rationale, minimumReleaseAge) are not needed, GitHub Dependabot is a simpler alternative. Minimum config for a Rust project covers `cargo` + `github-actions` ecosystems. Use `commit-message: { prefix: "chore", include: "scope" }` to produce conventional commits. Dependabot lacks Renovate's grouping and release-age guards — use Renovate when those guards are needed. Note: even with Dependabot, GitHub Actions should reference pinned SHA digests — Dependabot's `github-actions` ecosystem will maintain SHA pins once added.

### Baseline Renovate config for multi-language repos

For multi-language monorepos (Go + TypeScript, Rust + TypeScript), four baseline conventions: (1) ecosystem grouping by manager and tightly-coupled namespaces, (2) `minimumReleaseAge` of 3 days, (3) `:semanticCommits` for conventional commit consistency, (4) `helpers:pinGitHubActionDigests` for supply-chain safety. A bare `{ "dependencyDashboard": true }` config is insufficient.

### Runtime & Package Manager Pinning

Layer enforcement to prevent wrong-runtime installs. Each layer catches a different failure mode:

1. **`packageManager` field** in `package.json`: declares the exact package manager + version (e.g., `"bun@1.3.9"`). Corepack uses this. Pin to specific patch versions for reproducibility
2. **`preinstall` script**: hard-blocks the wrong package manager at install time. Two variants:
   - `"npx only-allow bun"` — simple, widely used
   - Inline `npm_execpath` check — no external dependency, detects non-Bun executors at runtime
3. **`engines` field**: declares minimum runtime version (`"bun": ">=1.0.0"`, `"node": ">=22.0.0"`). Informational unless paired with `engine-strict`
4. **`.npmrc` with `engine-strict=true`**: makes `engines` constraints enforced rather than advisory

All four layers should be present in Bun-first projects. For pnpm projects, substitute `npx only-allow pnpm` and appropriate engine constraints.

### Transitive Dependency Security Overrides

Use `package.json` `overrides` (npm/bun) or `resolutions` (yarn/pnpm) to pin transitive dependencies to patched versions when the direct dependency hasn't released a fix yet. Include a comment or Renovate `description` explaining which CVE or advisory the override addresses. Remove overrides once the direct dependency updates.

### cargo-deny for Rust Dependency Auditing

Use `deny.toml` for license compliance, advisory scanning, and source restrictions. Baseline configuration:
- `graph.all-features = true` — audit with all features enabled to catch conditional deps
- `licenses.confidence-threshold = 0.8` — reasonable SPDX detection threshold
- `advisories.yanked = "deny"` — fail on yanked crates
- `sources.unknown-registry = "deny"`, `sources.unknown-git = "deny"` — restrict to crates.io
- `bans.multiple-versions = "warn"` — flag duplicate transitive versions without hard-failing

### CI/Local Tool Version Synchronization

Pin CI-sensitive tools (especially Go linters like `golangci-lint`) to exact versions in both `mise.toml` and CI config. The failure mode: CI runs `latest` but local mise resolved `latest` a week ago — CI fails on new lint rules that don't reproduce locally. Use `"latest"` for primary runtimes (Rust, Bun) where version drift is benign, but exact pins for tools whose output changes with every release. When CI pins a version, local mise must match.

## Anti-Patterns

- Pinning to exact versions without automated updates
- Ignoring audit warnings
- Adding deps for trivial functions
- Blocking major version upgrades without documenting the reason
- **Bare Renovate config (`{ "dependencyDashboard": true }`)**: produces noisy per-package PRs without release-age guards, ecosystem grouping, or SHA-pinned GitHub Actions. A bare config is worse than no Renovate because it creates false confidence in dependency management. Use the baseline config documented above

## Open Questions

- Renovate vs Dependabot feature parity and selection criteria
