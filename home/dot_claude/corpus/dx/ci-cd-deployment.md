---
name: ci-cd-deployment
category: dx
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/banner
    path: .github/workflows/ + Dockerfile
    note: "6-job parallel CI, cargo-chef Docker builds, release-please gated on CI, binary executed in the runtime stage as a build gate"
  - repo: Xevion/doujin-ocr-summary
    path: .github/workflows/ci.yml + Dockerfile
    note: 5-job parallel CI with codegen verification, multi-stage Go Docker build
  - repo: local/inkwell
    path: .github/workflows/ci.yml
    note: "Dual codegen verification (sqlc + tygo) in same job, frozen-lockfile frontend, GHA cache for Docker"
  - repo: Xevion/ferrite
    path: .github/workflows/
    note: "workflow_run gating for release-please, Dependabot for cargo + github-actions"
  - repo: Xevion/xevion.dev
    path: .github/workflows/ci.yml
    note: "5-job parallel CI with binding-verification, security job (cargo-audit + zizmor + Trivy SARIF)"
  - repo: local/borde.rs
    path: .github/workflows (quality cross-lint job, determinism workflow)
    note: "Cross-lint matrix (cargo-xwin for Windows, cargo-zigbuild for macOS), costly-runner determinism goldens on a weekly cron and before releases"
  - repo: local/currents-render
    path: CI test job
    note: "GPU tests on lavapipe with WGPU_BACKEND=vulkan forced; toolchain channel read from rust-toolchain.toml"
  - repo: local/toriix
    path: .github/workflows/ci.yml
    note: "Vendor patch-apply-in-every-job pattern for submodule-with-in-tree-patches dependency"
---

# CI/CD & Deployment

## Philosophy

GitHub Actions as default CI. Docker for reproducible deploys. Fail fast — independent parallel jobs with short timeouts. Release automation gated on CI success.

## Conventions

- **Parallel independent jobs**: split quality, tests, security, and docker-build into separate jobs that run simultaneously. Each job must declare an explicit `timeout-minutes` (e.g., 10) to prevent hung jobs from consuming CI minutes indefinitely — especially important for jobs hitting external networks
- **cargo-chef for Rust Docker builds**: separate dependency-only cook step from source compilation for layer cache reuse. Final image based on a minimal runtime matching the actual entrypoint (e.g. `bun-slim` for Bun-based TypeScript orchestration, `alpine` for static binaries), not the build toolchain

```dockerfile
# Pattern: cargo-chef multi-stage
FROM cargo-chef AS planner
RUN cargo chef prepare --recipe-path recipe.json
FROM cargo-chef AS builder
COPY --from=planner recipe.json .
RUN cargo chef cook --release --recipe-path recipe.json  # cached layer
COPY . .
RUN cargo build --release
```

- **Release automation gated on CI**: trigger release-please (or equivalent) via `workflow_run` only after CI completes successfully. Use language-native release types (e.g. `rust` type bumps `Cargo.toml` and `Cargo.lock`). For single-branch repos, `workflow_run` gating with `if: conclusion == 'success'` is simpler than branch-protection merge queues and doesn't require GitHub Advanced Security
- **Frozen lockfiles in CI**: use `--frozen-lockfile` / `--locked` flags to ensure CI reproduces exactly what was committed
- **Multi-language parallel job split**: one job per subsystem (Go, Svelte, Python) running in parallel, with a gated docker job depending on all quality jobs passing
- **Generated artifact verification in CI**: regenerate and diff (`sqlc diff`, `tygo generate && git diff --exit-code`). Catches stale generated code in PRs before merge
- **Go Docker builds with `go mod download` as cached layer**: analogous to cargo-chef. Use UPX for binary compression. Match runtime base image to the actual entrypoint runtime (e.g. Bun, not Alpine) when the final binary is not standalone
- **`zizmor` workflow security scanning**: run `zizmorcore/zizmor-action` in a dedicated job with `security-events: write` permission for SARIF upload. Catches misuse of untrusted input (`pull_request_target` without sanitization), excessive permissions (`permissions: write-all`), missing SHA pins in workflow files themselves, and shell injection via `${{ }}` interpolation into `run:` blocks. The check is cheap (~30s on typical workflows) and catches a class of supply-chain errors that cargo-deny and npm-audit don't see. Pair with Dependabot's `github-actions` ecosystem to keep the action versions themselves pinned
- **Vendor patch-apply in every job** (for dependencies maintained as submodules with in-tree patches): when a project carries patches against a git submodule dependency (e.g., `vendor/gpui-component/` with `patches/*.patch`), every CI job that compiles must apply the patches *before* the compile step. Patches do not survive `actions/checkout` — each job needs its own `cd vendor/submodule && git apply ../../patches/*.patch` step. Extract to a composite action (`.github/actions/apply-vendor-patches/action.yml`) to avoid copy-paste drift when the dependency setup evolves

- **Gate the build on executing the artifact in the final runtime stage.** Running the binary in the runtime image resolves every symbol at build time, so an ABI or library mismatch fails the build. Pair it with an atomic rollout (`helm --atomic`) to catch startup failures the gate cannot see
- **One local gate that mirrors CI.** A single `check` recipe covers fmt, actionlint, zizmor, clippy, cross-target clippy for crates without native deps, a content-hash-cached clang-tidy, hermetic docs and a README that must match the crate docs. A floating `ubuntu-latest` canary leg with `continue-on-error` catches runner drift early, and a `repro-ci` recipe reproduces a CI failure in a stock distro container. Heavy sanitizer, coverage and mutation recipes stay advisory, outside `check`
- **Put costly runners on a schedule.** Windows and macOS runners bill at multiples of Linux, so run only the checks that need them (cross-platform determinism goldens, headless Chrome) on a weekly cron and before releases
- **Read the toolchain from `rust-toolchain.toml`** instead of an env var that CI ignores, so CI installs onto the pin
- **Gate secrets through a repo variable**, since secrets cannot gate a job-level `if`
- Advisory scanning on pull requests should judge a change on what it introduces; see [dependency-management](./dependency-management.md)

## Anti-Patterns

- Hardcoded secrets in CI configs (use GitHub Secrets / Doppler)
- Deploy-on-merge without passing checks
- Monolithic single-job pipelines (slow feedback)
- Skipping security scanning (cargo-audit, bun audit, Trivy)
- **GitHub Actions referenced by version tag without SHA pinning**: `actions/checkout@v6` is mutable — the tag can be moved. `aquasecurity/trivy-action@master` is even worse (branch ref). Pin all Actions to full SHA digests. Use `helpers:pinGitHubActionDigests` in Renovate or let Dependabot's `github-actions` ecosystem maintain SHA pins
- **Same-workflow deploy gating**: for static-site/WASM deployments (Cloudflare Pages, Netlify), job-level `needs:` + `if: github.event_name == 'push'` is simpler than `workflow_run` gating when build and deploy are in the same workflow file. Reserve `workflow_run` for cross-workflow coordination

## Open Questions

- Self-hosted runners economics and maintenance burden
- Fly.io vs Railway vs Vercel decision criteria per project type
