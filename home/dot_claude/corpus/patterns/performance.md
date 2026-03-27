---
name: performance
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/xevion.dev
    path: src/cache.rs
    note: "ISR cache with lazy per-encoding compression, stale-while-revalidate, singleflight via moka"
---

# Performance

## Philosophy

Measure first. Profile-guided optimization. Avoid premature optimization. Budget-based thinking.

## Conventions

- Benchmarks before optimizing, lazy loading by default, caching with explicit invalidation, connection pooling

## Language-Specific

### Rust

- **Lazy per-encoding compression on cached responses**: use an `RwLock<HashMap<Encoding, Bytes>>` per entry. Compute each encoding (zstd/brotli/gzip) once on first request, skip if compressed size exceeds the original. Pair with stale-while-revalidate for SSR: serve stale immediately, trigger background refresh only once per path using a `DashSet` of in-flight refreshes to prevent thundering herd

### TypeScript
<!-- Bundle analysis, code splitting, lighthouse budgets -->

### Go
<!-- pprof for profiling, benchmarks with testing.B, sync.Pool for allocation reduction -->

## Anti-Patterns

- Optimizing without measuring
- Premature caching
- N+1 queries
- Unbounded memory growth

## Open Questions
