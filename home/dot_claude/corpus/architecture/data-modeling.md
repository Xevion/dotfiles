---
name: data-modeling
category: architecture
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/banner
    path: migrations/
    note: JSONB sub-entities, materialized views, safe CHECK constraint migrations
  - repo: Xevion/doujin-ocr-summary
    path: internal/database/ + sqlc.yml
    note: sqlc with json.RawMessage JSONB overrides, goose sequential migrations
---

# Data Modeling

## Philosophy

Schema-first — the database schema is the source of truth. Normalize first, denormalize intentionally for read performance. Migrations are first-class artifacts, not afterthoughts.

## Conventions

- **Migration versioning**: timestamp-prefixed, one logical change per file, never modify after application
- **JSONB for volatile sub-entities**: use JSONB arrays for 1-to-many data whose shape changes frequently and is rarely filtered individually (e.g. meeting times, attribute lists). Keep enumerable filter columns (campus, method) as plain `VARCHAR` with btree indexes
- **Materialized views for read-heavy aggregations**: precompute expensive joins/aggregations. Add a UNIQUE index on the grouping key to enable `REFRESH CONCURRENTLY`. Refresh explicitly after mutations
- **Safe constraint migrations**: when adding CHECK constraints to tables with existing data, include (1) a data-repair step for historical invalid values and (2) a validation block that raises an exception if any rows still violate the constraint
- **Table consolidation migration**: when backfilling from a related table before applying NOT NULL, the sequence is (1) add columns nullable, (2) backfill via `UPDATE...FROM` join, (3) apply NOT NULL constraints, (4) optionally drop the source table. This is a superset of the CHECK constraint pattern

```sql
-- Pattern: safe CHECK constraint addition
UPDATE courses SET enrollment = 0 WHERE enrollment < 0;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM courses WHERE enrollment < 0) THEN
        RAISE EXCEPTION 'courses contains negative enrollment values';
    END IF;
END $$;
ALTER TABLE courses ADD CONSTRAINT chk_enrollment_nonneg CHECK (enrollment >= 0);
```

## Language-Specific

### Rust

- **SQLx compile-time checked queries**: use `sqlx::query!` / `sqlx::query_as!` with offline mode (`.sqlx/` metadata). `QueryBuilder<Postgres>` for dynamic conditions
- **JSONB in Rust**: `sqlx::types::Json<Vec<T>>` or `Json<Struct>`, never `Option<String>`. Ensures ts-rs generates `Array<T>` in TypeScript, not `string`

### TypeScript

<!-- Placeholder: Drizzle/Prisma patterns, migration tooling -->

### Go

- **sqlc compile-time checked queries with `encoding/json.RawMessage` type overrides**: use type overrides in `sqlc.yml` to map JSONB columns to `json.RawMessage` rather than `[]byte`. Avoids double serialization while maintaining type safety for known-shape columns. Pair with `tstype` override for TypeScript output
- **One logical change per migration file**: mixing unrelated schema evolutions in a single file makes rollback and bisecting harder. Each migration file should describe exactly one intentional change
- **bbolt for embedded state persistence**: for CLI tools and collectors that need durable state across restarts without a separate database process, bbolt with raw JSON byte values per named bucket is a lightweight alternative to SQLite/sqlc. Name buckets as package-level byte slice constants. Store raw JSON and defer deserialization to the caller to keep the store interface generic. Ensure bucket existence at Open time via a single `db.Update`

- **Versioned migration files over ORM auto-migrate**: adopt golang-migrate or goose with numbered SQL migration files. Each schema change becomes a file with up/down steps. `AutoMigrate` (GORM) is acceptable for rapid prototyping but cannot drop columns, reorder constraints, or express data backfills as first-class steps. Once schema has real production data, migration files are mandatory

## Anti-Patterns

- Schemaless by default ("we'll figure out the schema later")
- **GORM AutoMigrate as sole migration strategy**: `AutoMigrate` on every startup with hand-rolled backfill functions is a maintenance liability. It cannot express destructive changes, rollbacks, or migration ordering
- EAV (Entity-Attribute-Value) tables
- Non-idempotent migrations
- Storing structured data as JSON-encoded TEXT columns

## Open Questions

- When materialized views stop scaling and whether a dedicated read store (CQRS-style) is worth the operational complexity for a solo developer
