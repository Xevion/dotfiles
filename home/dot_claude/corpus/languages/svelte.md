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
  - repo: local/inkwell
    path: web/src/lib/stores/
    note: "Version-counter reactivity, discriminated union $state phases, $effect.root class stores, cursor-paginated ETag sync"
  - repo: Xevion/xevion.dev
    path: web/src/lib/stores/
    note: ThemeStore factory-singleton with SSR guard, onNavigate + View Transitions
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
- **`onNavigate` + View Transitions**: use `onNavigate` with `document.startViewTransition` for SvelteKit page transitions. Assign `view-transition-name` to persistent shell elements (nav, theme toggle) to exclude them from transitions
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
- Global mutable singletons instead of context-scoped state

## Open Questions

- Svelte 5 snippet composition patterns
- Server-only vs shared state boundaries in SvelteKit
