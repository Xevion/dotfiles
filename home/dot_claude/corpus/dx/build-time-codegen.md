---
name: build-time-codegen
category: dx
last_audited: 2026-04-03
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
  - repo: local/maestro
    path: codegen/src/ + common/src/config/
    note: "KSP + KotlinPoet annotation processor for config snapshot codegen"
---

# Build-Time Code Generation

## Philosophy

Treat codegen as a build step. Generated files are build artifacts, not source code. Verify staleness in CI via regen+diff.

## Conventions

- **Commit generated output**: generated files are committed to the repo (not .gitignored) so consumers can use the package without running codegen. CI verifies staleness via `regenerate && git diff --exit-code`
- **Justfile recipe for regeneration**: expose codegen as a `just` recipe (e.g., `just generate`, `just mocks`) so the command is discoverable and consistent

## Language-Specific

### Rust

- **build.rs for compile-time codegen**: generate static data structures (PHF maps from JSON atlases, embedded asset tables) at compile time. Output to `$OUT_DIR` and `include!` in source
- **sqlx offline mode**: `cargo sqlx prepare` generates `.sqlx/` query metadata for offline compilation. Commit the `.sqlx/` directory; `SQLX_OFFLINE=true` in Docker builds avoids requiring a live database

### Go

- **genqlient for type-safe GraphQL client**: `go:generate go run github.com/Khan/genqlient` directive. Scalar bindings in `genqlient.yaml` map API types to Go types (`DateTime` → `string`, `BigInt` → `int64`, `JSON` → `any`). Generated file is committed; regenerate after schema or query edits
- **gomock for interface-driven mocks**: `go:generate mockgen -source=interfaces.go -destination=mocks/mocks.go`. Define interfaces in a dedicated file, commit output to `mocks/`, expose regeneration as a Justfile recipe (`just mocks`). Mock staleness should be verified in CI like other codegen
- sqlc for SQL→Go, tygo for Go→TypeScript, codegen outputs committed and CI-verified

### Kotlin (KSP)

- **KSP + KotlinPoet for annotation-driven codegen**: use a dedicated `codegen/` Gradle subproject as a KSP processor applied to the main module. Generate typed snapshot data classes, `snapshot()` extension functions, and TOML-compatible `@TomlComments` annotations from `@PropertyDoc`/`@GenerateConfigSnapshot` annotations. KotlinPoet handles code generation. This is the Kotlin-native equivalent of Rust's `build.rs` — annotation processing as a compile-time artifact generator
- Generated output lives in the build directory (`common/build/`), not committed — KSP re-runs on each build

### TypeScript

- See [cross-language-type-generation](./cross-language-type-generation.md) for ts-rs barrel index generation

## Anti-Patterns

- Manually editing generated files (changes are overwritten on next regen)
- Stale generated files not caught in CI
- Generating into source directories without clear separation from hand-written code

## Related Topics

- [cross-language-type-generation](./cross-language-type-generation.md) — ts-rs and tygo conventions for backend→frontend type generation

## Open Questions

- Cross-language codegen orchestration (ordering, dependency between generators)
