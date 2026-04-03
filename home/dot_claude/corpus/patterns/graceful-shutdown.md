---
name: graceful-shutdown
category: patterns
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/rustdoc-mcp
    path: src/worker.rs
    note: "ServiceContext with CancellationToken + TaskTracker, cancellation-aware tick()"
  - repo: Xevion/recall
    path: src/utils/shutdown.ts
    note: "Singleton ShutdownController, AbortSignal threading, dual Ctrl+C escalation"
  - repo: local/inkwell
    path: internal/shutdown/
    note: "Go WaitGroup + sync.Once tracker with bounded drain timeout"
---

# Graceful Shutdown

## Philosophy

Every long-running process must handle termination signals cleanly. In-flight work completes or checkpoints before exit. Abrupt kills are a last resort, not the norm. Resources (database connections, file handles, network sockets) are released in reverse acquisition order.

## Conventions

- **Signal handler with escalation**: first SIGINT/SIGTERM signals graceful shutdown (finish current unit of work). Second signal force-quits via `process.exit(1)`. Auto force-quit after a context-dependent timeout (e.g., 5s for fast commands, 30s for analysis pipelines)
- **AbortSignal threading through write pipelines**: pass an `AbortSignal` through every write operation in the pipeline. Check `signal.aborted` between units of work. Read-only commands rely on the force-quit timeout instead of explicit signal checks
- **Bounded drain with timeout**: after signaling shutdown, wait for in-flight operations to complete with a bounded timeout. Use `select!` (Rust), `Promise.race` (TypeScript), or `select` on a done-channel (Go) to race completion against the timeout
- **Singleton shutdown controller**: a process-wide singleton coordinates signal handlers, timeout configuration, and abort signal distribution. Per-command timeouts are set in a `preAction` hook or equivalent

## Language-Specific

### Rust

- **CancellationToken + TaskTracker**: `tokio_util`'s `CancellationToken` broadcasts shutdown; `TaskTracker` tracks in-flight spawned tasks. A `ServiceContext` wraps both with `tick()` (cancellation-aware interval) and `shutdown(timeout)` (cancel + drain). See [concurrency-async](../architecture/concurrency-async.md) for the full pattern
- **`Arc<AtomicBool>` for synchronous loops**: for `std::thread`-based workers where async cancellation is unnecessary, a shared `AtomicBool` flag checked between iterations is sufficient

### TypeScript

- **ShutdownController singleton with dual Ctrl+C**: install signal handlers per-command with context-dependent timeouts. First signal sets abort, current unit finishes. Second signal calls `process.exit(1)`. The controller is a singleton accessed via `getShutdownController()`
- **`runPool()` with abort-aware concurrency**: a signal-aware concurrency pool replaces chunk-based `Promise.all`. Supports abort signal and circuit breaker via `onTaskComplete` callback

### Go

- **`sync.WaitGroup` + `sync.Once`-protected close channel**: `Add()`/`Done()` bracket in-flight operations. `Stop()` closes the channel via `once.Do`. `Wait(timeout)` drains with a bounded timeout using a goroutine + `select`

## Anti-Patterns

- Force-killing processes that hold database locks (risk of corruption, especially single-writer DBs like DuckDB)
- Fixed `time.Sleep` between shutdown phases instead of signaling via channels
- Unbounded drain waits (hung operations block shutdown forever)
- Ignoring `AbortSignal` / `CancellationToken` in write pipelines

## Open Questions

- Graceful shutdown in serverless/edge environments with hard timeout limits
