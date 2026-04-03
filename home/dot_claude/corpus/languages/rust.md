---
name: rust
category: languages
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/banner
    path: src/banner/errors.rs
    note: thiserror enum with #[source] struct variants and #[from] transparent catch-all
  - repo: Xevion/banner
    path: src/web/error.rs
    note: Extension traits on Option/Result for domain-specific error mapping
  - repo: Xevion/instant-upscale
    path: crates/common/error module
    note: Manual From<sqlx::Error> with domain-logic mapping, or_not_found(entity, id) extension trait
  - repo: Xevion/Pac-Man
    path: pacman/src/error.rs
    note: Multi-level thiserror hierarchy (GameError aggregates sub-enums), error type as Bevy ECS Event
  - repo: local/bose-re
    path: crates/bose-protocol/src/error.rs
    note: thiserror + miette::Diagnostic with machine-readable codes, two-level hierarchy wrapping TransportError
  - repo: Xevion/ferrite
    path: src/alloc.rs
    note: thiserror with #[source] nix::Error wrapping, anyhow Context at application entry point
  - repo: Xevion/rustdoc-mcp
    path: src/error.rs
    note: "Five-level thiserror hierarchy with help() guidance method, shared futures singleflight"
---

# Rust

## Philosophy

Ownership-centric design, zero-cost abstractions, compile-time guarantees over runtime checks. Prefer borrowing over cloning, iterators over loops, and typed errors over string messages.

## Conventions

- **thiserror/anyhow split**: `thiserror` for module-boundary error enums (pattern-matchable by callers), `anyhow` for opaque upstream errors. Use `#[source]` on struct variants to attach cause, `#[from]` on transparent catch-all variants for ergonomic conversion. **Exception**: `anyhow`-only is appropriate for batch CLI tools and application entry points where errors are only displayed, never matched by callers — the split is for library crates and service boundaries
- **Multi-level thiserror hierarchies**: a top-level error enum aggregates domain sub-enums via `#[from]`. Pac-Man's `GameError` composes `AssetError`, `PlatformError`, `ParseError`, etc., and doubles as a Bevy ECS `Event` for surfacing errors into the game loop

```rust
#[derive(Debug, thiserror::Error)]
pub enum ClientError {
    #[error("Failed to parse response")]
    ParseFailed { status: u16, url: String, #[source] source: anyhow::Error },
    #[error(transparent)]
    RequestFailed(#[from] anyhow::Error),
}
```

- **Extension traits for domain-specific conversions**: attach methods like `.or_not_found()` to `Option<T>` and `.conflict_on_unique()` to `Result<T, sqlx::Error>` via extension traits, centralizing error mapping instead of scattering match arms
- **Named constructors on error types**: `ApiError::not_found(msg)`, `ApiError::bad_request(msg)` instead of direct struct construction
- **`help()` method on error sub-enums**: each error variant carries a `help() → Option<&'static str>` returning actionable resolution guidance. A top-level `user_message()` fuses `Display` + `help()` into a single consumer-facing string. Useful for MCP/CLI tools where callers need actionable next steps alongside the error description
- **Iterator combinators**: `.map()`, `.filter()`, `.find_map()`, `.collect()` over explicit for loops
- **Lifetime-bound filter structs**: short-lived query/filter structs borrow from the request context (`&'a str`, `&'a [String]`) rather than cloning into owned Strings
- **SQLx QueryBuilder**: use `QueryBuilder<Postgres>` for dynamic multi-condition queries instead of string interpolation
- **ts-rs for TypeScript contracts**: `#[derive(TS)]` + `#[ts(export)]` + `serde(rename_all = "camelCase")` on all shared API types. Generated bindings are the frontend's source of truth
- **ts-rs optional fields**: `#[ts(optional_fields)]` on structs with `Option<T>` fields produces `field?: T` in TypeScript output. Use `#[ts(type = "string")]` for types like `NaiveDateTime` that have no automatic TS mapping
- **Manual `From` for domain-logic conversions**: prefer `#[from]` for transparent catch-alls, but write manual `From<sqlx::Error>` impls when conversion carries domain logic (e.g., mapping DB error codes to `NotFound`/`Conflict`)
- **`Params<'a>` naming suffix**: consider `Params<'a>` for insert/upsert input structs that borrow from request context — distinguishes them from read-only filter structs
- **thiserror + miette::Diagnostic for user-facing library crates**: derive both `thiserror::Error` and `miette::Diagnostic` on error enums. Attach machine-readable codes via `#[diagnostic(code(crate::module::variant))]` on each variant. Callers get structured error codes for programmatic handling; end-users get miette's rich display with source spans and hints
- **Extension traits for non-sqlx error conversion**: the extension trait pattern generalizes beyond database errors — use it for any third-party crate with `Display`-but-not-`std::Error` error types. E.g., `OrtResultExt` with `.ort()` converts opaque ONNX Runtime errors to `anyhow::Error`
- **Dual-channel error separation for thread pools**: separate result and error into distinct `crossbeam-channel` channels rather than `Result<T, E>`-wrapping results. The error channel carries fatal worker state; the coordinator uses `select!` to race both with a timeout arm
- **Typed errors at Tauri IPC boundary**: implement `serde::Serialize` on command error enums so Tauri commands return `Result<T, CommandError>` instead of `Result<T, String>`. Eliminates stringly-typed failure paths across the IPC boundary

## Anti-Patterns

- Excessive `.clone()` to satisfy the borrow checker — restructure ownership instead
- Stringly-typed errors or `anyhow` at public API boundaries (hides structure from callers)
- `Rc<RefCell<T>>` as a first resort — reach for it only when shared ownership is genuinely needed
- Manual `From` impls when `#[from]` or `#[source]` suffice — but this is the right choice when conversion carries domain logic (see Conventions above)

- **cfg on individual enum variants**: use `#[cfg(not(target_arch = "wasm32"))]` on specific enum variants (e.g., `NetworkMode::Remote`) rather than gating the entire module. The type compiles on all targets with platform-specific variants restricted. Match arms referencing cfg-gated variants must also be cfg-gated

## Open Questions

- When to reach for `async` vs OS threads (CPU-bound work boundaries)
- Macro hygiene standards and when to prefer proc macros vs declarative macros
