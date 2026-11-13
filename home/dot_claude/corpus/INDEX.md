# Design Corpus Index

Reference of preferred patterns, conventions, and approaches. Read specific topic files on demand.

## Languages

- [rust](./languages/rust.md) — snafu everywhere, lint policy in files (expect, clippy.toml bans), workspace shape and hakari (not in no_std workspaces), per-package dev opt-levels, CPU baseline and glibc floor with zig, ts-rs contracts
- [typescript](./languages/typescript.md) — Two-tier strictness, Biome-only vs type-aware ESLint trade-off, discriminated unions, ts-rs over Zod
- [go](./languages/go.md) — Sentinel errors, writeJSON buffer, slog LogValuer types, koanf layered config, dynamic rate limiting
- [python](./languages/python.md) — Type hints everywhere, dataclasses over dicts, uv/ruff toolchain
- [kotlin-jvm](./languages/kotlin-jvm.md) — @JvmInline value classes, sealed state, tick-driven state machines, sealed ApiError
- [sql](./languages/sql.md) — JSONB patterns, tsvector search, materialized views, safe constraint migrations
- [shell](./languages/shell.md) — *(stub)* Fish-first, POSIX fallback, shellcheck compliance
- [css-styling](./languages/css-styling.md) — PandaCSS preferred, Tailwind v4 acceptable, oklch tokens, class dark mode (OS-follow for no-toggle sites), wrapped primitives, View Transitions
- [html-markup](./languages/html-markup.md) — Semantic elements, accessibility defaults
- [config-languages](./languages/config-languages.md) — TOML for human-edited, TS configs for complex schemas, self-registration
- [svelte](./languages/svelte.md) — Runes-only, SvelteKit config (adapter by runtime owner, CSP situational), SSR module-state caveat, controllers with injected ports, version-counter reactivity, FSM pipelines

## Architecture

