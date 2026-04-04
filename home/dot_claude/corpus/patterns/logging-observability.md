---
name: logging-observability
category: patterns
last_audited: 2026-04-03
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
  - repo: Xevion/rustdoc-mcp
    path: src/tracing.rs
    note: "NEXTEST env detection for test-mode tracing, runtime format selection with TTY detection"
  - repo: Xevion/recall
    path: src/logging/sink.ts
    note: "Used-key deduplication in colored stderr sink, LogTape structured logging"
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
- **NEXTEST env detection for test-mode tracing**: detect test runner context via `NEXTEST` or `CARGO_TARGET_TMPDIR` env vars and switch to `DEBUG` level with `.with_test_writer()` for cargo test capture. Extends the "interactive vs non-interactive" subscriber selection to test runners — test output gets captured by the test framework rather than polluting stderr
- **Interactive vs non-interactive tracing**: runtime-selectable subscriber layers — TUI layer (ratatui channel-backed) if TTY, plain `fmt` layer if not. Extends "format is a deployment concern" beyond dev/prod to interactive vs non-interactive environments
- **Channel-backed tracing Layer for TUI**: implement a custom `tracing::Layer` that serializes log records into an `mpsc` channel; the TUI rendering thread drains the channel each frame. Keeps log records in-process and lets the TUI control presentation
- **WASM tracing with build-profile-gated filters**: for Rust compiled to WASM, use `wasm-tracing` with `tracing-subscriber::EnvFilter` for browser console output. Use `#[cfg(debug_assertions)]` to select debug vs production log levels at compile time (no runtime overhead). `set_report_logs_in_timings(true)` integrates logs with the browser performance timeline. This is the WASM-specific equivalent of "runtime format selection" for native targets
- **EMA-based outlier detection in hot paths**: use exponential moving average (α=0.05) as a running baseline and emit a structured warning when a measurement exceeds 5×EMA. Lighter than a sliding window and naturally adapts to warm-up

### TypeScript

- **Used-key deduplication in formatted sinks**: when a structured log formatter interpolates `{key}` references into the message template, extract those key names from the raw message and filter them out of the trailing `properties` display. Avoids showing `key=value` twice — once inlined in the message, once in the suffix. Keeps formatted output clean without losing structured data in the underlying record
- Batching dev-forward sink: browser/worker log records serialized to flat format, batched (count threshold or debounce timer), POSTed to dev server relay for unified terminal output
- E2E console.debug sentinel capture: build-flag-gated sink emits structured logs as `'__LOGTAPE__' + JSON`, Playwright intercepts via `page.on('console')`, tests assert on structured records

### Go

- Path-based request log level: `/api/*` at Info, SSR pages at Debug, dev assets (HMR, node_modules, `/@`, `/.svelte-kit/`, `/styled-system/`) at custom Trace level. Define `LevelTrace = slog.LevelDebug - 4` as a named constant rather than an inline value. Prevents proxy noise without suppressing API traffic
- slog-formatter middleware for value-level transformations (duration humanization, integer comma-formatting, typed `Pct` wrapper). Keeps structured fields intact while improving readability
- Context-based logger propagation: attach pre-seeded `*slog.Logger` (with request_id) to context in middleware, retrieve with `LoggerFromContext` helper with default fallback
- **Composable slog handler chain**: base handler (tint or JSON) → slog-formatter middleware → FilteringHandler. Each layer is a distinct concern — formatting, value transforms, noise suppression. The FilteringHandler silently drops records matching known-noisy substrings (e.g., `net/http` idle connection chatter)
- **Status-proportional levels in RoundTripper**: apply status-proportional log levels inside `http.RoundTripper` implementations, not just at HTTP handler middleware layers. Debug for normal responses, Warn for rate limits and errors. Keeps transport-level noise at Debug while surfacing rate limit events at Warn without requiring the caller to inspect every response

## Value Display Patterns

Structured log values are only useful if they're readable. Apply formatting at the type or sink level — never at call sites — so call sites stay clean and formatting stays consistent.

### Bytes / Storage Sizes

Always display byte counts as human-readable. Raw integers are unreadable at scale.

- **IEC units** (1024-based: KiB, MiB, GiB) for memory, file sizes, buffer allocations
- **SI units** (1000-based: KB, MB, GB) for network throughput — matches how bandwidth is advertised
- Log both the raw value and the formatted string for exact machine-parseable + human-readable output: `{ bytes: 1048576, bytes_human: "1.0 MiB" }`
- In Rust: `bytesize` crate provides `ByteSize(n)` with `Display` and serde support — use `%ByteSize(n)` in tracing fields. Or implement `struct Bytes(u64)` with a custom `fmt::Display`
- In Go: implement `slog.LogValuer` on a `ByteSize int64` type, return `slog.StringValue(formatIEC(b))`. Wire into the slog-formatter middleware chain — call sites log `slog.Int64("size", n)` and the formatter upgrades it
- In TypeScript: format in the sink's field renderer, not at call sites. Call sites emit `{ bytes: n }` as a raw number; the sink transforms it. Keeps business logic independent of display format

### Durations

