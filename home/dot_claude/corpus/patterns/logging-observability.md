---
name: logging-observability
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: src/logging/
    note: Custom tracing formatters (pretty + JSON), request ID middleware, runtime format selection
---

# Logging & Observability

## Philosophy

Structured logging always. Tracing spans for request lifecycle. Log format is a deployment concern, not a code concern — same call sites produce human-readable dev output and machine-parseable JSON in production.

## Conventions

- **Runtime format selection**: choose log format via CLI flag or config, not compile-time features. Pretty for dev, JSON for production
- **Request correlation**: generate or inherit a request ID (prefer upstream edge headers like `X-Railway-Request-Id`, `X-Request-Id`) and attach it to a span that wraps the entire request lifecycle
- **Status-proportional log levels**: 2xx/3xx at debug, 4xx at info, 5xx at warn/error. Keeps normal traffic out of INFO while surfacing errors automatically
- **Sane filter defaults**: suppress noisy internal modules (session middleware, HTTP retry loops) at `warn` level while keeping application modules at the configured level

```rust
// Pattern: EnvFilter with per-module overrides
let filter = EnvFilter::new(format!(
    "warn,myapp={level},myapp::middleware=warn,myapp::session=warn"
));
```

## Language-Specific

### Rust

- `tracing` crate with spans and the `#[instrument]` attribute macro
- Custom `FormatEvent` + `FormatFields` implementations for per-environment formatting without changing call sites
- Techniques: group dotted field names into inline structs, type-aware coloring in dev, never truncate the `error` field, abbreviate high-cardinality IDs (ULIDs) on log lines

### TypeScript

<!-- Placeholder: pino/winston, OpenTelemetry, request ID middleware -->

### Go

<!-- Placeholder: slog, context-based logger propagation -->

## Anti-Patterns

- `println!` / `console.log` debugging left in production code
- Logging sensitive data (tokens, passwords, PII)
- Log-and-throw (double reporting the same error)
- Using `--quiet` on commands you're trying to diagnose

## Open Questions

- OpenTelemetry maturity and adoption per language ecosystem
- Log aggregation preferences (Loki vs Elasticsearch vs Axiom)
