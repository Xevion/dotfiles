---
name: concurrency-async
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/banner
    path: src/services/
    note: Service trait + ServiceManager with broadcast/mpsc, nested select! cancellation
  - repo: Xevion/instant-upscale
    path: crates/common/shutdown module
    note: ShutdownTracker/ShutdownToken RAII pair with atomic drain counter
  - repo: Xevion/glint
    path: backend/src/services/lifecycle.rs
    note: "TaskTracker + CancellationToken with ServiceContext tick/sleep helpers"
  - repo: Xevion/rustdoc-mcp
    path: src/worker.rs
    note: "ServiceContext pattern (independently derived), Shared<BoxFuture> singleflight for doc generation dedup"
  - repo: local/inkwell
    path: internal/shutdown/
    note: Go shutdown tracker with WaitGroup + sync.Once + select/timeout drain
  - repo: Xevion/railway-collector
    path: internal/collector/unified_scheduler.go
    note: Two-phase shutdown, independent housekeeping goroutine, dynamic rate-limiter adjustment
  - repo: local/topaz-video-ai-re
    path: inference/src/pipeline.rs
    note: crossbeam thread pool with dual result/error channels, AtomicBool quit flag
  - repo: local/bose-re
    path: crates/bose-protocol/src/device/session.rs
    note: Serial fan-out/collect with deadline-bounded recv loop for RFCOMM
  - repo: Xevion/ferrite
    path: src/main.rs
    note: thread::scope + AtomicBool, Rayon two-phase write-verify
---

# Concurrency & Async

## Philosophy

Structured concurrency — every spawned task has an owner that awaits its completion. Explicit cancellation. Backpressure by default. Avoid shared mutable state.

## Conventions

- **Service trait pattern**: define a trait with `run()` + `shutdown()` for every long-running subsystem. A manager coordinates lifecycle via broadcast channel (shutdown signal) and mpsc channel (completion notification). Parallel shutdown with a shared timeout budget

```rust
// Pattern: service lifecycle coordination
#[async_trait]
pub trait Service: Send + Sync {
    fn name(&self) -> &'static str;
    async fn run(&mut self) -> Result<()>;
    async fn shutdown(&mut self) -> Result<()>;
}
// Manager broadcasts shutdown to all, collects completions via mpsc
```

- **Nested select! for lifecycle-aware cancellation**: use different `select!` arms at different points in a task's lifecycle. Before acquiring a resource: cancel cleanly. After acquiring: do explicit cleanup in the cancellation arm (don't rely on async Drop)
- **Unified signal + exit handling**: race OS signals (SIGINT, SIGTERM) against service completion in a single top-level `select!`. Any unexpected service exit triggers the same graceful shutdown path as a signal. Stub platform-specific signals with `std::future::pending()` for cross-platform compilation
- **TaskTracker + CancellationToken (alternative to Service trait)**: `tokio_util::task::TaskTracker` with `CancellationToken` provides structured task ownership without requiring an interface. A `ServiceContext` wraps both and offers cancellation-aware `tick()` and `sleep()` helpers that eliminate boilerplate `select!` arms at each use site. `child_token()` isolates per-task cancellation. Shutdown broadcasts cancel via the root token and drains the tracker with a bounded timeout

```rust
// Pattern: ServiceContext with cancellation-aware helpers
pub async fn tick(&self, interval: &mut Interval) -> bool {
    tokio::select! {
        _ = interval.tick() => true,
        () = self.token.cancelled() => false,
    }
}
pub async fn shutdown(self, timeout: Duration) -> bool {
    self.token.cancel();
    self.tracker.close();
    tokio::select! {
        () = self.tracker.wait() => true,
        () = tokio::time::sleep(timeout) => false,
    }
}
```

## Language-Specific

### Rust

