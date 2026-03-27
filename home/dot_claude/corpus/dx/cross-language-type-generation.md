---
name: cross-language-type-generation
category: dx
last_audited: 2026-03-26
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
---

# Cross-Language Type Generation

## Philosophy

<!-- Generated types are the single source of truth for API contracts. Backend defines the canonical type; frontend consumes the generated binding. No hand-maintained TypeScript interfaces that mirror backend types. -->

## Conventions

<!-- ts-rs (Rust→TS), tygo (Go→TS), protobuf as language-agnostic alternative. Barrel index generation, staleness detection, CI verification via regen+diff -->

## Language-Specific

### Rust (ts-rs)

- `#[derive(TS)]` + `#[ts(export)]` on all request/response types. `#[ts(optional_fields)]` maps `Option<T>` to optional TypeScript properties
- **Type overrides for unmapped types**: `#[ts(type = "string")]` for `DateTime<Utc>`, `#[ts(as = "Option<String>")]` for `Option<DateTime<Utc>>`, `#[ts(optional, type = "Array<string>")]` for `Json<Vec<String>>` (JSONB columns)
- **Wire format casing**: when both Rust and JSON use `snake_case`, `serde(rename_all = "camelCase")` is unnecessary. Document the project's casing convention explicitly so contributors don't add it reflexively
- Output to `frontend/src/lib/bindings/` with auto-maintained barrel index

### Go (tygo)

<!-- tygo.yaml config, tstype struct tag overrides for nullable pointers and omitempty. json.RawMessage → any (override with tstype for known shapes). Output to web/src/lib/types.gen.ts -->

## Anti-Patterns

<!-- Hand-maintaining TypeScript interfaces, json.RawMessage without tstype override, stale generated files not caught in CI -->

## Open Questions

<!-- Protobuf vs language-specific codegen, OpenAPI as an intermediate representation, handling breaking changes in generated types -->
