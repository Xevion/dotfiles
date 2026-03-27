---
name: testing-quality
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: tests/
    note: sqlx::test with builder fixtures, Vitest named projects (unit + storybook browser tests)
  - repo: Xevion/doujin-ocr-summary
    path: internal/testutil/
    note: Go test factories with options-struct pattern, pgtestdb template cloning
  - repo: local/maestro
    path: common test module
    note: Kotest DescribeSpec with property-based pathfinding tests, TestWorldBuilder spatial DSL
  - repo: Xevion/instant-upscale
    path: frontend/src/lib/pipeline/__tests__/
    note: Vitest recording sink setup, EngineTestHarness browser API mock factory
  - repo: Xevion/tempo
    path: tests/
    note: Cross-runtime compat tests with CI env-var gate
  - repo: Xevion/glint
    path: mod/common test module + scripts/check.ts
    note: "Cross-language JSON schema contract tests between Rust backend and Kotlin mod"
  - repo: local/inkwell
    path: internal/testutil/dbtest/
    note: pgtestdb with CI env-var port switching, goosemigrator with embedded FS
  - repo: Xevion/Pac-Man
    path: pacman/tests/ + pacman-server/tests/
    note: "ECS World fixture builders, testcontainers + bon builder for app-state integration tests"
  - repo: local/bose-re
    path: crates/bose-protocol/ tests
    note: "proptest roundtrip + never-panics for BMAP codec, nested test sub-modules, assert2 chaining"
  - repo: local/novel
    path: src-tauri/src/dialogue/parser_tests.rs
    note: "let_assert! chaining for nested tree traversal, check!() for non-fatal leaf assertions"
  - repo: Xevion/ferrite
    path: src/output.rs + src/pattern.rs
    note: "Temp-file NDJSON recording sink, runtime hardware-feature gate for AVX-512 tests"
  - repo: Xevion/railway-collector
    path: internal/state/store_test.go
    note: "openTestStore(t) factory for bbolt, countingHandler slog test helper"
---

# Testing & Quality

## Philosophy

TDD when test infrastructure exists. Integration tests over mocks — hit real databases, real APIs. Test behavior, not implementation. Property tests for invariants.

## Conventions

- **Test naming**: describes the behavior being tested, not the method name
- **Arrange-Act-Assert**: clear three-phase structure in every test
- **Builder patterns for fixtures**: test helpers with sensible defaults (`Default::default()`) so test bodies focus on the scenario, not fixture construction
- **Cross-language contract tests via JSON schema**: for multi-language systems without a shared IDL, export JSON schemas from the source-of-truth language (e.g., `schemars` in Rust) and write deserialization compatibility tests in consumer languages (e.g., Kotlin). Must be paired with mtime-based schema regeneration in CI so stale schemas don't mask drift

## Language-Specific

### Rust

- `cargo nextest` as the test runner (parallel, better output than `cargo test`)
- `#[sqlx::test]` for database integration tests — each test gets a fresh `PgPool` with migrations applied. No manual setup/teardown
- Builder pattern in `tests/helpers/` for test data construction
- `assert2` crate: `assert!()` over `assert_eq!()`, `let_assert!()` for pattern matching, `check!()` for non-fatal assertions. Prefer `assert2` over `speculoos` — `speculoos` is verbose without providing the structural pattern-matching that `assert2::let_assert!` enables. For nested tree output (parse → scene → node), chain `let_assert!` steps into a readable traversal that fails fast on the first mismatch, with `check!()` for non-fatal leaf assertions
- **proptest roundtrip + panic safety for protocol codecs**: pair `parse(serialize(x)) == x` roundtrip tests with a separate never-panics invariant over arbitrary byte slices. These two properties together ensure both correctness and robustness for any binary parser
- **Nested test sub-modules**: `mod proptest_tests` inside `#[cfg(test)] mod tests` allows type-level organization without requiring separate files. Useful when a single module has both unit and property-based tests
- **Runtime hardware-feature gate for SIMD tests**: use `is_x86_feature_detected!` guard and early-return in the test body (not `#[cfg]`) for hardware-dependent tests. Keeps tests compiled and visible on all machines, avoiding both false failures and CI configuration complexity
- **Temp-file recording sink for structured output tests**: write events to a named tempfile (using `process::id()` for collision avoidance), drop the sink to flush, then parse NDJSON lines as `serde_json::Value` for field-level assertions
- **ECS World fixture builders**: for Bevy ECS tests, use `create_*_world()` to initialize required resources, named `spawn_*_entity()` helpers to abstract component bundles, and a `run_*_system()` driver to encapsulate system invocation. Each test body reads as Arrange → Act → Assert against a single variable
- **testcontainers + typed builder for app-state integration tests**: combine `testcontainers` (isolated DB) with a typed builder (e.g. `bon::builder`) to compose full app state (router, auth, health). Gate behind a Cargo feature (`postgres-tests`) so CI runs both fast in-memory and full Postgres variants. Complements `#[sqlx::test]` by covering router/extractor/auth layers

