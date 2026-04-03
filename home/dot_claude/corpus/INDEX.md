# Design Corpus Index

Reference of preferred patterns, conventions, and approaches. Read specific topic files on demand.

## Languages

- [rust](./languages/rust.md) — thiserror/anyhow split, custom Cargo profiles, mold linker, rustls-tls, pedantic clippy, ts-rs contracts
- [typescript](./languages/typescript.md) — Two-tier strictness, Biome/ESLint conventions, discriminated unions, ts-rs over Zod
- [go](./languages/go.md) — Sentinel errors, writeJSON buffer, slog LogValuer types, koanf layered config, dynamic rate limiting
- [python](./languages/python.md) — Type hints everywhere, dataclasses over dicts, uv/ruff toolchain
- [kotlin-jvm](./languages/kotlin-jvm.md) — @JvmInline value classes, sealed state, tick-driven state machines, sealed ApiError
- [sql](./languages/sql.md) — JSONB patterns, tsvector search, materialized views, safe constraint migrations
- [shell](./languages/shell.md) — *(stub)* Fish-first, POSIX fallback, shellcheck compliance
- [css-styling](./languages/css-styling.md) — PandaCSS preferred, Tailwind v4 acceptable, oklch tokens, semantic dark mode, View Transitions
- [html-markup](./languages/html-markup.md) — Semantic elements, accessibility defaults
- [config-languages](./languages/config-languages.md) — TOML for human-edited, TS configs for complex schemas, self-registration
- [svelte](./languages/svelte.md) — Runes-only, SvelteKit config (Vite, adapter, CSP), version-counter reactivity, FSM pipelines

## Architecture

- [api-design](./architecture/api-design.md) — Typed error codes, multi-tier rate limiting, dual-surface error mapping, ts-rs contracts
- [web-platform](./architecture/web-platform.md) — OPFS persistence, Comlink Web Workers, CSP reportOnly, breakpoint matchMedia
- [data-modeling](./architecture/data-modeling.md) — JSONB sub-entities, materialized views, safe constraint migrations
- [state-management](./architecture/state-management.md) — Signals/runes for reactivity, page-scoped factories, server vs client state
- [concurrency-async](./architecture/concurrency-async.md) — Service trait lifecycle, TaskTracker/CancellationToken, Shared singleflight, crossbeam thread pools
- [graphql-schema-design](./architecture/graphql-schema-design.md) — Runtime aliased-batch queries, breadth-based budget packing, genqlient codegen
- [object-storage-patterns](./architecture/object-storage-patterns.md) — S3-compatible uploads, imgproxy/cdn-cgi transforms, orphan cleanup
- [image-processing-pipeline](./architecture/image-processing-pipeline.md) — Capture→transform→progressive loading, thumbhash placeholders
- [real-time-subscriptions](./architecture/real-time-subscriptions.md) — GraphQL WS, typed domain events, connectivity state
- [wasm-compilation-targets](./architecture/wasm-compilation-targets.md) — Dual-target Rust (native + Emscripten), platform-gated modules
- [isr-caching-proxy-patterns](./architecture/isr-caching-proxy-patterns.md) — *(stub)* Stale-while-revalidate, lazy multi-encoding compression, singleflight
- [game-loop-ecs-architecture](./architecture/game-loop-ecs-architecture.md) — Bevy ECS standalone, fixed-tick scheduling, SDL2 graphics
- [minecraft-mod-architecture](./architecture/minecraft-mod-architecture.md) — *(stub)* Architectury cross-loader, Mixin injection, tick-driven state machines
- [asset-pipeline-atlas](./architecture/asset-pipeline-atlas.md) — *(stub)* Sprite atlas packing, build.rs PHF maps, typed asset handles
- [custom-binary-protocol](./architecture/custom-binary-protocol.md) — Protocol framing, dispatch enums, newtype invariants, proptest roundtrip
- [ml-inference-pipeline](./architecture/ml-inference-pipeline.md) — ONNX Runtime, tiled spatial inference, crossbeam thread pool, EMA outlier detection
- [tauri-desktop-app](./architecture/tauri-desktop-app.md) — Rust command/event IPC, typed errors at boundary, Pest PEG parser
- [linux-hardware-interfaces](./architecture/linux-hardware-interfaces.md) — /proc/pagemap, EDAC sysfs, SMBIOS, mlock, AVX-512 non-temporal stores
- [deterministic-simulation](./architecture/deterministic-simulation.md) — Seeded RNG, context-keyed streams, deterministic client-side execution
- [scoring-ranking-algorithms](./architecture/scoring-ranking-algorithms.md) — Weighted multi-factor scoring, log-scale proximity, adaptive priority
- [platform-abstraction-layer](./architecture/platform-abstraction-layer.md) — Transport interface, build-time platform selection, separate entry points
- [scheduling](./architecture/scheduling.md) — Priority-queue dispatch, optimistic stamping, activity-adaptive polling
- [search-index-tfidf](./architecture/search-index-tfidf.md) — Custom inverted TF-IDF index vs managed FTS extensions, build-vs-buy

