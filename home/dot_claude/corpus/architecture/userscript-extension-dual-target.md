---
name: userscript-extension-dual-target
category: architecture
last_audited: 2026-04-10
exemplars:
  - repo: local/ts-chan
    path: src/core/ + src/entries/
    note: "Vite-plugin-monkey userscript + WXT MV3 extension from one src/ tree, runtime platform detection, unified StorageBackend"
  - repo: local/Applyhelm
    path: packages/extension/
    note: "WXT Svelte 5 extension with backend-dependent auth, bearer token in browser.storage.local"
---

# Userscript + Extension Dual-Target

## Philosophy

Browser scripts need both distribution channels: **userscripts** (Tampermonkey, Violentmonkey, Greasemonkey — install without developer account, auto-update from a URL) reach power users who don't want another extension, while **MV3 extensions** (Chrome Web Store, Firefox AMO) reach everyone else and get proper service-worker lifecycle, declarative permissions, and store discoverability. A single TypeScript codebase should produce both artifacts without a fork — divergence compounds fast when features ship once and backfill to the other target later.

## Conventions

- **Shared source under `src/`, divergent entry points under `src/entries/`** — everything except the bootstrap sequence lives in shared modules (`core/`, `services/`, `stores/`, `ui/`). The userscript entry (`src/entries/userscript.ts`) is an IIFE with direct `GM_*` grants; the extension entries (`src/entries/extension/content.ts`, `background.ts`) follow the WXT `defineContentScript`/`defineBackground` convention. The two init sequences should be *identical* — extract to a shared `initApp(platform)` function if drift appears
- **Build tools are per-target, not per-file** — Vite + `vite-plugin-monkey` for the userscript, WXT for the extension. Both consume the same `src/` tree via separate entry points. Keep `emitCss: false` on the userscript build so CSS inlines into the IIFE and the distributed `.user.js` is one file
- **Runtime platform detection, not build-time constants** — the same `.user.js` artifact may run under Tampermonkey, Violentmonkey, or a future compatible manager. `typeof GM_getValue !== "undefined"` and `typeof chrome?.storage?.sync !== "undefined"` are the correct discriminants, evaluated once at module init and cached. Build-time `__USERSCRIPT__` constants are valid for the extension entry (where WXT knows the target) but not inside shared code
- **`StorageBackend` interface over raw storage APIs** — define a `StorageBackend` interface with `get/set/delete/keys` and an optional `onChange(key, callback(newValue, remote))`. Back it with three implementations: GM (`GM_setValue`/`GM_getValue`/`GM_addValueChangeListener`), chrome (`chrome.storage.local/sync` + `onChanged` event), and localStorage (degraded fallback). Callers depend only on the interface
- **Permissions are declared once, enforced twice** — the userscript `@grant` list (`vite-plugin-monkey` metadata block) and the extension `manifest.json` permissions array must stay in sync. Keep them in one place (e.g., a `const PERMISSIONS` array imported by both build configs) to prevent permission drift between targets

## Cross-Tab Sync

The two environments expose fundamentally different cross-tab APIs:

| Surface | Userscript | Extension |
|---|---|---|
| Storage change notification | `GM_addValueChangeListener(key, cb)` — fires in all tabs on any write | `chrome.storage.onChanged.addListener(cb)` — fires in all tabs + background |
| Direct message passing | None native; use `BroadcastChannel` polyfill | `chrome.runtime.onMessage` between content script and background |
| Shared memory | None | `chrome.storage.session` for tab-session scope |

Unify behind `StorageBackend.onChange(key, callback)`. The `remote` boolean in the callback distinguishes local-tab writes from cross-tab writes — both GM and chrome.storage.onChanged expose this, and consumers can filter to avoid echo loops. When `onChange` is unavailable (localStorage backend), callers must degrade gracefully or poll.

## Request Routing

Cross-origin requests differ sharply:

- **Userscript**: `GM_xmlhttpRequest` bypasses CORS entirely. Grants must declare each origin in `@connect`. Prefer over `fetch` for any cross-origin call
- **Extension**: `fetch` from the content script is subject to page CORS. Cross-origin requests must go through the background service worker via `chrome.runtime.sendMessage` and hit `host_permissions` origins
- **Abstraction**: a `getRequestFn()` factory returns the correct implementation based on the cached platform detection. Call sites see `request(url, opts)` only

## Shared UI

Both targets inject UI into third-party pages, so both face the Shadow DOM CSS isolation problem. See [shadow-dom-hostile-page-injection](../patterns/shadow-dom-hostile-page-injection.md) for the mount pattern. Shared components (panels, overlays, buttons) live under `src/ui/`, built once and mounted per-target by the entry points.

## Anti-Patterns

- **Build-time constants in shared code** — `if (__USERSCRIPT__)` inside `src/core/` couples shared modules to the build pipeline and makes testing harder. Detect at runtime in a single `core/platform.ts` module; constants are fine inside entry points
- **Separate codebases with a "shared utils" package** — tempting during the first feature split, but the divergence rate is high. Keep one codebase with per-target entry points; extract to a package only when a third consumer appears
- **Relying on chrome.storage in shared stores** — breaks the userscript target. Always go through `StorageBackend`
- **Hand-syncing manifest permissions and `@grant` lists** — drift is inevitable. Single source of truth

## Open Questions

- Firefox userscript support (Greasemonkey 4 uses a promise-based API that differs from Tampermonkey's GM_* globals) — worth a second abstraction layer or scope reduction?
- Extension MV3 service worker lifecycle (unloads aggressively) vs userscript long-lived page context — shared state that survives in one but not the other needs explicit handling
