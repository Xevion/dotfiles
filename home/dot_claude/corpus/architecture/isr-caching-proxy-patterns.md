---
name: isr-caching-proxy-patterns
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/xevion.dev
    path: src/cache.rs
    note: "Stale-while-revalidate ISR, lazy multi-encoding compression, singleflight via moka, session-aware bypass"
---

# ISR & Caching Proxy Patterns

## Philosophy

<!-- Cache in front of slow renderers. Stale-while-revalidate for responsiveness. Singleflight for thundering herd prevention. -->

## Conventions

<!-- Stale-while-revalidate with fresh/stale windows, lazy per-encoding compression (brotli/gzip/zstd), singleflight coalescing, URL normalization, session-aware cache bypass -->

## Language-Specific

### Rust

<!-- moka for singleflight, parking_lot::RwLock per-entry compression cache, DashSet for in-flight refresh tracking -->

## Anti-Patterns

<!-- Compressing on every response instead of caching compressed variants, no singleflight causing thundering herd, caching authenticated responses -->

## Open Questions

<!-- Edge caching vs application-level caching, CDN invalidation strategies, cache key design -->
