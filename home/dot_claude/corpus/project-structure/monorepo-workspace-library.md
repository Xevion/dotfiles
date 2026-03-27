---
name: monorepo-workspace-library
category: project-structure
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/relatives
    path: packages/core/
    note: "Bun workspace with raw TS source exports for internal consumers, tsc+tsc-alias build for publishing"
  - repo: Xevion/borde.rs
    path: frontend/
    note: "pnpm workspaces with shared game protocol types consumed by multiple frontend targets"
---

# Monorepo Workspace Library

## Philosophy

Structure monorepos with clear package boundaries: `apps/` for deployable applications, `packages/` for internal libraries. Internal libraries are framework-agnostic and independently testable. The workspace protocol (`workspace:*`) ensures internal consumers always use the source version.

## Conventions

- **apps/ + packages/ split**: `apps/` contains deployable applications (SvelteKit, React, Electron), `packages/` contains internal libraries shared across apps. This mirrors the Turborepo convention and is the de-facto standard for npm/Bun monorepos
- **Source exports for internal consumers**: within a Bun workspace, point `exports` at raw `.ts` source files. Bun resolves TypeScript natively, eliminating the build step during development. The build script (tsc + tsc-alias) produces `dist/` only for external publishing
- **Dual export modes**: workspace-internal consumption uses source exports (`"import": "./src/index.ts"`). Before npm publish, exports must point at compiled `dist/` files with conditional `types`/`import` fields
- **Root package.json orchestration**: for Bun monorepos without a Justfile, root `scripts` with `bun run --cwd` commands serve as the task runner surface. Loses tab-completion and parameterization compared to Justfile recipes

## Anti-Patterns

- Publishing a workspace package with source exports (consumers outside the monorepo can't resolve `.ts`)
- Circular dependencies between packages/ entries
- Putting framework-specific code in a `packages/core` library (defeats the purpose of separation)

## Open Questions

- Turborepo/Nx adoption criteria for solo-developer monorepos
- Versioning strategy for internal packages (fixed vs independent)
