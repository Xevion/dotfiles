---
name: logging-observability
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: src/logging/
    note: Custom tracing formatters (pretty + JSON), request ID middleware, runtime format selection
  - repo: Xevion/doujin-ocr-summary
    path: internal/middleware/ + internal/logging/
    note: Go slog with path-based levels, slog-formatter, context propagation
  - repo: Xevion/instant-upscale
    path: frontend/src/lib/logging.ts
    note: Batching dev-forward sink, E2E console sentinel capture
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

- Batching dev-forward sink: browser/worker log records serialized to flat format, batched (count threshold or debounce timer), POSTed to dev server relay for unified terminal output
- E2E console.debug sentinel capture: build-flag-gated sink emits structured logs as `'__LOGTAPE__' + JSON`, Playwright intercepts via `page.on('console')`, tests assert on structured records

### Go

- Path-based request log level: `/api/*` at Info, SSR pages at Debug, dev assets (HMR, node_modules) at custom Trace level (`slog.LevelDebug - 4`). Prevents proxy noise without suppressing API traffic
- slog-formatter middleware for value-level transformations (duration humanization, integer comma-formatting, typed `Pct` wrapper). Keeps structured fields intact while improving readability
- Context-based logger propagation: attach pre-seeded `*slog.Logger` (with request_id) to context in middleware, retrieve with `LoggerFromContext` helper with default fallback

## Anti-Patterns

- `println!` / `console.log` debugging left in production code
- Logging sensitive data (tokens, passwords, PII)
- Log-and-throw (double reporting the same error)
- Using `--quiet` on commands you're trying to diagnose

## Open Questions

- OpenTelemetry integration for tracing across service boundaries
