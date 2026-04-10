---
name: monorepo-workspace-library
category: project-structure
last_audited: 2026-04-10
exemplars:
  - repo: Xevion/relatives
    path: packages/core/
    note: "Bun workspace with raw TS source exports for internal consumers, tsc+tsc-alias build for publishing"
  - repo: Xevion/borde.rs
    path: frontend/
    note: "pnpm workspaces with shared game protocol types consumed by multiple frontend targets"
  - repo: local/Applyhelm
    path: crates/ + packages/
    note: "Cross-language monorepo: Cargo workspace (crates/) alongside Bun workspace (packages/), shared via ts-rs bindings"
---

# Monorepo Workspace Library

## Philosophy

Structure monorepos with clear package boundaries: `apps/` for deployable applications, `packages/` for internal libraries. Internal libraries are framework-agnostic and independently testable. The workspace protocol (`workspace:*`) ensures internal consumers always use the source version.

## Conventions

- **apps/ + packages/ split**: `apps/` contains deployable applications (SvelteKit, React, Electron), `packages/` contains internal libraries shared across apps. This mirrors the Turborepo convention and is the de-facto standard for npm/Bun monorepos
- **Source exports for internal consumers**: within a Bun workspace, point `exports` at raw `.ts` source files. Bun resolves TypeScript natively, eliminating the build step during development. The build script (tsc + tsc-alias) produces `dist/` only for external publishing
- **Dual export modes**: workspace-internal consumption uses source exports (`"import": "./src/index.ts"`). Before npm publish, exports must point at compiled `dist/` files with conditional `types`/`import` fields
- **Root package.json orchestration**: for Bun monorepos without a Justfile, root `scripts` with `bun run --cwd` commands serve as the task runner surface. Loses tab-completion and parameterization compared to Justfile recipes
- **Cross-language monorepos: language-boundary split, not apps/packages split**: when a monorepo combines a Cargo workspace with a Bun workspace, use `crates/` for all Rust packages and `packages/` for all TypeScript packages regardless of the apps-vs-libraries distinction within each language. The language boundary takes precedence because Cargo and Bun each need a flat workspace member list, and introducing an `apps/crates/backend/` nesting breaks Cargo's workspace discovery. Document the choice explicitly in an ADR — it diverges from the single-language Turborepo convention for a concrete tooling reason. `packages/extension` and `packages/web` are still "apps" in the Turborepo sense; `packages/api-client` and `packages/shared-types` are still "libraries"; the split lives in naming and documentation, not directory structure

## Anti-Patterns

- Publishing a workspace package with source exports (consumers outside the monorepo can't resolve `.ts`)
- Circular dependencies between packages/ entries
- Putting framework-specific code in a `packages/core` library (defeats the purpose of separation)

## Open Questions

- Turborepo/Nx adoption criteria for solo-developer monorepos
- Versioning strategy for internal packages (fixed vs independent)
