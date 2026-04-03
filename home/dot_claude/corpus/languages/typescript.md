---
name: typescript
category: languages
last_audited: 2026-04-03
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

- **Strict compiler options (two tiers)**:
  - **Baseline** (all projects): `strict: true`, `skipLibCheck: true`, `moduleResolution: "bundler"`. For Bun projects add `"types": ["bun-types"]`, `allowImportingTsExtensions: true`, `verbatimModuleSyntax: true`
  - **Aggressive** (libraries, CLI tools, mature projects): baseline + `noUnusedLocals: true`, `noUnusedParameters: true`, `noUncheckedIndexedAccess: true`, `noFallthroughCasesInSwitch: true`. tempo and recall use this tier. Catches more bugs but adds friction during prototyping — enable when the codebase stabilizes
- **ts-rs replaces Zod for backend-driven types**: when the backend generates TypeScript bindings (via ts-rs, protobuf, etc.), trust those types as the source of truth. No redundant Zod schemas for API response shapes
- **Discriminated unions**: use a string-literal field as the discriminant. The field name need not be `"type"` — any domain-native literal field works (e.g., `objectClassName` for RDAP objects). The key convention is that the field is a string literal union and narrowing is done via `switch`/`if` without casts

```typescript
// Pattern: discriminated union (ts-rs output format)
type InstructionalMethod =
  | { type: "InPerson" }
  | { type: "Online"; variant: OnlineVariant }
  | { type: "Hybrid"; variant: HybridVariant };
```

- **Absolute imports** (`$lib/...`, `@/...`) over relative imports when the project supports path aliases
- **Functional patterns**: `map`, `filter`, `reduce` over `for` loops when idiomatic
- **Zod for external/user input**: use Zod only at system boundaries (form input, third-party APIs), not for internally-generated types. For pure-frontend apps consuming multiple third-party APIs without a backend code-gen pipeline, Zod at each API boundary with `z.infer<typeof schema>` as the sole type source is the correct approach — not a violation of this convention
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
- Type assertions (`as`) without prior narrowing — includes duck-typed capability detection (`"method" in obj`) with inline cast, which indicates an incomplete interface
- **Untagged multi-state unions**: `boolean | string` with 3 semantic states (false=no match, true=valid, string=invalid) — prefer a tagged union (`{ kind: "no-match" } | { kind: "valid" } | { kind: "invalid"; message: string }`) for self-documenting narrowing
- **Inline types in component scripts**: when a type and its associated data constants are large enough to constitute a domain concept, extract to a co-located utility module rather than embedding in a component script block
- Barrel exports (`index.ts`) in large projects — causes circular dependencies and tree-shaking issues
- Hand-maintaining TypeScript interfaces that mirror backend types when a code-gen pipeline exists

### Linting & Formatting

- **Biome preferred** for new JS/TS projects as both formatter and linter. Standard config: VCS integration (`useIgnoreFile: true`), tab indentation, `noConsole` with `allow: ["assert", "error", "warn"]`, `noExcessiveCognitiveComplexity` at warning level. Biome replaces both Prettier and ESLint for pure TS projects
- **Biome formatter + ESLint linter** for SvelteKit projects — Biome's Svelte linting support is incomplete. Use Biome for formatting (.ts, .json, .css) and ESLint with `svelte-eslint-parser` + `@xevion/ts-eslint-extra` for linting .svelte files. Prettier+ESLint is legacy but acceptable for existing projects
- **Underscore prefix for intentionally-unused params**: configure ESLint's `no-unused-vars` (or `@typescript-eslint/no-unused-vars`) with `argsIgnorePattern: "^_"`, `varsIgnorePattern: "^_"`. Standard across all projects
- **`--max-warnings 0` in lint scripts**: zero-tolerance warning policy. Combined with `--cache` for incremental speed. Prevents warnings from accumulating
- **Type-aware ESLint** (`parserOptions: { project: "./tsconfig.json" }`): enables rules like `consistent-type-imports`, `no-floating-promises`, `no-misused-promises`. Slower startup but catches real bugs. Enable for mature projects; skip for rapid prototyping
- **`svelte/no-navigation-without-resolve` disabled**: this rule conflicts with SPA routing patterns (Tauri, static adapters). Disable project-wide in Svelte projects that don't use server-side navigation resolution

## Open Questions

- Effect-TS adoption for typed error handling
- Module resolution strategies (bundler vs node16)
