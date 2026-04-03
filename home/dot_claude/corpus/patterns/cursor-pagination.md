---
name: cursor-pagination
category: patterns
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/motophoto
    path: internal/server/pagination.go
    note: "Base64-encoded cursor with (sort_order, id) tuple, max-limit cap, bidirectional navigation"
---

# Cursor Pagination

## Philosophy

Opaque cursors over offset-based pagination. Cursors are stable across concurrent inserts/deletes, perform well on large datasets (no `OFFSET` scan), and don't leak implementation details to the client.

## Conventions

- **Opaque cursor encoding**: encode a `(sort_value, id)` tuple as base64 to produce an opaque cursor string. The client passes this cursor back; the server decodes and validates it. Never expose raw database IDs or offsets in the cursor
- **Keyset WHERE clause**: use `WHERE (sort_col, id) > (cursor_sort, cursor_id) ORDER BY sort_col, id LIMIT N+1` for forward pagination. The `N+1` trick detects whether more pages exist without a separate COUNT query
- **Max-limit cap**: enforce a server-side maximum page size (e.g., 100). If the client requests more, silently cap to the maximum. Never allow unbounded result sets
- **Cursor validation**: decode and validate the cursor before executing the query. Return a 400 error for malformed cursors rather than falling back to the first page (which would silently reset pagination)

## Anti-Patterns

- Offset-based pagination (`OFFSET N`) on large or frequently-mutated tables — performance degrades linearly with page depth and results shift when rows are inserted/deleted. OFFSET is acceptable for small, stable tables (e.g., admin lists with <1K rows) where the simplicity tradeoff is worth it
- Exposing raw database IDs in pagination tokens
- No limit cap — clients can request millions of rows
- Silently resetting to page 1 on invalid cursor (masks client bugs)

## Open Questions

- Cursor pagination with complex sort orders (multi-column, mixed direction)
