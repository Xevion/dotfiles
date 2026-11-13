---
name: device-code-auth-flow
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/xevion.dev
    path: cli_auth registry (device start/events/approve), auth session manager
    note: "SSE completion from an in-memory watch channel, re-entered user code on approval, SHA-256 token hash with an in-memory index"
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

- **How completion reaches the device is a transport choice, not part of the flow.** Polling is the simplest and works on any headless client. Server-sent events (the server holds a stream open and pushes `approved` or `denied`) avoid polling latency and are preferred when the client can hold a connection, at the cost of more server state (xevion.dev keeps a watch channel per pending code). Long-polling (hold the response open until it resolves or times out) is similar to SSE without being a standard, and is weaker in browsers. WebSockets work but are more than a one-way notification needs. Choose by whether the client can hold a connection; the typed states (`pending`, `approved`, `denied`, `expired`) stay the same on every transport

- **The approver confirms the code the device displays.** The approval page asks the person to type the user code shown on their own device, and refuses a mismatch (case-insensitively). This blocks the phishing case where an attacker starts a login on their own machine and sends the victim a link: a victim who never started a login has no code to type

- **Store only a hash of the long-lived CLI token server-side.** Keep a SHA-256 of the bearer token in the session row and resolve it through an in-memory index from hash to session. The plaintext token exists only in the approval payload and the client's config file

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
