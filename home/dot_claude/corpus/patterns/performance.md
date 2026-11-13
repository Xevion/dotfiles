---
name: performance
category: patterns
last_audited: 2026-11-12
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
- **SIMD dispatch: runtime or compile time is a trade-off.** Runtime dispatch gates SIMD implementations behind `is_x86_feature_detected!` and annotates them with `#[target_feature(enable = "avx512f")]`, keeping one binary portable while taking the fast path on capable hardware, with a scalar fallback. When results must be bit-identical across machines (a seed that must render the same everywhere), probe no CPU at run time: select the vector path by `cfg(target_feature)` at compile time and keep the scalar walk compiled as the reference. Use only operations with per-lane IEEE semantics, forbid FMA contraction and reassociated sums, and do not enable relaxed-SIMD. Which baseline to compile for (`x86-64-v2`, `v3`, `native`) is a per-project distribution decision; see [rust](../languages/rust.md)
- **Dedicated Cargo profile for mutation testing**: a `[profile.mutant]` inheriting from `dev` with `incremental = false`, `debug = 0`, `codegen-units = 256` optimizes for cargo-mutants' many-sequential-clean-builds pattern. `opt-level = 3` on dependencies + `opt-level = 1` on local crates balances compile speed with test runtime. Combined with `mutants.toml` for per-crate scope

- **Budget the whole task, not each attempt.** A per-search limit cannot bound a scenario that replans, so cap the cumulative wall-clock and account for discarded attempts in telemetry. Build expensive structures only after the cheap path has refused. Tune stall rules offline to the zero-false-kill point, with a harsher live preset and a raw expansion cap as the backstop
- **A measured stack for performance work** (borde.rs): per-package dev `opt-level`, jemalloc on Linux and mimalloc elsewhere with allocator stats reported, rayon row jobs batched so scratch rows are reused, per-key map-cache locking, per-turn search budgets, per-system timing that names the slowest system, and a frame budget derived from the monitor refresh rate
- **For a binary launched on every tool call, count spawns as the real cost** (guard): replacing a `git rev-parse` subprocess with an in-process `.git` walk took the path from 3.5 ms to 1.9 ms with a quarter of the variance. Binary size is cosmetic when pages are demand-loaded
- **Verify a GPU execution provider took effect.** ONNX Runtime can fall back to CPU silently when a provider fails to register. Make the provider fail hard at session build, then time the warmup run against a threshold and bail when the timing implies CPU
- **Measure throughput limits and write them down.** Bench with patched frame templates, tune transfer sizes and in-flight counts, and record measured link rates and ceilings in docs instead of assuming them

### TypeScript

- **Asymmetric EMA for adaptive latency estimation**: use different EMA alphas for improving vs worsening latency. A faster downward alpha (e.g., 0.5) prevents over-debouncing when the model warms up; a slower upward alpha (e.g., 0.1) avoids over-reacting to transient spikes. Derive debounce floors as a fraction of the EMA, clamped to sane bounds
- **Promise-chain mutex for sequential inference**: when the underlying API (WebGPU, WASM) cannot handle concurrent calls, chain all async calls through a promise queue (`workerBusy = task.then(...)`) rather than a lock primitive. Prevents stalls from overlapping inference without requiring explicit lock/unlock
- **Cached results for interactive re-processing**: cache raw output (logits, masks, scores) from expensive inference calls. Post-processing functions (re-threshold, smoothing) operate on cached results without re-running inference, enabling sub-millisecond interactive parameter adjustment

### Go

- pprof profiling, `testing.B` benchmarks, `sync.Pool` for allocation reduction — to be populated from future project audits

## Anti-Patterns

- Optimizing without measuring
- Premature caching
- N+1 queries
- Unbounded memory growth

## Open Questions

- Flamegraph-driven optimization workflow and tooling
