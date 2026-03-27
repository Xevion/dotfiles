---
name: performance
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/xevion.dev
    path: src/cache.rs
    note: "ISR cache with lazy per-encoding compression, stale-while-revalidate, singleflight via moka"
  - repo: Xevion/ferrite
    path: src/simd.rs + src/pattern.rs
    note: "Runtime AVX-512 dispatch with #[target_feature], Rayon two-phase write-verify"
---

# Performance

## Philosophy

Measure first. Profile-guided optimization. Avoid premature optimization. Budget-based thinking.

## Conventions

- Benchmarks before optimizing, lazy loading by default, caching with explicit invalidation, connection pooling

## Language-Specific

### Rust

- **Lazy per-encoding compression on cached responses**: use an `RwLock<HashMap<Encoding, Bytes>>` per entry. Compute each encoding (zstd/brotli/gzip) once on first request, skip if compressed size exceeds the original. Pair with stale-while-revalidate for SSR: serve stale immediately, trigger background refresh only once per path using a `DashSet` of in-flight refreshes to prevent thundering herd
- **Runtime SIMD dispatch with `#[target_feature]`**: gate SIMD implementations behind `is_x86_feature_detected!` at the call site and annotate the implementation with `#[target_feature(enable = "avx512f")]`. Keeps the binary portable while taking the SIMD fast path on capable hardware. Pair with a scalar fallback for unsupported architectures

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