### TypeScript

- Vitest as the test framework. Named projects for different test environments in the same `vite.config.ts`
- Storybook stories as browser component tests via `@storybook/addon-vitest` — stories run with Playwright in headless Chromium alongside jsdom unit tests
- Playwright for E2E tests
- MSW for API mocking when backend is unavailable
- Global Vitest `setupFiles` recording sink: configure logger once with recording sink, auto-clear via `beforeEach`, export query helpers by category/level for asserting on structured log output
- EngineTestHarness factory for browser API mocking: single factory installs all global mocks (Worker, ResizeObserver, HTMLAudioElement), returns cleanup closure, each mock exposes typed `Controls` interface
- CI compat env-var gate: `test.skipIf(!available)` locally, hard throw when `CI_COMPAT=1`. Prevents both false negatives (skipped in CI) and false positives (failing locally)
- Stale smoke test anti-pattern: smoke tests must use valid fixture data matching the actual schema. Identity-function wrappers like `defineConfig` won't catch wrong shapes. Smoke tests that only assert element presence (e.g., `expect(canvas).not.toBeNull()`) are fragile proxies for real behavior — test state transitions instead

### Go

- Options-struct builder for test factories: pointer fields for selective override, atomic sequence counters for collision-free defaults, `t.Helper()` + `t.Fatal` for single-line call sites. Factories call real DB queries, no mocks
- pgtestdb + template-database cloning: per-test Postgres isolation. Single `NewEnv(t)` wires full stack (pool → queries → services → handler) for integration tests. Fast due to template cloning. CI env-var port-switching (`os.Getenv("CI")`) to select between local dev port and standard CI postgres is simpler than build-tag gating
- **`openTestStore(t)` factory for embedded K/V stores**: for bbolt, pebble, etc., create the DB in `t.TempDir()` and register cleanup via `t.Cleanup`. Single-line call sites, zero teardown boilerplate — analogous to pgtestdb template cloning for embedded stores
- **`countingHandler` slog test helper**: implement `slog.Handler` with `atomic.Int32` counting log records matching a pattern. Thread-safe, composable with any inner handler. Asserts log emission without full log capture infrastructure

### Kotlin

- Kotest DescribeSpec for nested scenario grouping in algorithm tests, `checkAll`/`Arb` for property-based testing. JUnit @Test and DescribeSpec coexist in same Gradle suite
- TestWorldBuilder DSL for spatial fixtures: high-level spatial primitives (floor, wall, column) instead of coordinate arrays. Overwrite guard via `check()` on duplicate positions

## Anti-Patterns

- Testing implementation details (private methods, internal state)
- Mocking everything — especially the database (mocks drift from reality)
- Flaky time-dependent tests without deterministic clocks
- Test-per-method instead of test-per-behavior

## Open Questions

- Mutation testing adoption and practical value
