---
name: outbound-http-client
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/Applyhelm
    path: leads http client (Client::fetch, budgets, SHARED_DOMAINS), store schedule
    note: "One client owns per-host slots, a host-wide block on long Retry-After, a per-run request budget, and a stop token checked during backoff"
  - repo: local/steamlib
    path: retry module (RetryPolicy, wait_with)
    note: "Wait comes from a trait that classifies each transport's errors as After(wait) or Unstated; floor, ceiling, jitter that only adds"
  - repo: local/bf4-emblems
    path: ingest egress limiter
    note: "Client-side GCRA per egress IP, authenticated and anonymous budgets measured separately"
  - repo: local/local-web-fetch
    path: urlguard, http, fetch
    note: "Every hop checked for non-public addresses, stale-if-error cache, managed headless browser as a bounded resource"
---

# Outbound HTTP Client

## Philosophy

A program that fetches from other people's servers must be polite to them and safe for itself. Route every fetch through one client that owns concurrency, budgets, retry and refusal, so no call site can forget any of them.

## Conventions

### Politeness

- **One client owns per-host concurrency slots**, a host-wide block when the server sends a long `Retry-After` (short waits stay local to the throttle), and a per-run request budget per host. Key hosts that share one rate limit across tenant subdomains by their registrable domain
- **A stop token is checked before each attempt and during backoff sleep.** Shutdown must not wait out a backoff
- **Retry wait comes from a classifier**: a trait maps each transport's errors to `After(wait)` or `Unstated`. A minimum floor, a ceiling, jitter that only adds time, a counted attempt limit, and a pure `wait_with(draw)` so jitter is testable. Retries on DB-backed jobs are written as `execute_at` on the row with equal jitter, not slept in the worker. See [scheduling](./scheduling.md)
- **Client-side rate limiting (GCRA) per egress IP**, with authenticated and anonymous budgets measured separately, and aggregate ceilings that scale with the live channel count. Measure a stage before touching its limiter
- **Classify hostile upstream responses.** A non-JSON body is a fact about the response, not about the item. A 422 gate-redirect means session expiry. A drained frontier is not evidence of coverage

### Safety

- **Validate every hop**: the requested URL, each redirect, and the address actually connected. Refuse non-public addresses at all three, and canonicalize URLs the way the consumer will read them, so parser differentials are closed. A DNS name rebound to a private address is covered by the connected-address check

### Caching and cost

- **Stale-if-error** with a synchronous refresh and a short negative cache. See [isr-caching-proxy-patterns](./isr-caching-proxy-patterns.md)
- **A headless browser is a bounded resource.** A render semaphore sized to container memory, an idle-close reaper, a stuck-browser watchdog with a deadline derived from the fetch budget, a container memory cap as the OOM backstop, and a "browser died, relaunch once" retry

## Anti-Patterns

- Fetching from a call site with its own ad hoc retry
- Sleeping through backoff while shutting down
- Checking only the first URL of a redirect chain
- Treating one bad response as a verdict on the item

## Related Topics

- [scheduling](./scheduling.md)
- [bot-abuse-defense](../patterns/bot-abuse-defense.md)
- [filesystem-safety](../patterns/filesystem-safety.md)
