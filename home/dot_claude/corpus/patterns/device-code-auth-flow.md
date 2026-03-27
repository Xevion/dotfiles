---
name: device-code-auth-flow
category: patterns
last_audited: 2026-03-26
exemplars: []
---

# Device Code Auth Flow

## Philosophy

<!-- RFC 8628 for headless clients that cannot host a redirect URI. The correct pattern for CLIs, game mods, TV apps, and embedded devices. -->

## Conventions

<!-- Polling with exponential backoff, user code display format, session token exchange, dual-source session model (web vs device) -->

## Language-Specific

### Rust

<!-- Backend: device code issuance, polling endpoint, token exchange and session creation -->

### Kotlin

<!-- Client: user code display, background polling with CompletableFuture, token storage -->

## Anti-Patterns

<!-- Fixed-interval polling without backoff, embedding browser auth in headless environments, long-lived device codes without expiry -->

## Open Questions

<!-- PKCE integration with device flow, multi-device session management, code format standardization -->
