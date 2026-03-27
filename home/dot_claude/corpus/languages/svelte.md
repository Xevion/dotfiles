---
name: svelte
category: languages
last_audited: 2026-03-26
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

- Mixing Svelte 4 stores with rune-based components
- Using `$effect` for derived state that `$derived` can express
- Global mutable singletons instead of context-scoped state

## Open Questions

- Svelte 5 snippet composition patterns
- Server-only vs shared state boundaries in SvelteKit
