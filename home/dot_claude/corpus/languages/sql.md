---
name: sql
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: migrations/
    note: 30+ PostgreSQL migrations demonstrating JSONB, tsvector, materialized views, UNLOGGED tables
  - repo: Xevion/doujin-ocr-summary
    path: internal/database/migrations/
    note: goose migrations with sequential numeric prefixes, sqlc-generated Go types
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

## Anti-Patterns

- `SELECT *` in application queries — acceptable in sqlc for simple single-table lookups where the full row is consumed, but avoid for JOIN queries (column name collisions, ambiguous types)
- Implicit joins (comma-separated FROM)
- Business logic in stored procedures
- Schema changes without migrations
- Modifying applied migration files (SQLx tracks checksums)

## Open Questions

- ORM vs query builder vs raw SQL decision criteria per project size
- Migration tool preferences per stack: SQLx (Rust), goose (Go), Drizzle (TS), golang-migrate (Go)
