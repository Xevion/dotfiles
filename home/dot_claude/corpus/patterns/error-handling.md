---
name: error-handling
category: patterns
last_audited: 2026-03-26
exemplars: []
---

# Error Handling

## Philosophy

<!-- Typed errors, no stringly-typed, separate library errors from application errors, errors are data -->

## Conventions

<!-- Error enums for expected failures, panic/throw only for bugs, error context at boundaries -->

## Language-Specific

### Rust
<!-- thiserror for library crates, anyhow for applications, ? operator chains, custom error enums per module -->

### TypeScript
<!-- Discriminated union Result types, Zod parse errors, never throw from library code -->

### Go
<!-- errors.Is/As for inspection, fmt.Errorf with %w for wrapping, sentinel errors for expected cases -->

## Anti-Patterns

<!-- Catch-all handlers that swallow errors, string matching on error messages, panic/throw for control flow -->

## Open Questions

<!-- Error telemetry standards, typed exceptions proposals -->
