---
name: css-styling
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: web/src/routes/layout.css
    note: Tailwind v4 @theme inline with oklch tokens, View Transitions for navigation
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
- **Responsive-first**: mobile-first breakpoints, avoid fixed widths

## Anti-Patterns

- `!important` overrides — fix specificity at the source
- Deeply nested selectors (> 3 levels)
- Inline styles for layout
- Defining colors as hex/rgb literals directly in Tailwind classes instead of going through the token system
- CSS class-toggling or JS-driven opacity fades for page transitions when View Transitions API is available

## Open Questions

- Container queries adoption and responsive component patterns
- CSS-in-JS relevance in a Tailwind-first world
