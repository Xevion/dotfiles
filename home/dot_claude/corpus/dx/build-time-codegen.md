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
  - repo: Xevion/railway-collector
    path: internal/railway/generate.go + genqlient.yaml
    note: "genqlient with scalar bindings, gomock from interfaces.go"
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

- **genqlient for type-safe GraphQL client**: `go:generate go run github.com/Khan/genqlient` directive. Scalar bindings in `genqlient.yaml` map API types to Go types (`DateTime` → `string`, `BigInt` → `int64`, `JSON` → `any`). Generated file is committed; regenerate after schema or query edits
- **gomock for interface-driven mocks**: `go:generate mockgen -source=interfaces.go -destination=mocks/mocks.go`. Define interfaces in a dedicated file, commit output to `mocks/`, expose regeneration as a Justfile recipe (`just mocks`). Mock staleness should be verified in CI like other codegen
- sqlc for SQL→Go, tygo for Go→TypeScript, codegen outputs committed and CI-verified

### TypeScript

<!-- ts-rs generated bindings, barrel index auto-generation -->

## Anti-Patterns

<!-- Manually editing generated files, stale generated files not caught in CI, generating into source directories without .gitignore separation -->

## Open Questions

<!-- When to commit generated files vs regenerate in CI, cross-language codegen orchestration -->
