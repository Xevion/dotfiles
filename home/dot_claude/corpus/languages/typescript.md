---
name: typescript
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: web/src/lib/bindings/
    note: ts-rs generated discriminated unions with "type" literal field
  - repo: Xevion/instant-upscale
    path: frontend/src/lib/pipeline/
    note: Typed Web Worker messages, Extract<Union> pub/sub, event bus with discriminated unions
  - repo: Xevion/tempo
    path: src/types.ts
    note: NoInfer<T> on Record keys, template-literal index types, const generic builder functions
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
- **Typed Web Worker communication**: define `MainToWorker` and `WorkerToMain` discriminated unions with typed wrapper functions over `postMessage`. Enforce compile-time exhaustiveness on message handling
- **`Extract<Union, { type: T }>`**: use this conditional type for narrowing subscriber callbacks in typed pub/sub systems
- **Hand-authored discriminated unions for frontend-only types**: when a type has no backend counterpart (e.g., loading state, UI error), hand-author a discriminated union with a `type` literal field following the same pattern as ts-rs-generated unions. Reserve ts-rs for API contract types
- **tygo as alternative to ts-rs for Go backends**: tygo generates TypeScript interfaces from Go structs with minimal transformation. Unlike ts-rs, tygo does not produce discriminated unions or enforce camelCase — it produces weaker TypeScript contracts. When event type fields are plain `string` rather than literal unions, callers resort to `as` casts instead of narrowing — an anti-pattern to watch for
- **Advanced type-level patterns** (situational — useful in config DSLs and builder APIs):
  - `NoInfer<T>` on Record keys to prevent generic widening
  - Template-literal types (`"${Subsystem}:${Action}"`) as index constraints for namespaced key systems
  - `<const T extends string>` on builder functions to preserve literal unions from inferred object keys
- **Tag-free structural unions for user-facing config**: `string | string[] | Object` unions are acceptable for user-facing config types. Reserve explicit `kind`/`type` discriminants for internal types

## Anti-Patterns

- `any` escape hatch — use `unknown` and narrow
- Type assertions (`as`) without prior narrowing
- Barrel exports (`index.ts`) in large projects — causes circular dependencies and tree-shaking issues
- Hand-maintaining TypeScript interfaces that mirror backend types when a code-gen pipeline exists

## Open Questions

- Effect-TS adoption for typed error handling
- Module resolution strategies (bundler vs node16)
