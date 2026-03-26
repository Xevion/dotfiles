---
name: typescript
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: web/src/lib/bindings/
    note: ts-rs generated discriminated unions with "type" literal field
---

# TypeScript

## Philosophy

Strict mode always. Types as documentation. Functional patterns preferred. Let the type system do the work — avoid runtime validation when a compile-time guarantee exists.

## Conventions

- **Strict compiler options**: `strict: true`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`
- **ts-rs replaces Zod for backend-driven types**: when the backend generates TypeScript bindings (via ts-rs, protobuf, etc.), trust those types as the source of truth. No redundant Zod schemas for API response shapes
- **Discriminated unions**: use a `"type"` string-literal field as the discriminant. Match with `switch`/`if` chains on the `"type"` field, never cast with `as`

```typescript
// Pattern: discriminated union (ts-rs output format)
type InstructionalMethod =
  | { type: "InPerson" }
  | { type: "Online"; variant: OnlineVariant }
  | { type: "Hybrid"; variant: HybridVariant };
```

- **Absolute imports** (`$lib/...`, `@/...`) over relative imports when the project supports path aliases
- **Functional patterns**: `map`, `filter`, `reduce` over `for` loops when idiomatic
- **Zod for external/user input**: use Zod only at system boundaries (form input, third-party APIs), not for internally-generated types

## Anti-Patterns

- `any` escape hatch — use `unknown` and narrow
- Type assertions (`as`) without prior narrowing
- Barrel exports (`index.ts`) in large projects — causes circular dependencies and tree-shaking issues
- Hand-maintaining TypeScript interfaces that mirror backend types when a code-gen pipeline exists

## Open Questions

- Effect-TS adoption for typed error handling
- Module resolution strategies (bundler vs node16)