Never log raw nanoseconds or milliseconds as plain integers — they're unreadable without knowing the unit.

- Sub-microsecond → `ns`, sub-millisecond → `µs`, sub-second → `ms`, sub-minute → `s` with one decimal, longer → `1m23s` or humantime format
- In Rust: `humantime` crate (`humantime::format_duration(d)`) or `Duration::as_secs_f64()` with manual µs/ms/s selection. Implement a `LogDuration(Duration)` newtype with `fmt::Display` for tracing fields
- In Go: `time.Duration.String()` is reasonable for basic cases; for slog, use formatter middleware that detects `time.Duration` type and calls `.String()`. For sub-millisecond precision, format manually as `"123µs"` rather than `"0.000123s"`
- In TypeScript: express as string label at the call site when duration is computed locally (`duration: "142ms"`); use a sink-level number→string transform when duration comes from an external source as a raw number

### Rates and Throughput

Express as auto-scaled `ops/s`, `req/s`, or `bytes/s`. Useful in batch job progress logging, stream processors, and retry loops.

- Pair with a time window: `{ rate: "1.2k req/s", window_s: 5 }`
- Use the same IEC/SI distinction as byte sizes when rate involves data volume
- For percentile reporting (p50/p95/p99), log as a structured object, not a flat string: `{ p50_ms: 12, p95_ms: 87, p99_ms: 340 }`

### Truncated Collections

When logging a `Vec`/slice/array, truncate after N items (5 is a reasonable default) and append a count of the remainder: `[a, b, c, d, e, …+7]`. Prevents megabyte log lines from inner-loop or batch operations.

- In Rust: implement `fmt::Display` for a `Truncated<'a, T>(&'a [T], usize)` wrapper. Use as `tracing::field::display(&Truncated(&items, 5))`
- In Go: helper function `truncatedSlice(items []any, max int) string` returns formatted string with `…+N more` suffix
- Never log entire request/response bodies inline — log length + a content-type label, then log the body separately at trace level behind an explicit `--log-bodies` flag or env var

### Sensitive Value Masking

Mask at the type/LogValuer level, not at the call site. Call sites should be able to log a token field freely — the masking logic should be structural.

- Show only the last 4 characters: `sk-...a4f2`. Enough to identify which key without exposing it
- In Rust: `struct MaskedSecret(String)` implementing `fmt::Display` as `***{last4}`. Store the pre-masked string at construction time so formatting is free
- In Go: `type Secret string` implementing `LogValue() slog.Value` returning `slog.StringValue("***" + s[max(0,len(s)-4):])`
- Audit: never let `Debug` format print the raw inner value — ensure `fmt::Debug` is also overridden to mask, not just `fmt::Display`

### Large Integers

Comma-format large integers for human readability: `1,234,567` not `1234567`. Applies to row counts, queue depths, cache hit counts, memory addresses.

- Already applied in Go via slog-formatter middleware detecting `int`/`int64` types
- In Rust: `num-format` crate, or a `CommaSep(u64)` newtype with `fmt::Display` that groups digits. Use selectively for counts likely to exceed 10,000 — don't over-apply
- In TypeScript: `n.toLocaleString('en-US')` in the sink formatter for numeric fields matching known high-cardinality field names (`count`, `total`, `rows`, `hits`, `misses`)

### Typed Semantic Wrappers

Extend the `Pct` pattern (Go section) to any value where the raw number is ambiguous or misleading:

| Raw value | Ambiguity | Wrapper |
|---|---|---|
| `0.9542` | Is this 95.42% or 95.42 of something? | `Pct(0.9542)` → `"95.4%"` |
| `1048576` | Bytes? Elements? | `Bytes(1048576)` → `"1.0 MiB"` |
| `142_000_000` | Nanoseconds? Unixtime? | `Nanos(142_000_000)` → `"142ms"` |
| `3` | Retries? Attempts? Factor? | Field name is the context; no wrapper needed |

- Implement `LogValue()`/`fmt::Display` on the wrapper, not on the raw type
- Keep wrappers in a shared `logging` or `fmt` module so they're reused, not reinvented per call site

### Nullable / Optional Values

Log missing values explicitly rather than as empty strings. Empty string is ambiguous — it could mean the field is absent, was set to `""`, or was never populated.

- In Rust: `Option<T>` fields in tracing: use `field = ?opt` for `Debug` formatting, which renders `Some(val)` / `None` explicitly
- In Go: use a helper `slog.Any("field", maybeNilPtr)` — slog renders `nil` as the JSON `null`, which is unambiguous in structured output
- In TypeScript: distinguish `undefined` (field absent) from `null` (field present but empty) — choose a consistent convention and document it in the sink

## Anti-Patterns

- `println!` / `console.log` debugging left in production code
- Logging sensitive data (tokens, passwords, PII)
- Log-and-throw (double reporting the same error)
- Using `--quiet` on commands you're trying to diagnose

## Related Topics

- [go](../languages/go.md) — slog LogValuer, LevelTrace, FilteringHandler, handler chain patterns
- [python](../languages/python.md) — stdlib logging conventions, level discipline, centralized setup

## Open Questions

- OpenTelemetry integration for tracing across service boundaries
