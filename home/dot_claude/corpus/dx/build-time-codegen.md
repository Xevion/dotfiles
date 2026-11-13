---
name: build-time-codegen
category: dx
last_audited: 2026-11-12
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

- **Commit generated output by default**: generated files are committed (not .gitignored) so consumers can use the package without running codegen. CI verifies staleness via `regenerate && git diff --exit-code`. **The exception is output that only feeds the compiler**: a `cxx` bridge, protobuf bindings or translated shaders written to `OUT_DIR` at build time are not committed, because nothing outside the build reads them. Where checked-in bindings exist (steamlib, stardew-bot), the build script verifies them against the source of truth instead
- **Justfile recipe for regeneration**: expose codegen as a `just` recipe (e.g., `just generate`, `just mocks`) so the command is discoverable and consistent

## Language-Specific

### Rust

- **build.rs for compile-time codegen**: generate static data structures (PHF maps from JSON atlases, embedded asset tables) at compile time. Output to `$OUT_DIR` and `include!` in source
- **`cxx-gen` from a path build-dependency when `macro_rules!` cannot author a `#[cxx::bridge]`** (idakit). A declarative spec (`Domain`, `FnSpec`, `BodyKind`) feeds the Rust bridge declaration, the C++ header and the shim glue; bodies stay hand-written in per-domain translation units, with trivial ones templated. `cxx_build::bridge()` parses bridge source textually with `syn`, so a macro can never author one. See [ffi-facade-and-unsafe-containment](../architecture/ffi-facade-and-unsafe-containment.md)
- **One shader source generates everything else**: `build.rs` joins WGSL parts, validates with naga, translates to HLSL, MSL and GLSL ES, and emits Rust consts for the uniform layout, bindings and backend register numbers. Snapshot tests pin the translated binding interfaces. See [gpu-shader-pipeline](../architecture/gpu-shader-pipeline.md)
- **Vendor third-party C without committing it** (ferrite-uefi): a pinned release (tag, URL, sha256 in `vendor.toml`) is fetched, configured and snapshotted by a tool subcommand into a gitignored directory. Verify the configure result with `expect_set` and `expect_unset` lists, because kconfig drops an option with unmet dependencies without a word. `build.rs` passes every compiler flag explicitly, and a check mode reports drift in tracked generated stubs. Make the vendor step a prerequisite of check, lint and test
- **Vendor `protoc`** (or the equivalent) rather than depending on PATH, and check a build-time metadata table for stale and unused rows
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
