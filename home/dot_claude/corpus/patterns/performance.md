---
name: performance
category: patterns
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/xevion.dev
    path: src/cache.rs
    note: "ISR cache with lazy per-encoding compression, stale-while-revalidate, singleflight via moka"
  - repo: Xevion/ferrite
    path: src/simd.rs + src/pattern.rs
    note: "Runtime AVX-512 dispatch with #[target_feature], Rayon two-phase write-verify"
  - repo: Xevion/WebSAM
    path: src/lib/inference/
    note: "Asymmetric EMA latency, Comlink serialize() mutex, cached logits for interactive re-threshold"
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
- **Dedicated Cargo profile for mutation testing**: a `[profile.mutant]` inheriting from `dev` with `incremental = false`, `debug = 0`, `codegen-units = 256` optimizes for cargo-mutants' many-sequential-clean-builds pattern. `opt-level = 3` on dependencies + `opt-level = 1` on local crates balances compile speed with test runtime. Combined with `mutants.toml` for per-crate scope

### TypeScript

- **Asymmetric EMA for adaptive latency estimation**: use different EMA alphas for improving vs worsening latency. A faster downward alpha (e.g., 0.5) prevents over-debouncing when the model warms up; a slower upward alpha (e.g., 0.1) avoids over-reacting to transient spikes. Derive debounce floors as a fraction of the EMA, clamped to sane bounds
- **Promise-chain mutex for sequential inference**: when the underlying API (WebGPU, WASM) cannot handle concurrent calls, chain all async calls through a promise queue (`workerBusy = task.then(...)`) rather than a lock primitive. Prevents stalls from overlapping inference without requiring explicit lock/unlock
- **Cached results for interactive re-processing**: cache raw output (logits, masks, scores) from expensive inference calls. Post-processing functions (re-threshold, smoothing) operate on cached results without re-running inference, enabling sub-millisecond interactive parameter adjustment

### Go
<!-- pprof for profiling, benchmarks with testing.B, sync.Pool for allocation reduction -->

## Anti-Patterns

- Optimizing without measuring
- Premature caching
- N+1 queries
- Unbounded memory growth

## Open Questions
