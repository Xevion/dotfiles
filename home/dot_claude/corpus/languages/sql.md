---
name: sql
category: languages
last_audited: 2026-04-10
exemplars:
  - repo: Xevion/banner
    path: migrations/
    note: 30+ PostgreSQL migrations demonstrating JSONB, tsvector, materialized views, UNLOGGED tables
  - repo: Xevion/doujin-ocr-summary
    path: internal/database/migrations/
    note: goose migrations with sequential numeric prefixes, sqlc-generated Go types
  - repo: local/stashapp-rapid-tag
    path: src/lib/server/db.ts + migrations/
    note: "better-sqlite3 startup migration runner, WAL + foreign_keys + busy_timeout pragmas, sequential 001_/002_/003_ files"
---

# SQL

## Philosophy

Migration discipline — schema is the source of truth. Normalize first, denormalize intentionally. Migrations are first-class artifacts that document schema evolution.

## Conventions

- **Naming**: `snake_case` for tables and columns, plural table names (`courses`, `instructors`), explicit `JOIN` types
- **Migration versioning**: timestamp-prefixed filenames (`20260128000000_description.sql`), one logical change per migration. Sequential numeric prefixes (goose default: `00001_`, `00002_`) are also valid — prefer sequential for solo projects to avoid clock-skew; reserve timestamps for multi-team repos with concurrent migration creation. Choose a consistent zero-pad width upfront (recommend 4-digit `0001_` for projects likely to exceed 100 migrations) — switching widths mid-project creates visual inconsistency and risks lexicographic mis-sort.
- **JSONB for volatile 1-to-many data**: when sub-entities change shape frequently and you rarely filter by individual sub-fields, use JSONB arrays instead of join tables. Add GIN indexes for containment queries

```sql
ALTER TABLE courses ADD COLUMN meeting_times JSONB NOT NULL DEFAULT '[]'::jsonb;
CREATE INDEX idx_courses_meeting_times ON courses USING GIN (meeting_times);
```

- **Full-text search**: generated `tsvector` columns (`GENERATED ALWAYS AS ... STORED`) with GIN indexes for search. `pg_trgm` GIN for substring/ILIKE patterns. Create indexes in the same migration as their columns
- **Materialized views for precomputed aggregations**: when aggregations are expensive and read-heavy but write-infrequent. Add a UNIQUE index on the grouping key to enable `REFRESH CONCURRENTLY`
- **UNLOGGED TABLE for ephemeral state**: scheduler timestamps, cache entries, bot command fingerprints — data where crash-loss is acceptable. No WAL overhead. Document the tradeoff in a migration comment
- **Safe CHECK constraint migrations**: include a data-repair step (UPDATE to fix invalid rows) and a validation block (`DO $$ ... RAISE EXCEPTION ... $$`) before adding the constraint. Even when data is believed clean, include the validation block — a comment asserting data cleanliness is acceptable documentation, but the validation block catches silent drift.

## TypeScript + SQLite

For SvelteKit/Bun server code that needs a local SQL database, `better-sqlite3` (synchronous) is the correct choice over `bun:sqlite` when the surface area is non-trivial — the synchronous API gives proper type inference, idiomatic transaction handling, and mature ecosystem tooling.

- **Startup migration runner with `_migrations` tracking table**: on first `getDb()` call, open the database, create `_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMP)` if it doesn't exist, read `migrations/*.sql` sorted lexicographically, compute the set of unapplied files, and execute them via `db.exec()` inside individual transactions. Idempotent by construction — re-running the app on an up-to-date database is a no-op
- **Sequential numeric prefix naming for SQLite migrations**: `001_initial.sql`, `002_add_users.sql`, etc. Pad to at least 3 digits. Timestamp prefixes are unnecessary for single-writer SQLite projects
- **Open-time pragmas for all SQLite databases**: `PRAGMA journal_mode = WAL` (concurrent readers with single writer), `PRAGMA foreign_keys = ON` (off by default — a footgun), `PRAGMA busy_timeout = 5000` (wait 5s on lock contention before throwing). Apply in `getDb()` before returning the connection
- **Query result typing via local interface casts**: since better-sqlite3 results are `unknown`, cast per-query with narrow inline interfaces (`.get(id) as { count: number }`). Acceptable for internal queries where the schema is controlled — flag at user-input boundaries where Zod validation is required

## Anti-Patterns

- `SELECT *` in application queries — acceptable in sqlc for simple single-table lookups where the full row is consumed, but avoid for JOIN queries (column name collisions, ambiguous types)
- Implicit joins (comma-separated FROM)
- Business logic in stored procedures
- Schema changes without migrations
- Modifying applied migration files (SQLx tracks checksums)

## Related Topics

- [data-modeling](../architecture/data-modeling.md) — JSONB sub-entities, materialized views, and migration strategy at the architecture level

## Open Questions

- ORM vs query builder vs raw SQL decision criteria per project size
- Migration tool preferences per stack: SQLx (Rust), goose (Go), Drizzle (TS), golang-migrate (Go)
