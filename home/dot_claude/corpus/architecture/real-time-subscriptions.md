---
name: real-time-subscriptions
category: architecture
last_audited: 2026-03-26
exemplars: []
---

# Real-Time Subscriptions

## Philosophy

<!-- Push state changes to clients rather than polling. Typed domain events over generic notifications. Connection state is a first-class UI concern. -->

## Conventions

<!-- GraphQL WS subscriptions, typed DomainEvent enum over broadcast channel, frontend connectivity state management, reconnection with backoff -->

## Language-Specific

### Rust

<!-- tokio broadcast channel for fan-out, async-graphql subscription resolvers, filter by topic/entity -->

### TypeScript

<!-- urql subscription exchange, graphql-ws transport, ConnectivityBanner for connection state -->

## Anti-Patterns

<!-- Polling disguised as subscriptions, unbounded event buffers, no reconnection strategy -->

## Open Questions

<!-- SSE vs WebSocket tradeoffs, subscription scaling patterns, event sourcing integration -->