- Tokio runtime, `select!` for racing futures, `JoinSet` for structured task spawning
- `broadcast` for fan-out shutdown signals (each receiver gets its own copy)
- `tokio::time::timeout` for bounded waits
- **RAII shutdown token pattern**: `ShutdownTracker` hands out drop-on-complete `ShutdownToken` guards. Atomic counter tracks in-flight operations. `wait(timeout)` provides bounded draining. Complement to the Service trait — no lifecycle interface required per subsystem
- **`Shared<BoxFuture>` for singleflight deduplication**: use `futures::future::Shared` to allow multiple concurrent callers to await the same in-flight async operation (e.g., doc generation, cache warming) without triggering duplicate work. Store `SharedDocFuture` in a `HashMap` keyed by request identity; clean up after resolution. This is a Rust-native singleflight pattern — no external crate required. Note: the `Shared` trait bound requires `'static`, which may force error type erasure to `String` for `Result`-based futures
- **CPU-bound thread pool with crossbeam**: use `std::thread` + `crossbeam-channel` (bounded work queue, unbounded result/error queues) rather than Tokio tasks for long-running blocking work (e.g., ONNX inference). Named threads via `thread::Builder::new().name(...)` for traceability. Use Rayon `par_iter` for data-parallel CPU work within each task unit
- **Serial fan-out/collect for serial I/O channels**: for RFCOMM, UART, or other serial protocols, batch sends followed by deadline-bounded collect. Use `tokio::time::Instant` + `saturating_duration_since` for the remaining-time calculation in the recv loop — correctly handles expired deadlines without panic. `JoinSet` is not applicable since work is sequentially serialized on a single stream
- **`Arc<AtomicBool>` for cooperative cancellation**: appropriate for synchronous thread-based loops where `CancellationToken` would require polling `is_cancelled()` anyway. Lower overhead and no external dependency. Reserve CancellationToken for async contexts where `.cancelled().await` is genuinely useful
- **`thread::scope` for OS-thread structured concurrency**: ensures all spawned threads join before the scope exits. Share a single `Arc<AtomicBool>` cancellation flag. Appropriate for CPU-bound parallel workers where async overhead is undesirable
- **Rayon two-phase write-then-verify**: separate `par_chunks_mut` write and `par_chunks` verify sweeps for cache-bypassed parallel memory access. Rayon's join barrier guarantees sfence visibility before the verify phase begins

### TypeScript

- **`Promise.race` for multi-process lifecycle management**: race child process `.exited` promises as the equivalent of Rust's top-level `select!`. Any process exit triggers cleanup of all others before the supervisor exits with the original code. Suitable for single-container multi-process supervisors (e.g., entrypoint.ts orchestrating Rust + Bun)

### Go

- **Lightweight shutdown tracker**: `sync.WaitGroup` + `sync.Once`-protected close channel. `Add()`/`Done()` bracket in-flight critical operations; `Stop()` signals shutdown via `once.Do(func() { close(stopping) })`; `Wait(timeout)` drains with a bounded timeout using a goroutine + `select`. No per-subsystem interface required — the Go equivalent of the Rust RAII ShutdownTracker pattern
- **Two-phase shutdown**: `Stop()` halts new ticks, then a bounded drain (select on done-channel vs timeout) precedes context cancellation. Avoids a fixed-sleep anti-pattern between phases — prefer signalling the scheduler's internal done-channel or waiting on a WaitGroup with a timeout
- **Independent housekeeping goroutines**: housekeeping tasks (heartbeat, self-metrics emission) that must not be gated by the primary work path run on independent goroutines with their own tickers, racing the same `stopCh` and `ctx.Done()` signals as the main loop. Prevents rate-limiter or API stalls from delaying observability data

### Kotlin

- **Tick-driven state machines for game-loop environments**: when the main loop cannot suspend (Minecraft ticks, UI event loops), use `CompletableFuture.isDone` polling in `tick()` to advance a `State` enum without blocking. See [kotlin-jvm](../languages/kotlin-jvm.md) for the full pattern

## Anti-Patterns

- Fire-and-forget `tokio::spawn` without awaiting or logging errors
- Unbounded channels/queues (memory pressure under load)
- Blocking in async context (use `spawn_blocking` for CPU-bound work)
- Relying on async Drop for cleanup (not yet stable in Rust)

## Open Questions

- Async drop in Rust — when it lands, how it changes cleanup patterns
- Structured concurrency proposals in JavaScript
