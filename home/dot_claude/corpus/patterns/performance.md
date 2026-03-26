---
name: performance
category: patterns
last_audited: 2026-03-26
exemplars: []
---

# Performance

## Philosophy

<!-- Measure first, profile-guided optimization, avoid premature optimization, budget-based thinking -->

## Conventions

<!-- Benchmarks before optimizing, lazy loading by default, caching with explicit invalidation, connection pooling -->

## Language-Specific

### Rust
<!-- criterion for benchmarks, #[inline] for hot paths, avoid unnecessary allocations, flamegraph profiling -->

### TypeScript
<!-- Bundle analysis, code splitting, React.memo/useMemo with purpose, lighthouse budgets -->

### Go
<!-- pprof for profiling, benchmarks with testing.B, sync.Pool for allocation reduction -->

## Anti-Patterns

<!-- Optimizing without measuring, premature caching, N+1 queries, unbounded memory growth -->

## Open Questions
