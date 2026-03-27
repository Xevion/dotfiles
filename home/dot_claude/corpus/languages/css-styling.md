---
name: css-styling
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: web/src/routes/layout.css
    note: Tailwind v4 @theme inline with oklch tokens, View Transitions for navigation
  - repo: Xevion/instant-upscale
    path: frontend/panda.config.ts
    note: PandaCSS with oklch tokens, semantic dark mode via _dark conditions
  - repo: Xevion/doujin-ocr-summary
    path: web/panda.config.ts
    note: PandaCSS primitive + semantic token system with oklch values
  - repo: Xevion/glint
    path: frontend/src/routes/layout.css
    note: "oklch relative color syntax, @custom-variant dark, direction-aware View Transitions"
  - repo: local/inkwell
    path: web/panda.config.ts
    note: "Two-tier oklch tokens (primitive → semantic), inline oklch alpha for dark borders, VT conflict suppression"
  - repo: Xevion/xevion.dev
    path: web/panda.config.ts
    note: VT keyframes defined in PandaCSS config (keyframes + globalCss)
---

# CSS & Styling

## Philosophy

Tailwind utility-first. Design tokens as CSS custom properties. Minimal custom CSS — reach for utilities first, extract components second, write raw CSS last.

## Conventions

- **Tailwind v4 with @theme inline**: bridge CSS custom properties to Tailwind utilities. Define tokens in `:root` and `.dark`, map them via `@theme inline { --color-background: var(--background); }`
- **oklch() color values**: use oklch for perceptually uniform color tokens. Easier to reason about lightness/chroma adjustments than hex/hsl

```css
:root {
  --background: oklch(0.985 0 0);
  --primary: oklch(0.205 0 0);
}
.dark {
  --background: oklch(0.145 0 0);
  --primary: oklch(0.922 0 0);
}
@theme inline {
  --color-background: var(--background);
  --color-primary: var(--primary);
}
```

- **Dark mode via class strategy**: `.dark` class on `documentElement`, not `prefers-color-scheme` media query (allows user override)
- **View Transitions API**: use for page navigation and theme-change animations. Named transition groups (`view-transition-name`) scope animations to specific elements. Data attributes on `:root` select directional keyframe variants
- **oklch() relative color syntax**: use `oklch(from var(--token) l c h / alpha)` for alpha-only variants without hardcoding or repeating the color value. Useful for glass effects, hover states, and overlay backgrounds
- **@custom-variant for dark mode with shadcn-ui**: declare `@custom-variant dark (&:is(.dark *));` in Tailwind v4 when using shadcn-ui's class-based dark mode. This replaces the default `prefers-color-scheme` detection with class-based toggling
- **Direction-aware View Transitions**: set a `data-nav-direction` attribute on `:root` before navigation, then select directional keyframe variants (slide-left vs slide-right) via `:root[data-nav-direction="left"]` selectors on `::view-transition-old`/`::view-transition-new`
- **View Transition conflict suppression**: when triggering a full-page theme transition, temporarily suppress all element-level `view-transition-name` assignments by adding a class (e.g., `.theme-transitioning *`) that sets `view-transition-name: none !important` on all descendants. Also reset the root group animation to prevent default crossfade conflicts
- **VT keyframes in PandaCSS config**: define View Transition keyframes in `panda.config.ts` (keyframes block) and reference them from `globalCss`, keeping all animation definitions co-located with the token system rather than mixing `@keyframes` into a separate CSS file
- **Responsive-first**: mobile-first breakpoints, avoid fixed widths
- **PandaCSS as an alternative to Tailwind v4**: define tokens in `panda.config.ts` using `oklch()`, with semantic aliases using `{ base: ..., _dark: ... }` condition variants. Same oklch + class-strategy dark mode philosophy as Tailwind v4, different implementation
- **PandaCSS dark mode**: use `_dark` semantic token conditions — equivalent to Tailwind's `dark:` variant. `.dark` class toggle on `documentElement` is the same activation mechanism as the Tailwind class strategy

## Anti-Patterns

- `!important` overrides — fix specificity at the source
- Deeply nested selectors (> 3 levels)
- Inline styles for layout
- Defining colors as hex/rgb literals directly in Tailwind classes instead of going through the token system
- CSS class-toggling or JS-driven opacity fades for page transitions when View Transitions API is available

## Open Questions

- Container queries adoption and responsive component patterns
- CSS-in-JS relevance in a Tailwind-first world
