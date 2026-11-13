---
name: postgres-operations
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/bf4-emblems
    path: db migrations and repo claim methods, ingest writer
    note: "Index-before-migration ordering, claim leases with an attempts counter, write coalescing, per-table vacuum, lock-ordered upserts"
  - repo: Xevion/banner
    path: backup CronJob template in the Helm chart
    note: "Dump, verify, upload, retain, with a Forbid concurrency policy"
---

# Postgres Operations

Source: bf4-emblems and banner. Observed practice for a busy Postgres.

## Philosophy

Treat the database as a shared resource with a boot path, a write rate and a backup story. Order schema changes around transactions, lease work instead of stamping it, and measure before tuning.

## Conventions

- **Build large indexes `CONCURRENTLY` before the migration that declares them `IF NOT EXISTS`.** Migrations run in a boot transaction, and an index built there is startup downtime. Never renumber an applied migration, since the migration table keys on version
- **Sort multi-row upsert batches** so row locks are taken in a consistent order (`INSERT ... ON CONFLICT` takes locks in array order), avoiding deadlocks
- **Lease work instead of stamping it.** A `claimed_at` timestamp with a reclaim TTL in the claim predicate, and an `attempts` counter incremented on every claim and cleared on success. A claim inserts the row it leases when none exists. Keep each claim to one statement per batch. The attempts counter exposes a stuck queue that depth alone hides (depth cannot tell "waiting" from "re-served forever")

```sql
UPDATE players SET claimed_at = now(), attempts = attempts + 1
WHERE id = ANY($1) AND (claimed_at IS NULL OR claimed_at < now() - make_interval(secs => $2))
```

- **Coalesce writes.** Completions flow through a bounded channel into one writer that flushes on a time window; commits fell from about 410 per second to about 201 WAL syncs per second. See [concurrency-async](./concurrency-async.md)
- **Tune the churn-heavy tables.** WAL and checkpoint cadence, per-table autovacuum scale factors, and `statement_timeout` server-side for abandoned statements. `pg_dump` pins the xmin horizon, so keep scratch tables out of backups
- **Shard sitemaps by hash with `COLLATE "C"`**
- **Backup order: dump, verify, upload, retain.** Check the dump with `pg_restore --list` before upload, and run retention only after a confirmed upload so a failed dump never expires history. A Kubernetes CronJob with `concurrencyPolicy: Forbid`, a `startingDeadlineSeconds` bound, an init-container dump on an emptyDir and an upload container
- **Test isolation**: template clones, see [integration-test-db-isolation](../patterns/integration-test-db-isolation.md). **sqlx offline metadata** against a migrations-only database, see [build-systems](../project-structure/build-systems.md)

## Anti-Patterns

- An index built inside the boot transaction
- A stamp that a crashed worker leaves permanently claimed
- Retention before a verified upload

## Related Topics

- [data-modeling](./data-modeling.md)
- [scheduling](./scheduling.md)
- [sql](../languages/sql.md)
