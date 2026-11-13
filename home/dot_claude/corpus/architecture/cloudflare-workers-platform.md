---
name: cloudflare-workers-platform
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/stashflare
    path: vitest.config.ts, Justfile (smoke), deploy script, D1 migrations
    note: "Workers with D1 and R2: a smoke test of the built worker gates deploy, test workers declared in the test config, migrations through wrangler"
---

# Cloudflare Workers Platform

Source: one project, stashflare. Observed practice.

## Philosophy

Node-based tests and type checking cannot see failures that exist only in the Workers runtime. Gate deploys on a test of the built worker, and keep test bindings away from production.

## Conventions

- **A smoke test of the built worker gates deploy**: `"deploy": "just smoke && bun run db:migrate && wrangler deploy"`. It catches workerd-only failures that Node tests and typechecking miss
- **Declare test Workers in the test config, never from `wrangler.jsonc`**, because remote bindings would run the suite against production. Set `remoteBindings: false` on purpose and keep it spelled out. Run two Vitest projects: `*.test.ts` in Node and `*.db.test.ts` in a Worker
- **Migrations go through `wrangler d1 migrations apply`**, never `drizzle-kit migrate` (it swallows errors on the `d1-http` driver) and never `db:push`
- **Hand-authored SQLite objects** (FTS5 tables and triggers) are kept as `generate --custom` migrations. Do not use external-content FTS, and never change a primary key or widen a column in a way that forces a table rebuild, because a rebuild drops the triggers (SQLite has no `ALTER COLUMN`)
- **Keep `disallow_eval_during_startup` in `compatibility_flags`**
- **Use `platform.cf` for geolocation** instead of a geo-IP service
- **Audit and access writes are best-effort** through `waitUntil`, so they never delay or fail a request
- **SvelteKit specifics**: `router.resolution: 'server'`, and plan for bfcache and for chunk 404s after a redeploy

## Anti-Patterns

- Trusting Node-only tests for Workers code
- Test configuration reaching production bindings
- Table rebuilds that silently drop hand-authored triggers

## Related Topics

- [data-modeling](./data-modeling.md)
- [object-storage-patterns](./object-storage-patterns.md)
- [security-auth](../patterns/security-auth.md)
