---
name: shadow-dom-hostile-page-injection
category: patterns
last_audited: 2026-04-10
exemplars:
  - repo: local/ts-chan
    path: src/ui/mount.ts
    note: "Canonical mountPanel + autoMount pattern with CSSStyleSheet.replaceSync and MutationObserver lifecycle"
  - repo: local/Applyhelm
    path: packages/extension/
    note: "WXT createShadowRootUi wrapper around Svelte 5 components with bits-ui portal overrides"
---

# Shadow DOM Hostile-Page Injection

## Philosophy

When a userscript or extension injects UI into a third-party page, the host page is **adversarial**: its CSS reset may strip your borders, its `z-index` stack may bury your overlay, its global selectors may style your inputs, and its MutationObservers may tear down nodes that look unfamiliar. Shadow DOM is the isolation boundary — styles and selectors on the outside don't leak in, and vice versa. But isolation breaks framework assumptions: `createContext()` doesn't cross mount boundaries, CSS-in-JS runtime injection targets `document.head` by default, and portal-based overlays (dropdowns, dialogs) escape to the light DOM and re-expose the host's styles. Treat every mount as a boundary, and every shared resource as something you must route explicitly.

## Conventions

- **One host div per logical panel, with `attachShadow({ mode: "open" })`** — `mode: "open"` lets your own code re-enter the shadow root for later queries; `mode: "closed"` adds nothing against a determined host page and hinders debugging. Create the host div, attach the shadow root, and treat the `ShadowRoot` as the scoped document from that point on
- **Inject styles via `adoptedStyleSheets` + `CSSStyleSheet.replaceSync`** — not via `<style>` tags inside the shadow root (fine but slower for large stylesheets), and never via `document.head` (defeats the isolation). One `CSSStyleSheet` object per logical style bundle; assign the array to `shadowRoot.adoptedStyleSheets`
- **Mount the framework into a child `div` of the shadow root, not the root itself** — Svelte's `mount()`, React's `createRoot()`, and similar APIs expect a plain element as target. Passing the `ShadowRoot` directly works for some frameworks but not others; a child `div` is portable
- **Module-scoped state over context for multi-root sharing** — `createContext()` in Svelte/React propagates through the mount tree, but NOT across independent `mount()` calls into separate shadow roots on the same page. A settings panel mounted at the top of the page and a per-post decoration mounted inline are two separate trees; shared state must live in a module-scoped store (`.svelte.ts` singleton, Zustand module, Valtio proxy) imported from both
- **Override headless component portal targets** — bits-ui, Radix, Headless UI, Ark UI all portal overlays (dropdowns, popovers, dialogs) to `document.body` by default. Inside a shadow root, this re-exposes the host's CSS. Pass a portal target prop pointing to a child of the shadow root instead. If the component doesn't support portal override, replace it or inline
- **`MutationObserver` for dynamic mount points** — some host pages render mount targets lazily, rebuild them on navigation, or remove them mid-session. `autoMount(selector, factory)` observes `document.body`, mounts when the target appears, and unmounts+cleans up when it disappears. Return a cleanup function from the factory so the observer can call it on removal

## Mount Pattern

```typescript
export function mountPanel<T>(
  Component: ComponentType<T>,
  target: HTMLElement,
  props: T,
  css: string,
): () => void {
  const host = document.createElement("div");
  const shadow = host.attachShadow({ mode: "open" });
  const sheet = new CSSStyleSheet();
  sheet.replaceSync(css);
  shadow.adoptedStyleSheets = [sheet];
  const mountPoint = document.createElement("div");
  shadow.appendChild(mountPoint);
  target.appendChild(host);
  const app = mount(Component, { target: mountPoint, props });
  return () => {
    unmount(app);
    host.remove();
  };
}
```

## CSS Inside Shadow Roots

Shadow roots are excluded from `@theme` token systems (Tailwind v4, PandaCSS) because those operate on `document.documentElement`. Pragmatic patterns:

- **Redefine tokens on `:host`** — `:host { --bg-primary: oklch(0.06 0 0); --accent: oklch(0.6 0.2 250); }`. Mirror the outer design system's tokens manually; the stylesheet is built once and injected
- **Prefer oklch over hex in the injected CSS** — the corpus convention still applies; the constraint is *where* the tokens live (`:host` vs `:root`)
- **Dark mode via `@media (prefers-color-scheme: dark)` inside the injected CSS** — `:host-context(.dark)` works if the host page uses a `.dark` class, but most hostile host pages won't, so prefer the media query
- **No Tailwind utilities inside the shadow root** — Tailwind's `@layer base` and utility generation target the light DOM. Hand-authored CSS with custom properties is the correct pattern; import CSS as `?raw` and pass as a string to `mountPanel`

## Sanitization

Any HTML from the host page (user posts, third-party content) that gets inserted into the shadow root must be sanitized — the shadow root does NOT block `<script>` execution or inline event handlers. Use DOMPurify or equivalent before `.innerHTML =` or `{@html}` insertion.

## Anti-Patterns

- **`mode: "closed"` for "security"** — adds no security against a host page that can observe your script, and strips debugging affordances
- **`document.body.appendChild(mountPoint)` with global styles** — inherits the host's CSS reset, box-sizing, z-index stack, and pseudo-element conflicts. If you're not using Shadow DOM, you're fighting the host page
- **`createContext()` shared across panels mounted at different points** — silently breaks when the two panels are in separate shadow roots. Use module-scoped stores
- **Injecting a `<link rel="stylesheet" href="...">` into the shadow root** — works but adds a network request per mount; `adoptedStyleSheets` is build-time resolved
- **Portal targets left as `document.body`** — the primary source of "my dropdown looks wrong" bug reports

## Open Questions

- Declarative Shadow DOM (`<template shadowrootmode>`) for SSR-injected panels in extensions with content scripts that run before DOMContentLoaded — does it survive site-page rewrites?
- ConstructableStyleSheet compatibility in older Safari (<16.4) — fallback to `<style>` element injection or drop support?