- [api-design](./architecture/api-design.md) — Typed error codes, multi-tier rate limiting, dual-surface error mapping, ts-rs contracts
- [web-platform](./architecture/web-platform.md) — OPFS persistence, Comlink Web Workers, CSP reportOnly, breakpoint matchMedia
- [data-modeling](./architecture/data-modeling.md) — JSONB sub-entities, materialized views, safe constraint migrations
- [state-management](./architecture/state-management.md) — Signals/runes for reactivity, page-scoped factories, server vs client state
- [concurrency-async](./architecture/concurrency-async.md) — Service trait lifecycle, child-process supervisor, TaskTracker/CancellationToken, singleflight (Rust and TS), write coalescing, worker futures, threads without a runtime
- [graphql-schema-design](./architecture/graphql-schema-design.md) — Runtime aliased-batch queries, breadth-based budget packing, DataLoader N+1, async-graphql subscriptions
- [object-storage-patterns](./architecture/object-storage-patterns.md) — S3-compatible uploads, imgproxy/cdn-cgi transforms, orphan cleanup
- [image-processing-pipeline](./architecture/image-processing-pipeline.md) — Capture→transform→progressive loading, thumbhash placeholders
- [real-time-subscriptions](./architecture/real-time-subscriptions.md) — GraphQL WS, typed domain events, connectivity state
- [wasm-compilation-targets](./architecture/wasm-compilation-targets.md) — Dual-target Rust (native + Emscripten), platform-gated modules
- [isr-caching-proxy-patterns](./architecture/isr-caching-proxy-patterns.md) — Stale-while-revalidate, stale-if-error, DashSet singleflight, per-encoding compression, shared-cache auth bypass (single-user caches judgment), validated host in the key
- [game-loop-ecs-architecture](./architecture/game-loop-ecs-architecture.md) — Bevy ECS standalone, fixed-tick scheduling, SDL2 graphics
- [minecraft-mod-architecture](./architecture/minecraft-mod-architecture.md) — Architectury cross-loader, Mixin injection, reflection integrations, KSP codegen, in-game tests
- [asset-pipeline-atlas](./architecture/asset-pipeline-atlas.md) — Atlas JSON baked to a phf map, typed sprite enums, procedural bakes, the unrecorded-packer anti-pattern
- [custom-binary-protocol](./architecture/custom-binary-protocol.md) — Protocol framing, number-preserving wire enums, newtype invariants, bounded decoders, resync, proptest roundtrip
- [ml-inference-pipeline](./architecture/ml-inference-pipeline.md) — ONNX Runtime, tiled spatial inference, crossbeam thread pool, EMA outlier detection
- [tauri-desktop-app](./architecture/tauri-desktop-app.md) — Rust command/event IPC, typed errors at boundary, Pest PEG parser
- [linux-hardware-interfaces](./architecture/linux-hardware-interfaces.md) — /proc/pagemap, EDAC sysfs, SMBIOS, mlock, AVX-512 non-temporal stores
- [deterministic-simulation](./architecture/deterministic-simulation.md) — Seeded RNG, context-keyed streams, deterministic client-side execution
- [scoring-ranking-algorithms](./architecture/scoring-ranking-algorithms.md) — Weighted multi-factor scoring, log-scale proximity, adaptive priority, epsilon escalation, feedback-driven cost
- [platform-abstraction-layer](./architecture/platform-abstraction-layer.md) — Transport interface, build-time platform selection, separate entry points
- [scheduling](./architecture/scheduling.md) — Priority-queue dispatch, persisted tiered schedules with jitter and deferral, claim leases, activity-adaptive polling, browser polling engine
- [search-index-tfidf](./architecture/search-index-tfidf.md) — Custom inverted TF-IDF index vs managed FTS extensions, build-vs-buy
- [userscript-extension-dual-target](./architecture/userscript-extension-dual-target.md) — Vite-plugin-monkey + WXT from one src/, runtime platform detection, unified StorageBackend, cross-tab sync transports
- [linux-process-memory-injection](./architecture/linux-process-memory-injection.md) — process_vm_readv/writev, ptrace+dlopen injection, glam-in-Pod, loader-based reload, catch_unwind at the boundary, community-server scope
- [ffi-facade-and-unsafe-containment](./architecture/ffi-facade-and-unsafe-containment.md) — Thin FFI facade, abort at the C ABI, unsafe firewall, thread-affine singletons proved by a source-scan test
- [reloadable-rust-payloads](./architecture/reloadable-rust-payloads.md) — Permanent loader, per-generation payload copies, fingerprint handshake, eject contract, firmware loaders with watchdogs
- [firmware-no-std-uefi](./architecture/firmware-no-std-uefi.md) — no_std exactly when it links into firmware, HAL traits, typed registers, single-home API bans, host simulators
- [gpu-shader-pipeline](./architecture/gpu-shader-pipeline.md) — Shader layout declared once, naga validation, GLSL ES translation tests
- [native-gpu-ui](./architecture/native-gpu-ui.md) — GPUI and egui apps: drop-to-abort tasks, revision-keyed memos, fixtures, worker lanes; open question on vendoring GPUI
- [outbound-http-client](./architecture/outbound-http-client.md) — Per-host budgets, Retry-After blocks, jittered retry, SSRF guard on every hop, bounded headless browser
- [cloudflare-workers-platform](./architecture/cloudflare-workers-platform.md) — Smoke test gates deploy, test bindings away from production, D1 migration tooling
- [postgres-operations](./architecture/postgres-operations.md) — Index-before-migration ordering, claim leases with attempts, write coalescing, backup order

## Patterns

