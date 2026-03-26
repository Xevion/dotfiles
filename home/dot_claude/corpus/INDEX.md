# Design Corpus Index

Reference of preferred patterns, conventions, and approaches. Read specific topic files on demand.

## Languages

- [rust](./languages/rust.md) — thiserror/anyhow split, extension traits, ts-rs contracts, lifetime-bound filters
- [typescript](./languages/typescript.md) — Strict mode, discriminated unions, ts-rs over Zod, functional patterns
- [go](./languages/go.md) — Simplicity-first, error values, interface-driven design
- [python](./languages/python.md) — Type hints everywhere, dataclasses over dicts, uv/ruff toolchain
- [kotlin-jvm](./languages/kotlin-jvm.md) — Coroutines, sealed classes, Gradle Kotlin DSL
- [sql](./languages/sql.md) — JSONB patterns, tsvector search, materialized views, safe constraint migrations
- [shell](./languages/shell.md) — Fish-first, POSIX fallback, shellcheck compliance
- [css-styling](./languages/css-styling.md) — Tailwind v4 @theme inline, oklch tokens, View Transitions
- [html-markup](./languages/html-markup.md) — Semantic elements, accessibility defaults
- [config-languages](./languages/config-languages.md) — TOML preferred, YAML when forced, JSON for machines
- [svelte](./languages/svelte.md) — Runes-only, createContext for shared state, bits-ui composition

## Architecture

- [api-design](./architecture/api-design.md) — Typed error codes, cache constants, rate limiting, ts-rs contracts
- [web-platform](./architecture/web-platform.md) — Native APIs over libraries, progressive enhancement
- [data-modeling](./architecture/data-modeling.md) — JSONB sub-entities, materialized views, safe constraint migrations
- [state-management](./architecture/state-management.md) — Signals/stores, server state vs client state
- [concurrency-async](./architecture/concurrency-async.md) — Structured concurrency, cancellation, backpressure

## Patterns

- [error-handling](./patterns/error-handling.md) — Typed errors, no stringly-typed, thiserror/anyhow split
- [logging-observability](./patterns/logging-observability.md) — Structured logging, tracing spans, metric naming
- [testing-quality](./patterns/testing-quality.md) — TDD when infra exists, property tests for invariants
- [security-auth](./patterns/security-auth.md) — Zero-trust defaults, token rotation, secret management
- [performance](./patterns/performance.md) — Measure first, profile-guided optimization

## Project Structure

- [repo-layout](./project-structure/repo-layout.md) — scripts/ dir, docs/ hierarchy, generated bindings, CLAUDE.md at root
- [build-systems](./project-structure/build-systems.md) — Justfile as thin wrapper, language-native builds, mise
- [documentation-naming](./project-structure/documentation-naming.md) — README templates, file naming, ADRs

## Developer Experience

- [git-workflow](./dx/git-workflow.md) — Conventional commits, rebase-merge, branch naming
- [dependency-management](./dx/dependency-management.md) — Lock files, update cadence, audit discipline
- [cli-tool-design](./dx/cli-tool-design.md) — Clap/Commander patterns, progressive disclosure
- [ci-cd-deployment](./dx/ci-cd-deployment.md) — GitHub Actions, Railway, Docker conventions
- [ai-assisted-dev](./dx/ai-assisted-dev.md) — CLAUDE.md patterns, skill design, subagent strategies
- [project-automation](./dx/project-automation.md) — Command registries, staleness detection, smart auto-fix
