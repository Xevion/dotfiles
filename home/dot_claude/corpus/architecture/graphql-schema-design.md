---
name: graphql-schema-design
category: architecture
last_audited: 2026-03-26
exemplars: []
---

# GraphQL Schema Design

## Philosophy

<!-- Code-first schema generation from Rust/Go types. Schema is a compiled artifact, not a hand-maintained file. -->

## Conventions

<!-- DataLoader for N+1 avoidance, cursor-based pagination via Connection types, compile-time schema verification in CI -->

## Language-Specific

### Rust (async-graphql)

<!-- SimpleObject vs Object, ErrorExtensions for typed error codes, subscription via broadcast channels -->

### TypeScript (gql-tada + urql)

<!-- gql-tada for compile-time typed documents, urql for client with exchange pipeline, graphql-ws for subscriptions -->

## Anti-Patterns

<!-- Over-fetching with deep nesting, N+1 without DataLoader, schema-first with manual resolvers -->

## Open Questions

<!-- Federation/stitching for multi-service GraphQL, persisted queries, schema evolution strategy -->
