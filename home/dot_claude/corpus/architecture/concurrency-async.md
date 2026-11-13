---
name: concurrency-async
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/banner
    path: runtime module (Service trait, ServiceManager, SsrService)
    note: "Service trait + ServiceManager with broadcast/mpsc, nested select! cancellation; the Node SSR child process is itself a Service; TS SingleFlight in the web api client"
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
  - repo: local/rekuma
    path: internal/framework/backoff.go + internal/server/handlers_media.go
    note: "DBBackoff state machine for resilient polling loops; chan-based singleflight with context.WithoutCancel for side effects"
  - repo: local/game-hacking
    path: crates/ac/external/src/daemon.rs
    note: "Raw OS thread force-shutdown watchdog surviving tokio runtime teardown; top-level select! racing signals + cancel token + accept"
  - repo: local/bf4-emblems
    path: ingest writer and pool modules
    note: "Write coalescing through one writer task, N worker futures with a refill queue, per-item claim release, pass-wide abort only for shared-resource errors"
  - repo: local/steamlib
    path: managed transport (Shared::publish, Supervised, Link/Dial traits)
    note: "A pair of watch channels written only through one publish method; reconnect policy tested against scripted fakes under paused tokio time"
  - repo: local/guard
    path: executor (run_captured, tap threads)
    note: "No async runtime: OS threads, std::io::pipe and waitpid for a short-lived startup-budgeted process"
  - repo: local/cargo-sweep-ng
    path: sweep-engine removal module
    note: "Pool sized to keep a filesystem busy (4 to 8 threads), dead worker reported as a failure, platform injected through a start_on constructor"
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
- **Raw OS thread force-shutdown watchdog**: when `tokio`'s runtime `Drop` would block on uncancellable `spawn_blocking` tasks (GLFW event loops, GPU render loops, OpenGL overlays that own their own thread), spawn a raw `std::thread` watchdog *before* awaiting async tasks. The watchdog sleeps to a deadline then calls `std::process::exit(1)`. On entry, reset SIGINT/SIGTERM handlers to `SIG_DFL` so any subsequent Ctrl+C bypasses Rust's signal handling and kills immediately. This avoids a hang on runtime drop while preserving a grace period for clean async shutdown. Pair with `wait()` only on the async task set — not the blocking workers — because the blocking workers can't be cancelled cleanly

- **A child-process supervisor is a `Service`.** It races `child.wait()` against the shutdown broadcast. An unexpected child exit is returned as `Err` so the manager triggers a full shutdown (the platform replaces the container) instead of restarting in place. On shutdown the child gets SIGTERM, a bounded grace period, then SIGKILL, with `kill_on_drop(true)` as the backstop. See [graceful-shutdown](../patterns/graceful-shutdown.md)
- **N worker futures for bounded fan-out over independent items**, not a task per item. Each worker holds one item; a refill future feeds a bounded queue. A failure releases only that item's claim. Reserve a pass-wide abort for errors about the shared resource (a blocked egress), never one item's failure. An earlier drain that aborted on the first `Err` produced six pass aborts against two clean completions
- **Coalesce writes through one writer task.** Producers send completions into a bounded channel; one task flushes on a wall-clock window, so producers never await the database for completions. Mark an item finished only after what it discovered has landed. The select drains ready input before checking the window, and on cancellation the writer closes the channel and drains. Flush errors are logged, not propagated, because leases return the items. See [postgres-operations](./postgres-operations.md)
- **Two channels that must agree are written through one method.** A pair of `watch` channels (state and detail) is updated only inside a single `publish` that calls `send_replace` on both, so they cannot diverge
- **Supervise a reconnecting session through two small traits** (a live link and a dialer), so reconnect policy runs against scripted fakes under paused tokio time. Classify failures as pure functions and test them with rstest tables
- **Size a pool to the device, not the core count.** Filesystem-bound work waits on I/O, so a pool of 4 to 8 threads keeps the disk busy. A dead worker thread is a reported failure, not a hang
- **No async runtime for a short-lived process.** A process with two or three pipe copiers and a `waitpid` is simpler with OS threads and `std::io::pipe`, and an async runtime is startup weight without benefit

