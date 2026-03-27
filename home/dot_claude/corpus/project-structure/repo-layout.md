---
name: repo-layout
category: project-structure
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/banner
    path: ""
    note: Rust + SvelteKit monorepo with scripts/, docs/ hierarchy, and auto-generated bindings
  - repo: local/maestro
    path: docs/
    note: Two-level docs structure with KOTLIN.md entry point + patterns/ subdirectory
  - repo: Xevion/tempo
    path: fixtures/sample-project/
    note: Runnable consumer fixture for integration testing
  - repo: Xevion/glint
    path: docs/
    note: "STYLE.md with per-subsystem links, VOCABULARY.md for domain terminology and anti-patterns"
---

# Repo Layout

## Philosophy

Convention over configuration. Predictable locations. Monorepo for tightly coupled backend + frontend. Every project should be navigable by reading the root directory listing.

## Conventions

- **Standard directories**: `src/` for source, `tests/` for integration tests, `docs/` for documentation, `scripts/` for build orchestration, `migrations/` for database migrations
- **Justfile at root**: the entry point for all project commands (see [build-systems](../project-structure/build-systems.md))
- **CLAUDE.md at root**: AI agent context document (see [ai-assisted-dev](../dx/ai-assisted-dev.md))
- **scripts/ with independent dependencies**: build scripts live in `scripts/` with a `scripts/lib/` for shared utilities. Give scripts their own `package.json`/lockfile so dependencies are tracked separately from the application
- **docs/ hierarchy**: a top-level `STYLE.md` that links to language-specific guides (`RUST.md`, `SVELTE.md`). In-flight design work goes in `docs/plans/`. CLAUDE.md references these as required reading
- **Auto-generated code in `lib/bindings/`**: cross-language generated types (ts-rs, protobuf, etc.) live in a dedicated bindings directory with an auto-maintained barrel index. The build system must remove stale files when source types are renamed or deleted
- **`lib/bindings/` barrel generator**: companion script with idempotent write (skip when content unchanged) to avoid spurious git diffs. Justfile `bindings` recipe chains type generation and barrel regeneration
- **Two-level `docs/`**: entry-point file (`docs/KOTLIN.md`, `docs/STYLE.md`) with a quick-reference table linking to `docs/patterns/` where each topic gets its own file with code examples. `docs/VOCABULARY.md` documents domain terminology AND naming anti-patterns to prevent naming drift across language stacks. In multilingual monorepos, models are the source of truth for structure; VOCABULARY.md is the source of truth for naming
- **Standard field names table in STYLE.md**: document structured log field names (`duration_ms`, `shader_id`, etc.) in the style guide, ensuring consistent field names across all language stacks in a monorepo
- **`fixtures/<name>/` for runnable consumer fixtures**: complete mini-projects at the project root, serving as integration test input and living documentation

## Anti-Patterns

- Flat file dumps at root (config files scattered instead of in standard locations)
- Deeply nested `src/` directories without clear module boundaries
- Generated code mixed with hand-written code without clear separation
- Build scripts without their own dependency tracking (polluting app dependencies)

- **apps/ + packages/ monorepo split**: for npm/Bun monorepos, `apps/` contains deployable applications (SvelteKit, Electron, mobile), `packages/` contains internal libraries shared across apps. This mirrors the Turborepo convention and is the de-facto standard even when no monorepo tooling is used. See [monorepo-workspace-library](./monorepo-workspace-library.md) for intra-repo package boundary conventions

## Open Questions

- Monorepo tooling maturity (Turborepo/Nx) for larger projects
- Workspace conventions per language (Cargo workspaces, npm workspaces, Go modules)
