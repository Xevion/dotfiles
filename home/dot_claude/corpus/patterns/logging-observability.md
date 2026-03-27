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
  - repo: Xevion/glint
    path: backend/src/logging.rs
    note: "CompactFields with field-level transforms, coordinated LOG_JSON across Rust + SvelteKit"
  - repo: local/inkwell
    path: internal/middleware/ + web/vite-plugin-json-logger.ts
    note: "Three-layer LOG_JSON coordination (Go + SvelteKit + Vite plugin), LevelTrace named constant"
  - repo: Xevion/xevion.dev
    path: src/middleware/request_id.rs + web/console-logger.js
    note: "RequestIdLayer with upstream header trust + ULID fallback, console-logger preload"
  - repo: Xevion/railway-collector
    path: internal/logging/
    note: "slog.LogValuer Pct type, composable handler chain, FilteringHandler, status-proportional RoundTripper"
  - repo: local/bose-re
    path: crates/bose-cli/src/main.rs + commands/common.rs
    note: "Three-tier log level override, OutputBuffer for async CLI output integrity"
  - repo: local/topaz-video-ai-re
    path: inference/src/main.rs + pipeline.rs
    note: "Interactive vs non-interactive tracing, EMA-based outlier detection"
  - repo: Xevion/ferrite
    path: src/tui/
    note: "Channel-backed tracing Layer for ratatui TUI"
---

# Logging & Observability

## Philosophy

Structured logging always. Tracing spans for request lifecycle. Log format is a deployment concern, not a code concern — same call sites produce human-readable dev output and machine-parseable JSON in production.

## Conventions

- **Runtime format selection**: choose log format via CLI flag or config, not compile-time features. Pretty for dev, JSON for production
- **Request correlation**: generate or inherit a request ID (prefer upstream edge headers like `X-Railway-Request-Id`, `X-Request-Id`) and attach it to a span that wraps the entire request lifecycle. In Rust/Axum, implement as a Tower middleware layer that combines upstream header trust (configurable via CLI flag), ULID fallback generation, span attachment, and status-proportional response logging
- **Status-proportional log levels**: 2xx/3xx at debug, 4xx at info, 5xx at warn/error. Keeps normal traffic out of INFO while surfacing errors automatically
- **Sane filter defaults**: suppress noisy internal modules (session middleware, HTTP retry loops) at `warn` level while keeping application modules at the configured level
- **Coordinated log format across co-located services**: in a container running multiple subsystems, coordinate log format via a single shared env var (`LOG_JSON`). All layers must implement the same selection logic (`env var override → build-profile default`). This extends beyond two layers — inkwell coordinates three: Go slog, SvelteKit LogTape, and a custom Vite plugin that intercepts Vite's logger to reformat stray console output into structured JSON

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
- **Field-level transforms via builder pattern**: `CompactFields` builder registers per-field transforms (e.g., truncate high-cardinality ULIDs to `4..6` chars in pretty mode) without affecting JSON output. Use `writer.has_ansi_escapes()` rather than `is_tty()` for the ANSI-safe check in custom `FormatEvent` impls
- **Three-tier log level override for CLIs**: `RUST_LOG` takes full control (verbatim filter string), `LOG_LEVEL` sets per-crate level while holding external crates at `warn`, `-v` flag controls verbosity when neither env var is set. All tracing routed to stderr explicitly (`.with_writer(std::io::stderr)`) so stdout is reserved for structured program output
- **OutputBuffer for async CLI output integrity**: when a command performs async I/O before printing results, buffer all stdout writes and flush atomically after async work completes. Prevents stderr (tracing) and stdout interleaving. Companion `buf_println!` macro provides `println!`-compatible ergonomics. Streaming commands (monitoring, tailing) are exempt
- **Interactive vs non-interactive tracing**: runtime-selectable subscriber layers — TUI layer (ratatui channel-backed) if TTY, plain `fmt` layer if not. Extends "format is a deployment concern" beyond dev/prod to interactive vs non-interactive environments
- **Channel-backed tracing Layer for TUI**: implement a custom `tracing::Layer` that serializes log records into an `mpsc` channel; the TUI rendering thread drains the channel each frame. Keeps log records in-process and lets the TUI control presentation
- **EMA-based outlier detection in hot paths**: use exponential moving average (α=0.05) as a running baseline and emit a structured warning when a measurement exceeds 5×EMA. Lighter than a sliding window and naturally adapts to warm-up

### TypeScript

- Batching dev-forward sink: browser/worker log records serialized to flat format, batched (count threshold or debounce timer), POSTed to dev server relay for unified terminal output
- E2E console.debug sentinel capture: build-flag-gated sink emits structured logs as `'__LOGTAPE__' + JSON`, Playwright intercepts via `page.on('console')`, tests assert on structured records

### Go

- Path-based request log level: `/api/*` at Info, SSR pages at Debug, dev assets (HMR, node_modules, `/@`, `/.svelte-kit/`, `/styled-system/`) at custom Trace level. Define `LevelTrace = slog.LevelDebug - 4` as a named constant rather than an inline value. Prevents proxy noise without suppressing API traffic
- slog-formatter middleware for value-level transformations (duration humanization, integer comma-formatting, typed `Pct` wrapper). Keeps structured fields intact while improving readability
- Context-based logger propagation: attach pre-seeded `*slog.Logger` (with request_id) to context in middleware, retrieve with `LoggerFromContext` helper with default fallback
- **Composable slog handler chain**: base handler (tint or JSON) → slog-formatter middleware → FilteringHandler. Each layer is a distinct concern — formatting, value transforms, noise suppression. The FilteringHandler silently drops records matching known-noisy substrings (e.g., `net/http` idle connection chatter)
- **Status-proportional levels in RoundTripper**: apply status-proportional log levels inside `http.RoundTripper` implementations, not just at HTTP handler middleware layers. Debug for normal responses, Warn for rate limits and errors. Keeps transport-level noise at Debug while surfacing rate limit events at Warn without requiring the caller to inspect every response

## Anti-Patterns

- `println!` / `console.log` debugging left in production code
- Logging sensitive data (tokens, passwords, PII)
- Log-and-throw (double reporting the same error)
- Using `--quiet` on commands you're trying to diagnose

## Open Questions

- OpenTelemetry integration for tracing across service boundaries
