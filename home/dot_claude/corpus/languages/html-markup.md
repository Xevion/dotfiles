---
name: html-markup
category: languages
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/rdap
    path: src/pages/index.tsx + src/rdap/components/
    note: "Radix asChild for semantic landmarks, sr-only labels, aria-label+title on icon buttons, inline validation via icon+tooltip"
---

# HTML & Markup

## Philosophy

Semantic elements first. Accessibility by default. ARIA only when semantic HTML falls short. Progressive enhancement — features degrade gracefully without JavaScript.

## Conventions

- **Radix UI `asChild` for semantic HTML**: `<Flex asChild><nav>` renders a single `<nav>` element with Flex styles applied, avoiding wrapper div soup. Use `asChild` to compose layout primitives with landmark elements
- **`sr-only` labels for unlabeled form controls**: when a form control has no visible label (e.g., a search input with placeholder text), add a `<label htmlFor="id" className="sr-only">` for screen readers
- **`aria-label` + `title` on icon-only buttons**: pair both attributes — `aria-label` for screen readers, `title` for sighted users who hover. Neither alone provides full accessibility
- **Inline validation via icon + tooltip**: for non-blocking real-time validation feedback, place a status icon in an input slot with a Tooltip wrapping it. The tooltip provides the accessible description; the icon provides the visual signal. Avoids `aria-invalid` + error text that shifts layout

## Anti-Patterns

- Div soup — use semantic elements (`nav`, `main`, `section`, `article`, `aside`)
- Missing alt text on images
- Layout tables
- Autofocus abuse (disorienting for screen reader users)

## Open Questions

- Dialog/modal accessibility patterns (focus trapping, escape handling)
