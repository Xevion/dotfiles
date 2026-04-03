---
name: svelte
category: languages
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/banner
    path: web/src/lib/
    note: Svelte 5 runes-only codebase with createContext for shared state, bits-ui components
  - repo: Xevion/instant-upscale
    path: frontend/src/lib/stores/
    note: Factory-singleton pattern with $state closures and getter properties for theme/session
  - repo: Xevion/doujin-ocr-summary
    path: web/src/lib/stores/
    note: SSE-driven operation stream factory with reactive getters and explicit close()
  - repo: Xevion/glint
    path: frontend/src/lib/stores/
    note: "Connectivity singleton with SSR guard, cursor-paginated list factory"
  - repo: local/inkwell
    path: web/src/lib/stores/
    note: "Version-counter reactivity, discriminated union $state phases, $effect.root class stores, cursor-paginated ETag sync"
  - repo: Xevion/xevion.dev
    path: web/src/lib/stores/
    note: ThemeStore factory-singleton with SSR guard, onNavigate + View Transitions
  - repo: Xevion/WebSAM
    path: src/lib/stores/ + src/lib/inference/
    note: "RAF-batched canvas render, runed FSM pipeline, stale-request-ID pattern, module-scoped $state for SPA"
---

# Svelte

## Philosophy

Svelte 5 runes exclusively — no legacy stores. Reactive state is explicit via `$state`, derived values via `$derived`, side effects via `$effect`. Shared state uses `createContext()` pairs, not global stores.

## Conventions

- **Runes only**: `$state`, `$derived`, `$effect`, `$props` — never `writable()`, `readable()`, or `$store` syntax
- **Shared state**: wrap reactive objects in a class or plain object with `$state` fields, expose via `createContext()` from `*.svelte.ts` files
- **Module-scoped reactive state (factory-singleton)**: for app-wide state (theme, session), use a factory function returning a plain object with `$state` closures and `get` accessor properties. Reserve `createContext()` for tree-scoped state. Factory must live in a `.svelte.ts` file (required for rune usage outside components)
- **SSE/WebSocket subscription state**: encapsulate in a `.svelte.ts` factory returning reactive getters plus a `close()` method. Caller handles cleanup via `$effect`
- **Browser environment singletons**: for app-wide browser state (online/offline, backend reachability), use module-scoped `$state` with a plain-object getter facade and mutation methods. SSR guard with `browser ? navigator.onLine : true` as the idiomatic initial value pattern
- **Version-counter reactivity for large collections**: for large Maps/Sets that are frequently mutated, keep the collection in a plain (non-reactive) field and bump a `$state` integer on writes. Readers call `void this._version` to create a reactive dependency. Avoids Svelte proxying every Map entry while enabling fine-grained reactivity
- **Discriminated union `$state` for multi-phase async flows**: model phases (idle, generating, uploading, complete, error) as a discriminated union on a `phase` string-literal field stored in `$state`. Each phase carries only the fields valid in that phase. Phase narrowing (`this.state.phase === 'x'`) gives TypeScript access to phase-specific properties
- **`$effect.root` for class-based store effects**: use `$effect.root(() => { ... })` in a class constructor to register reactive effects outside the component tree. Store the returned cleanup function and call it from `destroy()`. This is the correct pattern for class stores that need to react to their own `$state` changes (e.g., debounced server sync)
- **`onNavigate` + View Transitions**: use `onNavigate` with `document.startViewTransition` for SvelteKit page transitions. Assign `view-transition-name` to persistent shell elements (nav, theme toggle) to exclude them from transitions. For progressive enhancement, prefer declaration merging (`interface Document { startViewTransition?: (cb: () => void) => void }`) over `document as any` — preserves type safety on the callback
- **Module-scoped `$state` objects for single-route SPAs**: for true single-route SPAs where tree-scoped isolation is unnecessary, bare module-scoped `$state({...})` objects are a valid alternative to factory-singleton functions. Simpler and less ceremony. Use factory-singletons for multi-route apps where testability and tree isolation matter; use bare `$state` objects when the app has a single route and no need for multiple instances
- **RAF-batched canvas render via `$effect`**: for canvas-heavy UIs, use a "tracking `$effect`" that reads all reactive dependencies then calls `markDirty()` (which schedules a `requestAnimationFrame` render) rather than rendering directly inside `$effect`. This decouples Svelte's reactivity tracking from actual GPU/canvas work and avoids scheduling multiple concurrent rAF callbacks. Continuous rAF animation loops are started/stopped by separate `$effect`s gated on reactive conditions
- **FSM pipeline orchestration with `runed` FiniteStateMachine**: for multi-phase async pipelines (download→model-ready→encoding→ready→decoding), use `runed`'s `FiniteStateMachine` as the primary state machine. `$derived` reads `pipeline.current` to compute boolean gates. `_enter` lifecycle hooks trigger async side effects on state transitions. Distinct from discriminated-union `$state` phases — externalizes transition logic to a library
- **Stale-request-ID for async concurrency control**: track a module-level integer counter. Each async call captures the ID at launch and checks against the current counter on resolution — if they differ, the result is discarded as stale. Useful for async Worker calls where multiple decode requests can overlap
- **Single-inflight + drain-latest pattern**: for pointer-tracking workloads (hover decode, mouse-move inference), maintain a single in-flight slot plus a pending-latest slot. If a call is in flight, store only the latest coordinates; on completion, drain the pending slot. Discards all intermediates — "latest-wins" with no backlog
- **Debounced `$effect` pattern**: when reactive inputs must trigger a debounced side effect (not a derived value), use `$effect` with an outer timeout ref, list dependencies explicitly on separate lines, and return cleanup that clears the pending timeout. This is distinct from the anti-pattern of using `$effect` for pure derived state
- **Browser API wiring via `$effect`**: for imperative browser APIs (IntersectionObserver, ResizeObserver) tied to reactive state, use `$effect` with a null-guard on the bound element ref and a boolean reactive gate. Return cleanup that calls the API's teardown method and clears any associated timers
- **Theme flash prevention via `{@html}` inline script**: for static SvelteKit sites without SSR, inject an IIFE in `<svelte:head>` via `{@html}` that reads localStorage and toggles the dark class on `documentElement` before hydration. This is the client-only complement to `transformPageChunk`. Note: bypasses CSP nonces and runs on every SPA navigation
- **Component composition**: prefer bits-ui headless primitives, compose with Tailwind utility classes
- **Type safety**: import types from auto-generated bindings (`$lib/bindings`), never hand-maintain TypeScript interfaces that mirror backend types

```svelte
<!-- Shared state pattern -->
<script lang="ts">
  import { getFiltersContext } from '$lib/stores/filters.svelte';
  const filters = getFiltersContext();
</script>
```

## Anti-Patterns

- **Mixing Svelte 4 stores with rune-based components**: projects on Svelte 5 that retain `writable()`/`derived()` stores alongside `$state` runes in components are in a partial migration state. Module-scoped `$state` variables are the idiomatic replacement. The `$store` subscription syntax mixed with `$derived` in the same component is the primary Svelte 5 anti-pattern
- Using `$effect` for derived state that `$derived` can express
- Global mutable singletons instead of context-scoped state (exception: bare module-scoped `$state` is acceptable for single-route SPAs — see Conventions)

## Open Questions

- Svelte 5 snippet composition patterns
- Server-only vs shared state boundaries in SvelteKit
