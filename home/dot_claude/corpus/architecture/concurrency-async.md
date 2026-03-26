---
name: concurrency-async
category: architecture
last_audited: 2026-03-26
exemplars: []
---

# Concurrency & Async

## Philosophy

<!-- Structured concurrency, explicit cancellation, backpressure by default, avoid shared mutable state -->

## Conventions

<!-- Cancellation tokens/contexts, bounded channels, task spawning discipline, error propagation across boundaries -->

## Language-Specific

### Rust
<!-- Tokio runtime, select! for racing, JoinSet for structured spawning, async trait patterns -->

### TypeScript
<!-- Promise.all for parallel, AbortController for cancellation, worker threads for CPU-bound -->

### Go
<!-- Goroutines + channels, context.Context for cancellation, errgroup for structured concurrency -->

### Kotlin
<!-- Structured concurrency via CoroutineScope, Flow for reactive streams, SupervisorJob for isolation -->

## Anti-Patterns

<!-- Fire-and-forget without error handling, unbounded queues, blocking in async context -->

## Open Questions

<!-- Async drop in Rust, structured concurrency proposals in JS -->
