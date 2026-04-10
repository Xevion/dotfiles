---
name: dependency-management
category: dx
last_audited: 2026-04-10
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
  - repo: local/Applyhelm
    path: renovate.json
    note: "Full baseline config + patch-update automerge, 8 ecosystem groups (SvelteKit, WXT, UnoCSS, Bits UI, Biome, ESLint, LogTape, Rust, Docker, Actions)"
  - repo: local/toriix
    path: renovate.json + deny.toml
    note: "Git-HEAD dependency handling: Renovate opt-out with rationale for gpui/gpui_platform, deny.toml unknown-git=allow with explicit allowlist, transitive advisory ignore with upstream-fix-pending comments"
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

### Patch-Update Automerge

Enable `automerge: true` with `automergeType: "pr"` on patch-level updates to reduce PR review burden. Patch releases are semver-promised to be non-breaking; after CI passes, there's little value in a human rubber-stamp. Exclude security-sensitive packages (auth, crypto, native binaries) from the automerge rule via a separate `packageRules` entry. Pair with `minimumReleaseAge` so a yanked patch doesn't auto-merge before the yank is detected.

### Git-HEAD Dependency Handling

When a project depends on an upstream crate at a git rev rather than a released version (e.g., tracking `zed-industries/zed` HEAD for an actively-developed dependency), Renovate cannot manage updates. Two complementary guards:

1. **Explicit Renovate opt-out with rationale**: add a `packageRules` entry matching the affected package names, set `enabled: false`, and put the reason in a `description` field. Prevents Renovate from generating spurious "no update found" noise and documents the exception
2. **`cargo-deny` with `unknown-git = "allow"` + explicit allowlist**: in `deny.toml`, set `sources.unknown-git = "allow"` (the default is `deny`), then list permitted git sources in `allow-git`. This keeps the default-deny posture for everything else while letting the specific git dependency through
3. **Advisory ignore list with upstream-fix-pending comments**: when transitive advisories from the git-pinned dep can't be resolved without upstream fixes, add them to `advisories.ignore` with a comment citing the upstream issue. Revisit on each audit — the goal is to keep the list short and stale entries marked

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
