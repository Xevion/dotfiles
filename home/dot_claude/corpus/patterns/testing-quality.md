---
name: testing-quality
category: patterns
last_audited: 2026-04-03
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
  - repo: Xevion/rustdoc-mcp
    path: tests/common/mod.rs
    note: "IsolatedWorkspace two-layer fixture (TempWorkspace + DocState), rstest fixtures, assert2 throughout"
  - repo: Xevion/recall
    path: tests/
    note: "createTestDb() in-memory DuckDB factory, behavioral triage tests with shared fixture defaults"
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
- **Two-layer fixture composition for MCP/server tests**: compose test isolation from two layers — a lightweight filesystem fixture (`TempWorkspace`) for cold-cache scenarios plus a stateful fixture (`IsolatedWorkspace`) wrapping it with domain-specific setup (doc generation state, cache seeding). `with_deps(&[...])` parameterizes external dependency inclusion. This is "containerless integration test isolation" — the Rust equivalent of pgtestdb template cloning without requiring an external database process
- **testcontainers + typed builder for app-state integration tests**: combine `testcontainers` (isolated DB) with a typed builder (e.g. `bon::builder`) to compose full app state (router, auth, health). Gate behind a Cargo feature (`postgres-tests`) so CI runs both fast in-memory and full Postgres variants. Complements `#[sqlx::test]` by covering router/extractor/auth layers

### TypeScript

- **Vitest dual-project setup**: configure two named test projects in `vite.config.ts` — (1) `unit` with jsdom environment for fast component/logic tests, (2) `storybook` with Playwright browser environment for visual Storybook tests via `@storybook/addon-vitest`. Both run from a single `vitest` invocation. Used in banner and glint
- Storybook stories as browser component tests — stories run with Playwright in headless Chromium alongside jsdom unit tests
- Playwright for E2E tests
- MSW for API mocking when backend is unavailable
- Global Vitest `setupFiles` recording sink: configure logger once with recording sink, auto-clear via `beforeEach`, export query helpers by category/level for asserting on structured log output
- EngineTestHarness factory for browser API mocking: single factory installs all global mocks (Worker, ResizeObserver, HTMLAudioElement), returns cleanup closure, each mock exposes typed `Controls` interface
- CI compat env-var gate: `test.skipIf(!available)` locally, hard throw when `CI_COMPAT=1`. Prevents both false negatives (skipped in CI) and false positives (failing locally)
- **`// @vitest-environment node` for real-network integration tests**: per-file environment override bypasses happy-dom's fake fetch to enable real HTTP calls. Distinct from the CI env-var gate — the gate is for tests that may be unavailable, while the environment override is for tests requiring real network semantics. Combine both when needed
- **Behavioral invariant testing for math/scoring functions**: test ordering (at1 > at2 > at5), symmetry (ratio 2 ≈ ratio 0.5), bounds, and monotonicity properties rather than pinning exact float values. More resilient to algorithm tuning than spot-checking specific outputs
- **Nested describe by input category**: group tests by input type rather than method under test, with explicit "should NOT" negative cases for parser edge coverage. Effective for type-detection, format-parsing, and other disambiguation logic
- Stale smoke test anti-pattern: smoke tests must use valid fixture data matching the actual schema. Identity-function wrappers like `defineConfig` won't catch wrong shapes. Smoke tests that only assert element presence (e.g., `expect(canvas).not.toBeNull()`) are fragile proxies for real behavior — test state transitions instead

### Go

- Options-struct builder for test factories: pointer fields for selective override, atomic sequence counters for collision-free defaults, `t.Helper()` + `t.Fatal` for single-line call sites. Factories call real DB queries, no mocks
- Database isolation patterns (pgtestdb, in-memory DuckDB, embedded K/V store factories): see [integration-test-db-isolation](./integration-test-db-isolation.md) for the full pattern catalog
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