## Patterns

- [error-handling](./patterns/error-handling.md) — Typed errors, no stringly-typed, thiserror/anyhow split
- [logging-observability](./patterns/logging-observability.md) — Structured logging, tracing spans, OutputBuffer, slog handler chains, TUI channel layer
- [testing-quality](./patterns/testing-quality.md) — TDD, Vitest dual-project setup, proptest roundtrip, assert2 chaining, recording sinks
- [security-auth](./patterns/security-auth.md) — Zero-trust defaults, token rotation, secret management
- [device-code-auth-flow](./patterns/device-code-auth-flow.md) — *(stub)* RFC 8628 for headless clients, polling with backoff, session token exchange
- [performance](./patterns/performance.md) — Lazy per-encoding compression, runtime SIMD dispatch, asymmetric EMA, promise-chain mutex
- [bot-abuse-defense](./patterns/bot-abuse-defense.md) — *(stub)* Tarpit streaming, per-IP semaphore limits, active defense
- [graceful-shutdown](./patterns/graceful-shutdown.md) — Signal escalation, AbortSignal threading, bounded drain, CancellationToken
- [session-cookie-auth](./patterns/session-cookie-auth.md) — Server-side sessions, opaque cookies, role-based middleware, pgtestdb auth tests
- [cursor-pagination](./patterns/cursor-pagination.md) — Opaque base64 cursors, keyset WHERE clauses, N+1 page detection
- [integration-test-db-isolation](./patterns/integration-test-db-isolation.md) — pgtestdb template cloning, in-memory factories, two-layer fixture composition
- [binary-reverse-engineering](./patterns/binary-reverse-engineering.md) — Documentation-driven RE, implementation-as-validation, static analysis workflow

## Project Structure

- [repo-layout](./project-structure/repo-layout.md) — scripts/ dir, docs/ hierarchy, generated bindings, CLAUDE.md at root
- [build-systems](./project-structure/build-systems.md) — Tempo as primary workflow tool, Justfile passthrough, mise conventions, randomized ports
- [documentation-naming](./project-structure/documentation-naming.md) — *(stub)* README templates, file naming, ADRs
- [monorepo-workspace-library](./project-structure/monorepo-workspace-library.md) — apps/packages split, Bun workspace source exports, dual export modes

## Developer Experience

- [git-workflow](./dx/git-workflow.md) — Conventional commits, rebase-merge, branch naming
- [dependency-management](./dx/dependency-management.md) — PM enforcement layers, Renovate grouping, cargo-deny, CI/local tool sync, transitive overrides
- [cli-tool-design](./dx/cli-tool-design.md) — Global flag pre-pass, prefix matching, TTY/CI auto-detection, language-specific CLI frameworks
- [ci-cd-deployment](./dx/ci-cd-deployment.md) — GitHub Actions, Railway, Docker conventions
- [ai-assisted-dev](./dx/ai-assisted-dev.md) — CLAUDE.md patterns, skill design, subagent strategies
- [project-automation](./dx/project-automation.md) — Command registries, staleness detection, smart auto-fix
- [cross-language-type-generation](./dx/cross-language-type-generation.md) — ts-rs (Rust→TS), tygo (Go→TS), CI verification via regen+diff
- [docker-multi-service](./dx/docker-multi-service.md) — Compose orchestration, multi-stage builds, UPX/BuildKit/bun-smol, randomized ports
- [headless-gpu-rendering](./dx/headless-gpu-rendering.md) — *(stub)* Xvfb + VirtualGL, NVIDIA passthrough, containerized GPU workloads
- [npm-library-publishing](./dx/npm-library-publishing.md) — ESM builds, conditional exports, cross-runtime testing, release-please
- [build-time-codegen](./dx/build-time-codegen.md) — build.rs, sqlc/tygo, sqlx offline, CI verification via regen+diff
- [multi-target-build-pipeline](./dx/multi-target-build-pipeline.md) — WASM + Tauri + native from one codebase, Vite build modes, mtime detection
