---
name: session-cookie-auth
category: patterns
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/motophoto
    path: internal/session/ + internal/middleware/
    note: "scs session manager backed by PostgreSQL, role-based middleware guards, pgtestdb integration tests"
---

# Session & Cookie Authentication

## Philosophy

Server-side session state with opaque cookie tokens. The server is the source of truth for session validity — no client-side JWT decoding or trust. Roles and permissions are checked at the middleware layer, not scattered across handlers.

## Conventions

- **Opaque session cookies**: session ID is a random, high-entropy token (ULID, UUID, or cryptographic random bytes — never a small integer like `u32`). The cookie carries only the session ID; all session data lives server-side
- **Database-backed session store**: sessions stored in PostgreSQL (or equivalent) with automatic expiry. Libraries like `scs` (Go) or `express-session` (Node) handle cookie serialization, rotation, and cleanup
- **Role-based middleware guards**: compose middleware functions that check session existence (`RequireAuth`) and role membership (`RequireRole(role)`) before the handler runs. Attach the authenticated user to the request context for downstream access via `UserFromContext(ctx)`
- **Session key convention**: store minimal data in the session (e.g., `"user_id"` as a string). Look up full user details from the database on each request rather than caching stale user data in the session
- **Integration tests with real login**: test auth flows by issuing actual login requests (`POST /api/auth/login`) and capturing the session cookie. Pass the cookie to subsequent requests. Never mock the session layer in integration tests

## Anti-Patterns

- Small token space (e.g., `rand::random::<u32>()`) — brute-forceable. Use at least 128-bit tokens
- Permanent cookies without server-side TTL or cleanup — sessions accumulate indefinitely
- Storing sensitive data in the cookie itself (use server-side session store)
- CORS wildcard (`*`) with session cookies — defeats same-origin protection

## Open Questions

- Cookie vs token trade-offs for mobile/native clients
- Session clustering strategies for horizontally scaled backends
