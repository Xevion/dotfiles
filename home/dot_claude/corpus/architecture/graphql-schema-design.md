---
name: graphql-schema-design
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/railway-collector
    path: internal/collector/loader/ + internal/railway/
    note: Runtime aliased-batch queries with breadth packing, genqlient codegen, Cloudflare workaround
  - repo: Xevion/glint
    path: backend/src/graphql/
    note: "async-graphql DataLoader N+1 prevention, BroadcastStream subscriptions with lag handling, gql-tada typed documents"
---

# GraphQL Schema Design

## Philosophy

Code-first schema generation from Rust/Go types. Schema is a compiled artifact, not a hand-maintained file. For consumer-side codegen (genqlient, gql-tada), the schema SDL is the source of truth.

## Conventions

- **Runtime aliased-batch queries**: when a codegen client (genqlient, gql-tada) cannot express a dynamic number of aliased fields, build query strings programmatically with per-alias variable namespacing (suffix alias key to all var names). Pack greedily, unpack by walking `resp.Data[alias]` to route results back to originators

```go
// Pattern: runtime query assembly with variable namespacing
func (r *Request) AssembleQuery() (string, map[string]any) {
    for _, f := range r.Fragments {
        aliases = append(aliases, fmt.Sprintf("%s: %s(%s) %s", f.Alias, f.Field, f.Args, f.Selection))
    }
    query = fmt.Sprintf("query Batch(%s) {\n%s\n}", strings.Join(varDecls, ", "), strings.Join(aliases, "\n"))
}
```

- **Breadth-based budget packing**: when a GraphQL API exposes a breadth/complexity budget per request, model each aliased sub-query as a fragment with a pre-measured cost. Pack greedily up to the budget, with secondary caps on alias count and estimated result cardinality. On computation-limit errors, retry aliases individually to isolate the offending item

## Language-Specific

### Rust (async-graphql)

- **DataLoader for N+1 prevention**: implement `async_graphql::Loader` per relationship (e.g., `ShaderAuthorsLoader`, `ShaderThumbnailLoader`). Batch queries via `list_by_ids()` functions keyed by parent ID. DataLoader automatically coalesces concurrent field resolutions into a single batch query per loader
- **BroadcastStream subscriptions with typed domain events**: define a `DomainEvent` enum with typed struct variants. Services publish via `event_tx().send()`. Subscription resolvers subscribe via `BroadcastStream::new(rx)` and `filter_map` to select relevant event types. Handle lag via `Err(BroadcastStreamRecvError::Lagged)` with a warning log rather than disconnecting the client
- **Dual-surface error mapping via ErrorExtensions**: when `AppError` serves both REST (`IntoResponse`) and GraphQL (`ErrorExtensions`), a shared `status_and_code()` method returns `(StatusCode, &'static str)`. Both impls call it, keeping error codes single-source across API surfaces

### TypeScript (gql-tada + urql)

- **gql-tada for compile-time typed documents**: gql-tada provides compile-time type inference for GraphQL documents including subscriptions. Combined with urql's `subscriptionExchange` and `graphql-ws` transport for typed WebSocket connections. No runtime codegen step required — types are inferred from the schema at build time

### Go (genqlient)

- `genqlient` for type-safe GraphQL client generation via `go:generate`. Scalar bindings in `genqlient.yaml` map API types to Go types (`DateTime` → `string`, `BigInt` → `int64`, `JSON` → `any`)
- For APIs with undocumented complexity limits, implement retry-individually fallback: on computation-limit error with a multi-alias batch, re-execute each alias as a separate single-alias request

## Anti-Patterns

- Over-fetching with deep nesting, N+1 without DataLoader, schema-first with manual resolvers
- **Cloudflare rate limit on bare POST**: Cloudflare-fronted GraphQL APIs may impose a separate undocumented rate limit on bare POST requests without a query string. Workaround: inject `operationName` as a URL query parameter in a custom `http.RoundTripper`. Transparent to the GraphQL client layer

## Open Questions

- Federation/stitching for multi-service GraphQL, persisted queries, schema evolution strategy
