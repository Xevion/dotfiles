---
name: cross-language-type-generation
category: dx
last_audited: 2026-04-10
exemplars:
  - repo: Xevion/instant-upscale
    path: crates/server/ + frontend/src/lib/bindings/
    note: ts-rs generating TypeScript from Rust structs with optional_fields and type overrides
  - repo: Xevion/doujin-ocr-summary
    path: internal/server/types.go + web/src/lib/types.gen.ts
    note: tygo generating TypeScript from Go structs with tstype tag overrides
  - repo: Xevion/glint
    path: backend models + schemas/ + frontend/src/lib/bindings/
    note: "ts-rs with JSON schema intermediary, DateTime/JSONB type overrides, snake_case wire format"
  - repo: local/inkwell
    path: tygo.yaml + web/src/lib/types.gen.ts
    note: "tygo type_mappings for pgx nullable types, dual codegen verification (sqlc + tygo) in CI"
  - repo: local/Applyhelm
    path: tempo.config.ts + packages/shared-types/bindings/
    note: "Reference mtime-gated ts-rs preflight: tmpdir isolation, file-by-file Buffer.compare diff, stale cleanup, idempotent barrel generator, CI verify via runner.temp + diff -r"
---

# Cross-Language Type Generation

## Philosophy

Generated types are the single source of truth for API contracts. Backend defines the canonical type; frontend consumes the generated binding. No hand-maintained TypeScript interfaces that mirror backend types.

## Conventions

- **CI verification via regen+diff**: regenerate bindings in CI and `git diff --exit-code` the output. Stale generated files are a build failure, not a warning
- **mtime-gated auto-regen preflight with tmpdir isolation**: rather than regenerating bindings manually or in a pre-commit hook, wire the regen into a typed build-tool preflight (`tempo.config.ts`, `scripts/bindings.ts`) that compares source mtime to artifact mtime. When source is newer: create a tmpdir, set `TS_RS_EXPORT_DIR` to the tmpdir, run `cargo test export_bindings`, diff the tmpdir against the committed output file-by-file via `Buffer.compare`, clean up files that no longer exist in the new output, and write only the changed files back. Tmpdir isolation prevents partial regeneration from corrupting the workspace on failure. Pair with an idempotent barrel generator (skip write when content unchanged) to avoid spurious git diffs on re-run. CI runs the same preflight with `TS_RS_EXPORT_DIR=$RUNNER_TEMP/bindings` and then `diff -r` against the committed output — catches drift without depending on a pre-commit hook being installed

## Language-Specific

### Rust (ts-rs)

- `#[derive(TS)]` + `#[ts(export)]` on all request/response types. `#[ts(optional_fields)]` maps `Option<T>` to optional TypeScript properties
- **Type overrides for unmapped types**: `#[ts(type = "string")]` for `DateTime<Utc>`, `#[ts(as = "Option<String>")]` for `Option<DateTime<Utc>>`, `#[ts(optional, type = "Array<string>")]` for `Json<Vec<String>>` (JSONB columns)
- **Struct-level `serde(rename_all = "camelCase")`**: always prefer struct-level `rename_all` over per-field `#[serde(rename)]` on types exposed to the frontend. Per-field renames add maintenance burden and risk inconsistency
- **Wire format casing**: when both Rust and JSON use `snake_case`, `serde(rename_all = "camelCase")` is unnecessary. Document the project's casing convention explicitly so contributors don't add it reflexively
- **Single-file vs barrel-index output**: for small type surfaces (~10 types), `#[ts(export_to = "bindings.ts")]` targeting a single file is acceptable. The barrel-index pattern (`frontend/src/lib/bindings/` with auto-maintained index) is warranted when the binding surface grows beyond ~10 types or is consumed selectively across many import sites
- Output to `frontend/src/lib/bindings/` with auto-maintained barrel index (or single `bindings.ts` for small surfaces)

### Go (tygo)

- `tygo.yaml` with `include_files` to scope generation to a specific types file. Output to `web/src/lib/types.gen.ts`
- **`type_mappings` for pgx nullable types**: `pgtype.Text` → `"string | null"`, `pgtype.Int4` → `"number | null"`, `pgtype.Timestamptz` → `"string | null"`. Also map `time.Time` and `uuid.UUID` → `"string"` for database scalar overrides
- `json.RawMessage` maps to `any` by default — override with `tstype` struct tag for known shapes
- CI verification: `tygo generate && git diff --exit-code -- web/src/lib/types.gen.ts`

### Kotlin (schemars JSON Schema intermediary)

- For multi-language systems without a shared IDL (e.g., Rust backend + Kotlin mod), derive `#[derive(schemars::JsonSchema)]` alongside `#[derive(TS)]` on mod-facing types. Export JSON schemas via a test (`export_all_schemas`). Kotlin reads them in `SchemaCompatibilityTest` to validate deserialization compatibility. This is a third cross-language type generation path — complementary to ts-rs (Rust→TS) and tygo (Go→TS)
- Pair with mtime-based schema regeneration in CI to catch drift

## Anti-Patterns

- **Hand-maintained TypeScript mirrors of Rust/Go enums**: TypeScript unions with a comment acknowledging they mirror a backend enum (e.g., `// These mirror the Rust enums in protocol.rs`) are a known maintenance hazard. Use ts-rs with `#[ts(export)]` on Rust message enums — the serde tag field (`msg_type`) maps naturally to a TypeScript discriminated union. This is the canonical motivation for ts-rs adoption
- `json.RawMessage` without tstype override — produces `any` in generated TypeScript
- Stale generated files not caught in CI

## Open Questions

- Protobuf vs language-specific codegen trade-offs
- OpenAPI as an intermediate representation
- Handling breaking changes in generated types
