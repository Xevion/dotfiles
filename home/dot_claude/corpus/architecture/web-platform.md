---
name: web-platform
category: architecture
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/xevion.dev
    path: web/src/lib/stores/theme.svelte.ts + web/src/hooks.server.ts
    note: "window.matchMedia for theme detection, transformPageChunk for production HTML minification"
---

# Web Platform

## Philosophy

Prefer native web platform APIs over library abstractions when browser support is sufficient. Progressive enhancement — features degrade gracefully when APIs are unavailable.

## Conventions

- **`window.matchMedia` for system theme detection**: use the native API rather than a library wrapper, falling back to localStorage persistence. Combine with a class toggle on `document.documentElement` rather than CSS variable injection. Skip any library when these three primitives suffice
- **SvelteKit `transformPageChunk` for production-only transforms**: use the hook as an integration point for HTML minification, injection, or other per-response transforms. Gate on `!dev` to preserve readable markup during development. Prefer over separate build steps when the transform is fast and per-response

- **`URLSearchParams` + shallow routing for URL-synced state**: use native `URLSearchParams` API with framework shallow routing (`router.push(queryString, undefined, { shallow: true })` in Next.js) for bidirectional URL state sync. Serialize on successful action, deserialize + auto-execute on mount when params are present. No URL state library needed for simple key-value query params

## Anti-Patterns

- Polyfilling APIs that have >95% browser support
- Using JS libraries for what CSS or HTML can do natively
- Ignoring platform APIs because "we've always used library X"

## Open Questions
