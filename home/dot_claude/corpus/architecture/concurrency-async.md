---
name: concurrency-async
category: architecture
last_audited: 2026-03-26
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

### TypeScript

<!-- Placeholder: Promise.all, AbortController, worker threads -->

### Go

<!-- Placeholder: goroutines + channels, context.Context, errgroup -->

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
