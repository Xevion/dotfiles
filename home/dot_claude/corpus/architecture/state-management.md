---
name: state-management
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/glint
    path: frontend/src/lib/stores/ + components/data-view/
    note: "Connectivity singleton, cursor-paginated list factory, module-scoped $state patterns"
  - repo: local/inkwell
    path: web/src/lib/stores/
    note: "Cursor-paginated list with ETag sync, $effect.root debounced server sync, version-counter reactivity"
  - repo: Xevion/xevion.dev
    path: web/src/lib/stores/
    note: Module-scoped auth singleton, ThemeStore with SSR guard
---

# State Management

## Philosophy

Reactivity through signals/runes, not manual subscription. Clear boundary between server state (fetched, cached, invalidated) and client state (UI, preferences, connectivity). Minimal global state — prefer page-scoped or tree-scoped state over app-wide singletons.

## Conventions

- **Server state via query libraries**: TanStack Query, urql, or SvelteKit load functions for data that originates from the server. The library handles caching, deduplication, and invalidation
- **Client state via signals/stores**: local reactive primitives ($state in Svelte, signals in Solid) for UI state that doesn't persist to the server
- **Derived over synced**: prefer `$derived` / computed values over manually keeping two pieces of state in sync
- **Page-scoped factory for server state**: for data-heavy pages (paginated lists, filtered tables), a factory function returning a `$state` object is preferable to `createContext()` when the state is local to one route. The factory encapsulates query logic, URL sync, and pagination without context boilerplate

```svelte
<!-- Pattern: page-scoped server state factory -->
<script lang="ts">
  const list = createCursorList({
    query: BrowseShadersQuery,
    extract: (d) => d.shaders,
    pageSize: 24,
    search: { debounce: 300 },
    sort: { options: [...], default: 'popular' },
    syncUrl: true
  });
</script>
```

- **Tree-scoped via createContext**: for state shared across a component subtree (filters, selected item, UI mode). Avoid for single-consumer state where a factory is simpler
- **Module-scoped class singleton**: for auth/session state that spans multiple pages, use a class with `$state` fields and async methods (login, logout, checkSession) exported as a module-level instance. Distinguish from page-scoped factories by lifecycle: singleton is created once per module load, factory per route mount
- **Cursor-paginated list store with background sync**: a class-based `$state` store combining pagination state, ETag tracking, visibility-triggered sync, and sessionStorage snapshot is preferable to a generic factory when the state needs lifecycle methods (`initialize`, `prepend`, `destroy`) and cross-tab reconciliation
- **`$effect.root` for class-based store effects**: use `$effect.root(() => { ... })` in a class constructor to register reactive effects (e.g., debounced PUT sync) outside the component tree. Store the returned cleanup function and call from `destroy()`. This is the correct pattern for stores that react to their own `$state` changes

- **Stale-request-ID concurrency control**: for async operations dispatched to Web Workers or external services, track a module-level integer counter. Each async call captures the ID at launch and compares against the current counter on resolution — stale results are discarded. This is the "optimistic concurrency via version counter" pattern applied to async calls rather than UI renders
- **Single-inflight + drain-latest for pointer tracking**: maintain one in-flight slot and one pending-latest slot. If a call is in flight, store only the most recent input; on completion, drain the pending slot if populated. Discards all intermediate inputs — "latest-wins" with zero backlog. Suitable for hover-decode, mouse-move inference, and similar pointer-tracking workloads

## Anti-Patterns

- Global state for everything — most state is page-scoped or tree-scoped
- Prop drilling 5+ levels deep — use context or a factory instead
- Syncing server and client state manually instead of using a query library
- Using `$effect` to derive values that `$derived` can express directly

### React

- **Custom hook as page-scoped state factory**: a custom hook (e.g., `useLookup`) that encapsulates co-located state, derived values (debounce, type detection), side effects, and async actions is the React equivalent of the Svelte `createCursorList()` factory. Returns a stable interface to the page component, keeping pages thin and making state logic independently testable. Use `useRef` for values that shouldn't trigger re-renders (e.g., repeatable URLs), `useDebouncedValue` for reactive throttling

## Open Questions

- Server-only vs shared state boundaries in SvelteKit
- Optimistic updates and rollback patterns
