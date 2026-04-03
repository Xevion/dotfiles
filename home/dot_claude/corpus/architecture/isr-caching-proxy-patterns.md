---
name: isr-caching-proxy-patterns
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/xevion.dev
    path: src/cache.rs + src/proxy.rs
    note: "DashSet singleflight, lazy per-encoding compression with RwLock, session-aware bypass, URL normalization"
---

# ISR & Caching Proxy Patterns

## Philosophy

Cache in front of slow renderers. Stale-while-revalidate for responsiveness — serve the cached version immediately and refresh in the background. Singleflight for thundering herd prevention. Never cache authenticated responses.

## Conventions

- **Stale-while-revalidate with fresh/stale windows**: each cached entry has a `fresh_until` and `stale_until` timestamp. Within the fresh window, serve directly. Within the stale window, serve the stale version and trigger a background refresh. Past the stale window, block on a fresh render

- **Lazy per-encoding compression**: store compressed variants in a `parking_lot::RwLock<HashMap<ContentEncoding, Bytes>>` per cache entry. Each encoding (zstd, brotli, gzip) is computed once on first request for that encoding. Skip compression if the compressed size exceeds the original — some responses (small HTML, already-compressed images) don't benefit. Read-lock for cache hits, upgrade to write-lock only on miss

- **Singleflight via DashSet**: track in-flight background refresh paths in a `DashSet<String>`. `start_refresh()` returns false if the path is already being refreshed. This prevents thundering herd on popular pages without the complexity of moka's entry-level coalescing. Remove the path from the set after refresh completes (success or failure)

- **Session-aware cache bypass**: authenticated requests skip ISR caching entirely (`use_cache = !is_authenticated`). Strip incoming `X-Session-User` headers before the proxy and re-inject them only after server-side session validation. This prevents both cache poisoning (serving authenticated content to anonymous users) and session spoofing (client-provided session headers)

- **URL normalization before cache lookup**: normalize trailing slashes (permanent redirect), `.html` extensions, and query string ordering before computing the cache key. Prevents duplicate cache entries for semantically equivalent URLs

## Language-Specific

### Rust

- `moka` or `DashMap` for the primary cache store with configurable max entries and TTL
- `parking_lot::RwLock` for per-entry compression caches (lower overhead than `tokio::sync::RwLock` for short critical sections)
- `DashSet` for singleflight tracking (simpler than a full singleflight crate for background-refresh dedup)
- `tower` middleware layer for cache-hit/miss routing and header injection

## Anti-Patterns

- **Compressing on every response**: recomputing brotli/gzip per-request wastes CPU. Cache the compressed variants alongside the raw response
- **No singleflight**: without dedup, a cache miss on a popular page triggers N concurrent renders (one per request during the refresh window)
- **Caching authenticated responses**: mixing authenticated and anonymous content in the same cache key serves private data to public users
- **Cache key without query string**: query parameters often change the rendered content. Include them in the key, but normalize ordering to avoid duplicates

## Open Questions

- Edge caching (Cloudflare, CDN) vs application-level ISR caching trade-offs
- CDN invalidation strategies for ISR content
- Cache warming strategies for cold starts (pre-render popular pages on deploy)
