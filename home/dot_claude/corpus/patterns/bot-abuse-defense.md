---
name: bot-abuse-defense
category: patterns
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/xevion.dev
    path: src/tarpit.rs
    note: "Multi-mode tarpit streaming, per-IP + global semaphores with RAII retention, comprehensive path detection"
---

# Bot & Abuse Defense

## Philosophy

Active defense beyond rate limiting. Waste attacker time rather than just blocking — a 403 gives immediate signal to try elsewhere, while a tarpit consumes their resources. Defense in depth: path detection identifies malicious requests, semaphores cap resource usage, streaming responses waste attacker time.

## Conventions

- **Multi-mode tarpit streaming**: randomly select a response mode per connection (RandomBytes, FakeHtml, FakeJson). Each mode sets an appropriate Content-Type header, mimicking plausible server responses. Randomization prevents attackers from fingerprinting the tarpit behavior

- **Dual semaphore with RAII permit retention**: use both a global semaphore (total concurrent tarpit connections) and per-IP semaphores (connections per source). Move the `OwnedSemaphorePermit` into the `stream::unfold` state tuple so it is held for the entire lifetime of the streaming response. Permits release only when the stream terminates (client disconnect or server close)

- **Timeout on semaphore acquisition**: use `tokio::time::timeout` (e.g., 100ms) on semaphore acquire rather than blocking. If the limit is reached, return 503 immediately — prevents the semaphore itself from becoming a resource exhaustion vector where queued requests consume memory waiting for permits

- **Comprehensive malicious path detection**: `is_malicious_path()` should cover common attack vectors: PHP/ASP/SQL extensions, WordPress admin paths, credential files (`.env`, `.aws`, `.kube`), Spring Boot actuators, API explorers (swagger, graphql), CGI paths, and infrastructure files (terraform, dockerfile). Case-insensitive matching via `to_lowercase()`

- **Path detection before rate limiting**: check malicious path patterns before consuming rate limit tokens. Malicious requests should not deplete the rate limit budget of legitimate traffic

## Language-Specific

### Rust

- `tokio::sync::Semaphore` for global limits, `DashMap<IpAddr, Arc<Semaphore>>` for per-IP tracking. `OwnedSemaphorePermit` (not `SemaphorePermit`) for ownership transfer into async stream closures
- `stream::unfold` with a state tuple carrying both permits, the response mode, and a byte counter. The stream yields chunks with randomized delays between them
- Randomized chunk sizes and inter-chunk delays to vary the tarpit's bandwidth profile

## Anti-Patterns

- **Only blocking (403/404)**: gives attackers immediate signal to rotate to another target. Tarpit instead to consume their time and resources
- **Unbounded connection acceptance**: without semaphores, a sufficiently aggressive scanner can exhaust server resources even with tarpitting. Always cap concurrent tarpit connections
- **No logging of abuse patterns**: log malicious path hits at info/warn level with source IP for pattern analysis. Silent tarpitting without logging misses reconnaissance trends
- **Tarpitting legitimate slow clients**: the malicious path detection must be accurate. False positives on legitimate paths waste real user time

## Open Questions

- Fingerprinting ethics and legal considerations for active defense
- Interaction with upstream CDN rate limiting (Cloudflare, AWS WAF)
- Honeypot deployment for intelligence gathering vs legal risk
