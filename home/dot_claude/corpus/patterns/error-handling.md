---
name: error-handling
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: src/banner/errors.rs
    note: thiserror enum with variant-specific handling via downcast_ref
  - repo: Xevion/banner
    path: src/scraper/worker.rs
    note: Recoverable/Unrecoverable job error enum for explicit retry policy
---

# Error Handling

## Philosophy

Typed errors — errors are data, not strings. Separate expected failures (typed enums) from unexpected failures (opaque wrappers). The type system should encode retry/recovery policy, not scattered if/else logic.

## Conventions

- **Error enums for expected failures**: each module/boundary defines its own error type. Callers `match` or `downcast_ref` on variants to take different action
- **Recoverable vs Unrecoverable at the type level**: in job/task systems, use a two-variant error enum to make retry policy explicit. Parse/corruption failures are unrecoverable; transient network errors are recoverable

```rust
// Pattern: typed retry policy
enum JobError {
    Recoverable(anyhow::Error),    // network timeout → retry
    Unrecoverable(anyhow::Error),  // parse failure → delete job
}
```

- **Error context at boundaries**: attach context (status codes, URLs, entity names) to errors at the boundary where it's available, not at the catch site
- **Single sanitization point**: a boundary function (e.g. `db_error()`) logs the raw error and returns a sanitized message. Handlers never log-and-throw

## Language-Specific

### Rust

- `thiserror` for typed error enums at module boundaries, `anyhow` for opaque upstream errors in application code
- `#[source]` attaches cause to struct variants, `#[from]` for transparent conversion
- Callers use `downcast_ref::<SpecificError>()` for variant-specific handling

### TypeScript

<!-- Placeholder: discriminated union Result types, Zod parse errors -->

### Go

<!-- Placeholder: errors.Is/As, fmt.Errorf with %w, sentinel errors -->

## Anti-Patterns

- Catch-all handlers that swallow errors silently
- String matching on error messages (`if err.to_string().contains("timeout")`)
- `panic!`/`throw` for control flow — only for bugs/invariant violations
- Implicit retry logic scattered across callers instead of typed at the error level

## Open Questions

- Error telemetry standards (when to emit metrics vs just log)
- Typed exceptions proposals in various languages
