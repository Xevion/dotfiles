---
name: testing-quality
category: patterns
last_audited: 2026-03-26
exemplars: []
---

# Testing & Quality

## Philosophy

<!-- TDD when infra exists, property tests for invariants, integration tests over mocks, test behavior not implementation -->

## Conventions

<!-- Test naming describes behavior, arrange-act-assert structure, fixtures for shared setup, snapshot tests sparingly -->

## Language-Specific

### Rust
<!-- cargo nextest, assert2 macros, proptest for property testing, inline unit tests + tests/ for integration -->

### TypeScript
<!-- Vitest preferred, Playwright for E2E, test factories over fixtures, MSW for API mocking -->

### Go
<!-- Table-driven tests, testify assertions, httptest for HTTP, go test -race -->

## Anti-Patterns

<!-- Testing implementation details, mocking everything, flaky time-dependent tests, test-per-method instead of test-per-behavior -->

## Open Questions

<!-- Mutation testing adoption, AI-assisted test generation quality -->
