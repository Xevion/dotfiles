---
name: api-design
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: src/web/
    note: Axum API with typed error codes, cache constants, multi-window rate limiting, ts-rs contract generation
  - repo: Xevion/doujin-ocr-summary
    path: internal/server/ + internal/cache/
    note: Go Chi API with writeServiceError sanitization, cache middleware, tygo contracts
  - repo: Xevion/glint
    path: backend/src/middleware/ + src/error.rs
    note: "Multi-tier rate limiting with RAII cleanup, dual-surface error mapping (REST + GraphQL)"
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
- **Per-tier rate limit configuration**: define named tiers (global, auth, device, upload, agent) with separate burst and rpm budgets in a `RateLimitConfig` struct. Use keyed governor limiter per tier. RAII `CleanupGuard` (abort background cleanup task via `Drop`) prevents leaking the retention task. `enabled: false` maps to `None` for zero-overhead bypass without conditional branching in the hot path
- **Dual-surface error mapping**: when an error type serves both REST and GraphQL, a `status_and_code()` method returns `(StatusCode, &'static str)` in one match arm. Both `IntoResponse` and `ErrorExtensions` call it, keeping HTTP-status and machine-readable code single-source across API surfaces

## Language-Specific

### Rust

- **IntoResponse for errors**: implement `IntoResponse` on the error type to centralize HTTP status code mapping. Handlers return `Result<Response, ApiError>`
- **Extension traits for handler ergonomics**: `.or_not_found("Course", &crn)?` on `Option<T>`, `.conflict_on_unique(msg)?` on SQLx results. See [error-handling](../patterns/error-handling.md) for the full extension trait pattern
- **Central `From<sqlx::Error>` with inline DB error code detection**: map DB-level error codes (SQLite unique constraint = "2067", Postgres = "23505") in one place. Better than per-callsite `.conflict_on_unique()` extension trait when the mapping is exhaustive
- **ts-rs for contract generation**: see [cross-language-type-generation](../dx/cross-language-type-generation.md) for ts-rs conventions

### TypeScript

- Express/Hono patterns and Zod request validation — to be populated from future project audits

### Go

- **tygo for typed Go→TypeScript contract generation**: parallel to ts-rs for Rust. `tstype` struct tag overrides handle nullable pointers and omitempty fields
- **`writeServiceError` as single sanitization point**: `errors.Is` with sentinel errors to select status codes, log at status-proportional level (Debug for 404/409, Warn for 403, Error for 500). Keeps handlers clean and response shapes consistent
- **Cache-Control named constants as Chi middleware**: define constants (`cache.Public`, `cache.Private`, `cache.NoStore`) and apply as middleware on route groups rather than inline per-handler. Makes caching intent explicit at route definition

## Anti-Patterns

- Verb-based URLs (`/api/getCourses`)
- Inconsistent error formats across endpoints
- Inline cache header strings instead of named constants
- Rate limiting only at the global level without endpoint-specific controls
- Logging raw DB errors to the client response

## Open Questions

- See [graphql-schema-design](../architecture/graphql-schema-design.md) for GraphQL-specific patterns
- OpenAPI-first vs code-first for contract generation
