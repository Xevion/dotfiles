---
name: ci-cd-deployment
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: .github/workflows/
    note: 6-job parallel CI, cargo-chef Docker builds, release-please gated on CI
---

# CI/CD & Deployment

## Philosophy

GitHub Actions as default CI. Docker for reproducible deploys. Fail fast — independent parallel jobs with short timeouts. Release automation gated on CI success.

## Conventions

- **Parallel independent jobs**: split quality, tests, security, and docker-build into separate jobs that run simultaneously. Each job has a short timeout (10min)
- **cargo-chef for Rust Docker builds**: separate dependency-only cook step from source compilation for layer cache reuse. Final image based on a minimal runtime (e.g. `bun-slim`), not the build toolchain

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

- **Release automation gated on CI**: trigger release-please (or equivalent) via `workflow_run` only after CI completes successfully. Use language-native release types (e.g. `rust` type bumps `Cargo.toml` and `Cargo.lock`)
- **Frozen lockfiles in CI**: use `--frozen-lockfile` / `--locked` flags to ensure CI reproduces exactly what was committed

## Anti-Patterns

- Hardcoded secrets in CI configs (use GitHub Secrets / Doppler)
- Deploy-on-merge without passing checks
- Monolithic single-job pipelines (slow feedback)
- Skipping security scanning (cargo-audit, bun audit, Trivy)

## Open Questions

- Self-hosted runners economics and maintenance burden
- Fly.io vs Railway vs Vercel decision criteria per project type
