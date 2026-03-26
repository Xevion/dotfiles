---
name: rust
category: languages
last_audited: 2026-03-26
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
---

# Rust

## Philosophy

Ownership-centric design, zero-cost abstractions, compile-time guarantees over runtime checks. Prefer borrowing over cloning, iterators over loops, and typed errors over string messages.

## Conventions

- **thiserror/anyhow split**: `thiserror` for module-boundary error enums (pattern-matchable by callers), `anyhow` for opaque upstream errors. Use `#[source]` on struct variants to attach cause, `#[from]` on transparent catch-all variants for ergonomic conversion

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
- **Iterator combinators**: `.map()`, `.filter()`, `.find_map()`, `.collect()` over explicit for loops
- **Lifetime-bound filter structs**: short-lived query/filter structs borrow from the request context (`&'a str`, `&'a [String]`) rather than cloning into owned Strings
- **SQLx QueryBuilder**: use `QueryBuilder<Postgres>` for dynamic multi-condition queries instead of string interpolation
- **ts-rs for TypeScript contracts**: `#[derive(TS)]` + `#[ts(export)]` + `serde(rename_all = "camelCase")` on all shared API types. Generated bindings are the frontend's source of truth
- **ts-rs optional fields**: `#[ts(optional_fields)]` on structs with `Option<T>` fields produces `field?: T` in TypeScript output. Use `#[ts(type = "string")]` for types like `NaiveDateTime` that have no automatic TS mapping
- **Manual `From` for domain-logic conversions**: prefer `#[from]` for transparent catch-alls, but write manual `From<sqlx::Error>` impls when conversion carries domain logic (e.g., mapping DB error codes to `NotFound`/`Conflict`)
- **`Params<'a>` naming suffix**: consider `Params<'a>` for insert/upsert input structs that borrow from request context — distinguishes them from read-only filter structs

## Anti-Patterns

- Excessive `.clone()` to satisfy the borrow checker — restructure ownership instead
- Stringly-typed errors or `anyhow` at public API boundaries (hides structure from callers)
- `Rc<RefCell<T>>` as a first resort — reach for it only when shared ownership is genuinely needed
- Manual `From` impls when `#[from]` or `#[source]` suffice — but this is the right choice when conversion carries domain logic (see Conventions above)

## Open Questions

- When to reach for `async` vs OS threads (CPU-bound work boundaries)
- Macro hygiene standards and when to prefer proc macros vs declarative macros
