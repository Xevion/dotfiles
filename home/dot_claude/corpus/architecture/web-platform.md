---
name: web-platform
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/xevion.dev
    path: web/src/lib/stores/theme.svelte.ts + web/src/hooks.server.ts
    note: "window.matchMedia for theme detection, transformPageChunk for production HTML minification"
  - repo: Xevion/WebSAM
    path: src/lib/stores/ + src/lib/inference/
    note: "OPFS binary caching, Web Worker via Comlink, breakpoint matchMedia store, custom window events"
---

# Web Platform

## Philosophy

Prefer native web platform APIs over library abstractions when browser support is sufficient. Progressive enhancement — features degrade gracefully when APIs are unavailable.

## Conventions

- **`window.matchMedia` for system theme detection**: use the native API rather than a library wrapper, falling back to localStorage persistence. Combine with a class toggle on `document.documentElement` rather than CSS variable injection. Skip any library when these three primitives suffice
- **SvelteKit `transformPageChunk` for production-only transforms**: use the hook as an integration point for HTML minification, injection, or other per-response transforms. Gate on `!dev` to preserve readable markup during development. Prefer over separate build steps when the transform is fast and per-response

- **`URLSearchParams` + shallow routing for URL-synced state**: use native `URLSearchParams` API with framework shallow routing (`router.push(queryString, undefined, { shallow: true })` in Next.js) for bidirectional URL state sync. Serialize on successful action, deserialize + auto-execute on mount when params are present. No URL state library needed for simple key-value query params

- **OPFS for binary blob persistence**: use `navigator.storage.getDirectory()` (Origin Private File System) to store large binary blobs (model weights, images) in-browser across page reloads without a server. Pair with idb-keyval for structured metadata — two-tier persistence (IndexedDB for metadata, OPFS for binary blobs). OPFS survives page reloads and has no size limits like localStorage
- **Web Worker + Comlink for CPU/GPU offloading**: offload CPU-intensive or WebGPU inference work to a dedicated Web Worker. Use Comlink for typed RPC bridging — `Comlink.expose(api)` in the worker, `Comlink.wrap<typeof api>()` in the main thread. Add a promise-chain mutex (`serialize()`) when the underlying API (e.g., WebGPU session) cannot handle concurrent calls, since Comlink does not serialize concurrent async invocations
- **Custom window events for cross-component communication**: dispatch custom events on `window` (e.g., `'app:load-file'`) carrying typed payloads in `detail` to communicate between unrelated components without prop drilling or shared state. The listener co-locates handling logic with the consuming component
- **Breakpoint reactive store via matchMedia**: track responsive breakpoints (isMobile, isDesktop) as reactive `$state` values driven by `window.matchMedia` listeners with SSR guards. Components read breakpoint state directly for conditional rendering (drawer vs sidebar, bottom sheet vs panel)
- **Worker-side preprocessing before inference**: perform all data transformation (resize, normalize, format conversion) inside the Web Worker rather than the main thread. The main thread passes only raw pixel data; tensor manipulation happens worker-side. Keeps the UI thread free during expensive preprocessing phases

### Content Security Policy

Start SvelteKit projects with CSP in `reportOnly` mode — observe violations without breaking functionality. Standard directives:
- `script-src: 'self'` + analytics domains
- `img-src: 'self', 'data:'` + CDN domains (Cloudflare R2, Discord CDN, etc.)
- `connect-src`: include `ws://localhost:*` in dev for HMR WebSocket
- `report-uri: '/api/csp-report'` for server-side violation collection

Tighten from `reportOnly` to enforcing once the violation log is clean. Configure in `svelte.config.js` via `kit.csp`.

## Anti-Patterns

- Polyfilling APIs that have >95% browser support
- Using JS libraries for what CSS or HTML can do natively
- Ignoring platform APIs because "we've always used library X"

## Open Questions
