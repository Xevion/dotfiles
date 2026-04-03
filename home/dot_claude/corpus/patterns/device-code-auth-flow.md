---
name: device-code-auth-flow
category: patterns
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/glint
    path: backend/src/routes/ + mod/common/src/.../api/
    note: "Rust backend device code issuance + polling, Kotlin client with typed ApiError states, dual-source sessions"
---

# Device Code Auth Flow

## Philosophy

RFC 8628 for headless clients that cannot host a redirect URI. The correct pattern for CLIs, game mods, TV apps, and embedded devices. The device displays a user code; the user authenticates in a browser. The device polls until authorization completes or expires.

## Conventions

- **Backend device code issuance**: `POST /api/device/authorize` returns `{ deviceCode, userCode, verificationUri, expiresIn, interval }`. The device code is a high-entropy opaque token; the user code is a short alphanumeric string for display. `expiresIn` sets the absolute deadline

- **Polling endpoint with typed error states**: `POST /api/device/token` accepts the device code. Returns typed error strings matching RFC 8628: `authorization_pending` (user hasn't authorized yet), `expired_token` (device code expired), `invalid_grant` (revoked or invalid). Rate limiting via `429` with `Retry-After` header. On success, creates a session and returns a session token

- **Dual-source session model**: sessions carry a `source` field (`"web"` for browser login, `"device"` for mod/CLI auth). This enables per-source audit trails and policy decisions (e.g., different session TTLs for device vs web). The source is stored in the session record at creation time

- **Client polling with typed error variants**: the polling client maps each RFC 8628 error string to a typed error variant (e.g., `ApiError.AuthorizationPending`, `ApiError.TokenExpired`, `ApiError.RateLimited(retryAfterSeconds)`). Parse `Retry-After` from headers into the typed variant. The UI drives the polling loop

## Language-Specific

### Rust

- Backend endpoints under a `/api/device/` route group. Device codes stored in a time-limited store (database row or in-memory with TTL). The token exchange endpoint creates a session with `source = "device"` and returns the session cookie or bearer token

### Kotlin

- `AuthClient.pollDeviceToken()` runs in a background `CompletableFuture` off the game thread. Each typed `ApiError` subclass carries domain-specific state (`retryAfterSeconds`, `userMessage`). The UI screen (`DeviceAuthScreen`) displays the user code and verification URL while polling runs asynchronously

## Anti-Patterns

- **Fixed-interval polling without backoff**: always respect the `interval` and `Retry-After` header. Polling faster than the server allows wastes resources and may get the client blocked
- **Embedding browser auth in headless environments**: opening a browser or webview from a game mod or CLI defeats the purpose. Display the code and URL for the user to open themselves
- **Long-lived device codes without expiry**: device codes must expire (typically 5-15 minutes). Expired codes return `expired_token` so the client can restart the flow
- **Storing session tokens in plaintext config files**: device auth tokens should be stored securely (system keychain, encrypted file) with appropriate file permissions

## Open Questions

- PKCE integration with device flow for additional security
- Multi-device session management (revoking device sessions from web UI)
- User code format standardization (length, character set, grouping with hyphens)