- [error-handling](./patterns/error-handling.md) — Typed errors with snafu, partial failure as values, stops as data, structured validation errors, never-throwing client API
- [logging-observability](./patterns/logging-observability.md) — Structured logging, tracing spans, wide events, decision labels, client telemetry, OutputBuffer, slog handler chains
- [testing-quality](./patterns/testing-quality.md) — TDD, Vitest dual-project setup, proptest roundtrip, assert2 (assert!(let ..)), rstest cases, insta, strict fakes, scenario ledgers
- [heavy-check-runners](./patterns/heavy-check-runners.md) — Experimental: purpose-built runners for shared-state-heavy checks (kk-dev, game-hacking dev crate, idakit-runner)
- [visual-regression-testing](./patterns/visual-regression-testing.md) — blink baselines, look at the images, offscreen UI fixtures, recorded stubs
- [untrusted-input-parsers](./patterns/untrusted-input-parsers.md) — Required limits, checked count chokepoint, explicit stacks, fuzz targets, resync
- [differential-oracle-testing](./patterns/differential-oracle-testing.md) — Independent oracles, both-direction predicates, per-artifact cases, refactor equivalence gates
- [filesystem-safety](./patterns/filesystem-safety.md) — Guards no config can disable, fd-anchored deletes, untrusted-repo git, atomic writes
- [security-auth](./patterns/security-auth.md) — Zero-trust defaults, token rotation, secret management
- [device-code-auth-flow](./patterns/device-code-auth-flow.md) — RFC 8628 issuance, typed states, completion transport as a choice (polling, SSE, long-poll), approver re-enters the code, hashed CLI tokens
- [performance](./patterns/performance.md) — Lazy per-encoding compression, SIMD dispatch (runtime vs compile-time trade-off), whole-task budgets, measured stacks, asymmetric EMA
- [bot-abuse-defense](./patterns/bot-abuse-defense.md) — Multi-mode tarpit streaming, dual semaphore RAII, malicious path detection
- [graceful-shutdown](./patterns/graceful-shutdown.md) — Signal escalation, AbortSignal threading, bounded drain, CancellationToken
- [session-cookie-auth](./patterns/session-cookie-auth.md) — Server-side sessions, opaque cookies, role-based middleware, pgtestdb auth tests
- [cursor-pagination](./patterns/cursor-pagination.md) — Opaque base64 cursors, keyset WHERE clauses, N+1 page detection
- [integration-test-db-isolation](./patterns/integration-test-db-isolation.md) — pgtestdb template cloning, in-memory factories, two-layer fixture composition
- [binary-reverse-engineering](./patterns/binary-reverse-engineering.md) — Documentation-driven RE, one findings doc per subject, live disassembler database as truth, name form encodes confidence, observe vs validate
- [shadow-dom-hostile-page-injection](./patterns/shadow-dom-hostile-page-injection.md) — Shadow DOM mount for Svelte/React into third-party pages, adoptedStyleSheets, MutationObserver lifecycle, module-scoped state over context
- [ai-assisted-reverse-engineering-loop](./patterns/ai-assisted-reverse-engineering-loop.md) — Six-phase RE loop, task-based agents, tool-use caps, anti-hallucination rules, autonomous probes

## Project Structure

- [repo-layout](./project-structure/repo-layout.md) — scripts/ dir, docs/ hierarchy, generated bindings, CLAUDE.md at root
- [build-systems](./project-structure/build-systems.md) — Runner by project shape (Justfile with gate for Rust-only, tempo task graph for multi-language), mise conventions, build locks, randomized ports
- [documentation-naming](./project-structure/documentation-naming.md) — File naming, numbered design specs with known gaps, durable docs, roadmaps, public naming with doc aliases
- [monorepo-workspace-library](./project-structure/monorepo-workspace-library.md) — apps/packages split, Bun workspace source exports, dual export modes

## Developer Experience

- [git-workflow](./dx/git-workflow.md) — Conventional commits, rebase-merge, branch naming
- [dependency-management](./dx/dependency-management.md) — Automated updates optional, advisory gating on what a change introduces (advisory-action), PM enforcement layers, Renovate grouping, cargo-deny, tool sync
- [cli-tool-design](./dx/cli-tool-design.md) — Global flag pre-pass, prefix matching, TTY/CI auto-detection, language-specific CLI frameworks
- [ci-cd-deployment](./dx/ci-cd-deployment.md) — GitHub Actions, Railway, Docker conventions
- [ai-assisted-dev](./dx/ai-assisted-dev.md) — CLAUDE.md patterns, skill design, subagent strategies
- [project-automation](./dx/project-automation.md) — Task graphs (needs/after), content-fingerprinted staleness, no-shell spawning, recoverable lock files, command registries
- [claude-code-hook-binaries](./dx/claude-code-hook-binaries.md) — Hook contract, typed abstain and fail-open, best-effort logging, startup budget, calibrated heuristics
- [cross-language-type-generation](./dx/cross-language-type-generation.md) — ts-rs (Rust→TS), tygo (Go→TS), schemars (Rust→Kotlin), CI verification via regen+diff
- [docker-multi-service](./dx/docker-multi-service.md) — Compose orchestration, multi-stage builds, UPX/BuildKit/bun-smol, randomized ports
- [headless-gpu-rendering](./dx/headless-gpu-rendering.md) — Guarded window tests under xvfb-run, lavapipe via WGPU_BACKEND, windowless devices, cross-runtime render comparison
- [npm-library-publishing](./dx/npm-library-publishing.md) — ESM builds, conditional exports, cross-runtime testing, release-please
- [build-time-codegen](./dx/build-time-codegen.md) — build.rs, cxx-gen, WGSL as one source, vendored C, sqlc/tygo, sqlx offline, KSP+KotlinPoet, commit-or-OUT_DIR, CI regen+diff
- [multi-target-build-pipeline](./dx/multi-target-build-pipeline.md) — Cargo-only native + browser pipeline, cross-linting Windows and macOS from Linux, Vite build modes (earlier form)
