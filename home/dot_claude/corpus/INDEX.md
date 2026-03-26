# Design Corpus Index

Reference of preferred patterns, conventions, and approaches. Read specific topic files on demand.

## Languages

- [rust](./languages/rust.md) — Ownership-centric design, error enums, builder patterns
- [typescript](./languages/typescript.md) — Strict mode, discriminated unions, Zod schemas
- [go](./languages/go.md) — Simplicity-first, error values, interface-driven design
- [python](./languages/python.md) — Type hints everywhere, dataclasses over dicts, uv/ruff toolchain
- [kotlin-jvm](./languages/kotlin-jvm.md) — Coroutines, sealed classes, Gradle Kotlin DSL
- [sql](./languages/sql.md) — Migration discipline, naming conventions, query patterns
- [shell](./languages/shell.md) — Fish-first, POSIX fallback, shellcheck compliance
- [css-styling](./languages/css-styling.md) — Tailwind utility-first, CSS custom properties
- [html-markup](./languages/html-markup.md) — Semantic elements, accessibility defaults
- [config-languages](./languages/config-languages.md) — TOML preferred, YAML when forced, JSON for machines

## Architecture

- [api-design](./architecture/api-design.md) — REST conventions, response envelopes, versioning
- [data-modeling](./architecture/data-modeling.md) — Schema-first, normalize then denormalize intentionally
- [state-management](./architecture/state-management.md) — Signals/stores, server state vs client state
- [concurrency-async](./architecture/concurrency-async.md) — Structured concurrency, cancellation, backpressure

## Patterns

- [error-handling](./patterns/error-handling.md) — Typed errors, no stringly-typed, thiserror/anyhow split
- [logging-observability](./patterns/logging-observability.md) — Structured logging, tracing spans, metric naming
- [testing-quality](./patterns/testing-quality.md) — TDD when infra exists, property tests for invariants
- [security-auth](./patterns/security-auth.md) — Zero-trust defaults, token rotation, secret management
- [performance](./patterns/performance.md) — Measure first, profile-guided optimization

## Project Structure

- [repo-layout](./project-structure/repo-layout.md) — Monorepo vs polyrepo, workspace conventions
- [build-systems](./project-structure/build-systems.md) — Cargo, Gradle KTS, Bun, Just
- [documentation-naming](./project-structure/documentation-naming.md) — README templates, file naming, ADRs

## Developer Experience

- [git-workflow](./dx/git-workflow.md) — Conventional commits, rebase-merge, branch naming
- [dependency-management](./dx/dependency-management.md) — Lock files, update cadence, audit discipline
- [cli-tool-design](./dx/cli-tool-design.md) — Clap/Commander patterns, progressive disclosure
- [ci-cd-deployment](./dx/ci-cd-deployment.md) — GitHub Actions, Railway, Docker conventions
- [ai-assisted-dev](./dx/ai-assisted-dev.md) — CLAUDE.md patterns, skill design, subagent strategies
