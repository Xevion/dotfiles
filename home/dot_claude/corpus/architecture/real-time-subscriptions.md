---
name: real-time-subscriptions
category: architecture
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/borde.rs
    path: crates/borders-core/src/ui/protocol.rs
    note: "Dual-channel JSON+binary protocol, serde tag dispatch for events, 1-byte envelope for deltas"
---

# Real-Time Subscriptions

## Philosophy

Push state changes to clients rather than polling. Typed domain events over generic notifications. Connection state is a first-class UI concern. Use the right channel for the right data — JSON for structured events, binary for high-frequency state deltas.

## Conventions

- **Dual-channel architecture**: typed JSON messages via serde tag dispatch (`#[serde(tag = "msg_type")]`) for game state events, separate binary channel with a 1-byte type envelope for high-frequency territory/pixel delta updates. Avoids JSON overhead on the hot path while keeping the event protocol typed and readable
- **Typed message enums**: define `BackendMessage` and `FrontendMessage` as discriminated union enums. JSON channel uses serde tag field for dispatch; binary channel uses `repr(u8)` type byte with `from_u8` constructor
- **Reconnection with backoff**: exponential backoff on disconnection, state reconciliation on reconnect

## Language-Specific

### Rust

- tokio broadcast channel for fan-out, async-graphql subscription resolvers, filter by topic/entity
- `#[serde(tag = "msg_type")]` on message enums for JSON channel discriminant

### TypeScript

- urql subscription exchange, graphql-ws transport, ConnectivityBanner for connection state
- **Typed `on()` API for WebSocket events**: use `Extract<WebSocketEventType, { type: T }>` in the subscription function signature so `on("board_update", handler)` infers the correct event type without `as` casts. When a WebSocket event type union exists, thread it through the subscription API

### Go

- **Typed inbound message structs**: unmarshal into `struct{ Type string }` first, switch on `Type`, then unmarshal the full payload into the concrete type. Eliminates chained `map[string]interface{}` type assertions
- **Fan-out broadcaster with correct lock discipline**: when a broadcast loop needs to remove slow clients (channel full), collect slow clients under RLock and delete in a subsequent write Lock. Never delete from a map while holding an RLock

## Anti-Patterns

- Polling disguised as subscriptions
- Unbounded event buffers (use select/default with drop for slow clients)
- Parsing inbound WebSocket messages into `map[string]interface{}` with manual type assertions — define typed structs
- Untyped `on(string, handler)` APIs that force callers to cast `event as SpecificType`

## Open Questions

- SSE vs WebSocket tradeoffs for unidirectional push
- Subscription scaling patterns (sharding, partitioning)