### TypeScript

- **`Promise.race` for multi-process lifecycle management**: race child process `.exited` promises as the equivalent of Rust's top-level `select!`. Any process exit triggers cleanup of all others before the supervisor exits with the original code. Suitable for single-container multi-process supervisors orchestrating a Rust binary and a JS server. A compiled backend that owns the child process directly (see the supervisor `Service` above) is the alternative
- **Singleflight with no cache.** Entries are deleted in `.finally` the moment they settle, so only simultaneous callers share work and no answer is ever held. A synchronous throw before the first await never reaches the map. Key coalescers per fetch function (a `WeakMap`), because a SvelteKit load's `fetch` must register its own dependency. Share only bare GETs; anything with a method or body bypasses it, and so does SSR

### Go

- **Independent housekeeping goroutines**: housekeeping tasks (heartbeat, self-metrics emission) that must not be gated by the primary work path run on independent goroutines with their own tickers, racing the same `stopCh` and `ctx.Done()` signals as the main loop. Prevents rate-limiter or API stalls from delaying observability data
- **`DBBackoff` state machine for resilient polling loops**: wraps a `*time.Ticker` to substitute a one-shot `*time.Timer` when the downstream (database, API) goes down. `Wait(ticker) <-chan time.Time` returns the appropriate channel for the `select` arm. `RecordResult(err, ticker)` drives the state machine and calls `onDown`/`onUp` callbacks exactly once per transition. Timer cleanup (`Stop` + drain) on recovery prevents channel leaks. Composes with the independent-housekeeping-goroutines pattern without special cases for outage handling
- **Chan-based singleflight for HTTP handlers**: `map[string]*flight{done: make(chan struct{})}` guarded by a `sync.Mutex`. `acquireOnDemand(key)` returns `(flight, isOwner bool)`; the owner performs the work and calls `completeOnDemand(key, result, err)` which closes `done` and unblocks all waiters. Waiters block on `<-flight.done` then read `flight.result`. Pair with `defer completeOnDemand(key, ..., err)` as a safety net for panics. Simpler than `golang.org/x/sync/singleflight` and more controllable for HTTP handlers that want explicit access to the in-flight state
- **`context.WithoutCancel` for side effects that must outlive client disconnect**: when an HTTP handler performs side effects (object storage uploads, DB writes) that must complete even if the client disconnects, derive `sideEffectCtx := context.WithoutCancel(ctx)` and pass that to the side-effect call. The original `ctx` still cancels when the HTTP request ends; the derived context carries the request's values (request ID, logger) without the cancellation. Available since Go 1.21 — previously required a manual `detachedCtx` struct
- See [graceful-shutdown](../patterns/graceful-shutdown.md) for Go shutdown tracker, two-phase shutdown, and bounded drain patterns

### Kotlin

- **Tick-driven state machines for game-loop environments**: when the main loop cannot suspend (Minecraft ticks, UI event loops), use `CompletableFuture.isDone` polling in `tick()` to advance a `State` enum without blocking. See [kotlin-jvm](../languages/kotlin-jvm.md) for the full pattern

## Anti-Patterns

- Fire-and-forget `tokio::spawn` without awaiting or logging errors
- Unbounded channels/queues (memory pressure under load)
- Blocking in async context (use `spawn_blocking` for CPU-bound work)
- Relying on async Drop for cleanup (not yet stable in Rust)

## Related Topics

- [graceful-shutdown](../patterns/graceful-shutdown.md) — Signal handling, bounded drain, Go two-phase shutdown
- [rust](../languages/rust.md) — Rust-specific async/sync patterns and thread pool conventions

## Open Questions

- Async drop in Rust — when it lands, how it changes cleanup patterns
- Structured concurrency proposals in JavaScript
