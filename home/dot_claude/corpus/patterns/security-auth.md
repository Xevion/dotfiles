---
name: security-auth
category: patterns
last_audited: 2026-03-26
exemplars: []
---

# Security & Auth

## Philosophy

<!-- Zero-trust defaults, validate at system boundaries, secrets never in code, token rotation discipline -->

## Conventions

<!-- JWT for stateless auth, session tokens for stateful, RBAC over ABAC unless needed, secret managers (Doppler/Vault) -->

## Language-Specific

### Rust
<!-- argon2 for hashing, tower middleware for auth, typed permission extractors -->

### TypeScript
<!-- bcrypt/argon2, middleware-based auth, Zod for input validation, helmet for headers -->

### Go
<!-- crypto/subtle for comparisons, middleware chains, context-based user propagation -->

## Anti-Patterns

<!-- Secrets in env files committed to git, rolling your own crypto, client-side-only auth checks -->

## Open Questions
