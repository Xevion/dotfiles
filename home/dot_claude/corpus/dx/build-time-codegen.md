---
name: build-time-codegen
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/Pac-Man
    path: pacman/build.rs
    note: "JSON texture atlas → PHF static map as Rust source via build.rs"
  - repo: local/inkwell
    path: sqlc.yml + tygo.yaml
    note: "sqlc (SQL→Go) and tygo (Go→TS) dual codegen with CI verification"
  - repo: Xevion/xevion.dev
    path: .sqlx/ + Cargo.toml
    note: "sqlx offline query caching, ts-rs for Rust→TypeScript bindings"
---

# Build-Time Code Generation

## Philosophy

<!-- Treat codegen as a build step. Generated files are build artifacts, not source code. Verify staleness in CI. -->

## Conventions

<!-- What to generate, when to regenerate, what to commit, CI verification via regen+diff -->

## Language-Specific

### Rust

<!-- build.rs for compile-time codegen (PHF maps, embedded data), sqlx offline mode (.sqlx/ directory) -->

### Go

<!-- sqlc for SQL→Go, tygo for Go→TypeScript, codegen outputs committed and CI-verified -->

### TypeScript

<!-- ts-rs generated bindings, barrel index auto-generation -->

## Anti-Patterns

<!-- Manually editing generated files, stale generated files not caught in CI, generating into source directories without .gitignore separation -->

## Open Questions

<!-- When to commit generated files vs regenerate in CI, cross-language codegen orchestration -->
