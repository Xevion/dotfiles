---
name: security-auth
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: local/inkwell
    path: web/src/hooks.server.ts
    note: Header allowlist for multi-hop proxy chain with per-header provenance comments
  - repo: Xevion/xevion.dev
    path: src/auth.rs
    note: "Hybrid in-memory/DB session management with DashMap, Argon2, ULID session IDs"
  - repo: Xevion/Pac-Man
    path: pacman-server/src/session.rs
    note: Stateless PKCE via JWT claims — PKCE verifier and CSRF state embedded as custom claims
---

# Security & Auth

## Philosophy

Zero-trust defaults. Validate at system boundaries. Secrets never in code — use secret managers (Doppler, Vault). Token rotation discipline.

## Conventions

- **Header allowlist for multi-hop proxy chains**: define the set of forwarded request headers as a typed const tuple, document each header's provenance in comments, and forward only the allowlist to prevent arbitrary header injection from reaching the backend
- **Hybrid in-memory/DB session management**: DashMap as the hot validation path (zero-latency lookups), PostgreSQL as durable store. Hydrate on startup, write-through on create/delete, periodic cleanup via `cleanup_expired()`. Use ULID session IDs (time-ordered, URL-safe). Suitable when session count is small (admin-only) and zero-latency validation matters
- **Stateless PKCE via JWT claims**: embed PKCE verifier and CSRF state as custom claims inside the session JWT instead of storing them in server-side state. Makes OAuth flow stateless at the cost of embedding sensitive ephemeral material in a cookie-transported token. Appropriate for single-server deployments; requires careful key management

## Language-Specific

### Rust

- Argon2 for password hashing, tower middleware for auth extraction, typed permission extractors

### TypeScript

- SvelteKit hooks.server.ts as the natural enforcement point for header filtering in SvelteKit+backend proxy chains

### Go

- `crypto/subtle` for constant-time comparisons, middleware chains for auth, `context.Context` for user propagation
- **Bearer token confinement in `http.RoundTripper`**: inject the bearer token inside `RoundTrip()` so the token never appears in call-site code, log fields, or error messages. Store the token in the transport struct, set the header once in `RoundTrip()`, and validate at startup (error if empty) before any requests fire

## Anti-Patterns

- Secrets in env files committed to git
- Rolling your own crypto
- Client-side-only auth checks
- Session cookies without the `Secure` flag when deployed behind TLS termination — `SameSite=Lax` alone is insufficient; `Secure` prevents the cookie from being sent over HTTP even if an attacker can downgrade the connection. Always set `.secure(true)` in production; gate on a config flag rather than hardcoding

## Open Questions

- Token rotation cadence and session expiry strategies
- OAuth provider configuration patterns across deployments
