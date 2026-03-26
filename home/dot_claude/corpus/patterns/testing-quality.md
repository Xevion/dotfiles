---
name: testing-quality
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: tests/
    note: sqlx::test with builder fixtures, Vitest named projects (unit + storybook browser tests)
---

# Testing & Quality

## Philosophy

TDD when test infrastructure exists. Integration tests over mocks — hit real databases, real APIs. Test behavior, not implementation. Property tests for invariants.

## Conventions

- **Test naming**: describes the behavior being tested, not the method name
- **Arrange-Act-Assert**: clear three-phase structure in every test
- **Builder patterns for fixtures**: test helpers with sensible defaults (`Default::default()`) so test bodies focus on the scenario, not fixture construction

## Language-Specific

### Rust

- `cargo nextest` as the test runner (parallel, better output than `cargo test`)
- `#[sqlx::test]` for database integration tests — each test gets a fresh `PgPool` with migrations applied. No manual setup/teardown
- Builder pattern in `tests/helpers/` for test data construction
- `assert2` crate: `assert!()` over `assert_eq!()`, `let_assert!()` for pattern matching, `check!()` for non-fatal assertions

### TypeScript

- Vitest as the test framework. Named projects for different test environments in the same `vite.config.ts`
- Storybook stories as browser component tests via `@storybook/addon-vitest` — stories run with Playwright in headless Chromium alongside jsdom unit tests
- Playwright for E2E tests
- MSW for API mocking when backend is unavailable

### Go

<!-- Placeholder: table-driven tests, testify, httptest, go test -race -->

## Anti-Patterns

- Testing implementation details (private methods, internal state)
- Mocking everything — especially the database (mocks drift from reality)
- Flaky time-dependent tests without deterministic clocks
- Test-per-method instead of test-per-behavior

## Open Questions

- Mutation testing adoption and practical value
- AI-assisted test generation quality and when to trust it
