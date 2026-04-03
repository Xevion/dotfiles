---
name: integration-test-db-isolation
category: patterns
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/motophoto
    path: internal/testutil/
    note: "pgtestdb template cloning, NewEnv(t) wiring, dbfactory row factories"
  - repo: Xevion/rustdoc-mcp
    path: tests/common/mod.rs
    note: "IsolatedWorkspace + TempWorkspace two-layer fixture, with_deps parameterization"
  - repo: Xevion/recall
    path: tests/helpers.ts
    note: "createTestDb() in-memory DuckDB factory, zero teardown"
---

# Integration Test Database Isolation

## Philosophy

Integration tests hit real databases, not mocks. Each test gets an isolated database instance so tests can run in parallel without interference. The isolation mechanism should be invisible to the test body — a single factory call returns a ready-to-use connection.

## Conventions

- **Per-test database isolation**: each test gets its own database instance (or schema) so writes in one test never affect another. The isolation mechanism varies by database engine but the interface is always a single factory call
- **Template cloning for PostgreSQL**: use `pgtestdb` (Go) or equivalent to clone a template database with migrations pre-applied. The template is created once per test suite; each test gets a fast clone. This is orders of magnitude faster than running migrations per test
- **In-memory embedded databases for lightweight stores**: for DuckDB, SQLite, bbolt, etc., create the database in `t.TempDir()` (Go) or in-memory mode (TypeScript). Register cleanup via `t.Cleanup()` or let the test framework handle teardown
- **Two-layer fixture composition**: separate filesystem isolation (temp directories, copied fixtures) from domain-specific state (database connections, service instances). The filesystem layer is reusable across test types; the domain layer composes it with app-specific setup. Example: `TempWorkspace` (filesystem) + `IsolatedWorkspace` (adds doc generation state)
- **Factory functions with sensible defaults**: test row factories (e.g., `dbfactory.Event(ctx, t, pool, service, &opts)`) insert rows with sensible defaults. Only the fields relevant to the test scenario are overridden via an options struct with pointer fields
- **CI port switching**: use `os.Getenv("CI")` or equivalent to switch between local dev Postgres port and standard CI port. Simpler than build-tag gating

## Language-Specific

### Go

- `pgtestdb` with `goosemigrator` for Postgres template cloning. `testutil.NewEnv(t)` wires pool → queries → services → HTTP handler in one call. `testutil.LoginPhotographer(t, handler, pool)` creates authenticated test sessions

### Rust

- `IsolatedWorkspace` copies rustdoc JSON to a temp directory for full test isolation. `with_deps(&["crate1", "crate2"])` parameterizes which external dependencies are available. Unit tests (`cargo nextest run --lib`) always pass without external setup

### TypeScript

- `createTestDb()` creates an in-memory DuckDB with schema initialized. Single-line call sites, zero teardown boilerplate. The TypeScript equivalent of Go's `openTestStore(t)` pattern

## Anti-Patterns

- Shared database across tests (order-dependent failures, flaky parallelism)
- Mocking the database layer (mock/prod divergence masks real bugs)
- Running full migrations per test when template cloning is available
- Silent test skips when the database is unavailable (use CI env-var gates)

## Open Questions

- testcontainers vs template cloning trade-offs for different database engines
