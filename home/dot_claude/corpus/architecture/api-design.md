---
name: api-design
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: src/web/
    note: Axum API with typed error codes, cache constants, multi-window rate limiting, ts-rs contract generation
---

# API Design

## Philosophy

REST conventions with typed contracts. Consistent error shapes. Cache-aware by default. Rate limiting as a first-class concern, not an afterthought.

## Conventions

- **Resource-oriented URLs**: nouns not verbs, plural resources (`/api/courses`, `/api/instructors/{slug}`)
- **Typed error responses**: error enum (e.g. `ApiErrorCode`) serialized as a stable machine-readable discriminant alongside a human message and optional structured `details` field

```rust
// Pattern: typed API error with stable code + flexible details
pub struct ApiError {
    pub code: ApiErrorCode,  // SCREAMING_SNAKE serialization
    pub message: String,
    pub details: Option<serde_json::Value>,
}
```

- **Cache-Control as named constants**: define per-route-category constants (`REFERENCE`, `SEARCH`, `DETAIL`, `ADMIN`), apply via a `with_cache_control()` wrapper. For mutable resources, combine with ETag + 304 based on a last-modified timestamp
- **Multi-window rate limiting**: classify routes by group (API/SSR/Admin/Static) with group-level limits, add endpoint-specific limits on expensive operations. Use internal bypass tokens for same-process proxied calls (e.g. SSR → API) to avoid double-counting

## Language-Specific

### Rust

- **IntoResponse for errors**: implement `IntoResponse` on the error type to centralize HTTP status code mapping. Handlers return `Result<Response, ApiError>`
- **Extension traits for handler ergonomics**: `.or_not_found("Course", &crn)?` on `Option<T>`, `.conflict_on_unique(msg)?` on SQLx results. A `db_error()` boundary function is the single place that logs raw DB errors and returns a sanitized 500
- **ts-rs for contract generation**: derive `TS` on all request/response types. `#[ts(export)]` + `serde(rename_all = "camelCase")` ensures the TypeScript contract stays in sync at compile time — no hand-maintained interface files

### TypeScript

<!-- Placeholder: Express/Hono patterns, Zod request validation -->

### Go

<!-- Placeholder: net/http vs chi/gin, middleware chains -->

## Anti-Patterns

- Verb-based URLs (`/api/getCourses`)
- Inconsistent error formats across endpoints
- Inline cache header strings instead of named constants
- Rate limiting only at the global level without endpoint-specific controls
- Logging raw DB errors to the client response

## Open Questions

- GraphQL adoption criteria vs REST
- OpenAPI-first vs code-first for contract generation
