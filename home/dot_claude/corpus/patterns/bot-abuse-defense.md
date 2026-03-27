---
name: bot-abuse-defense
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/xevion.dev
    path: src/tarpit.rs
    note: "Tarpit with per-IP semaphore limits, streaming fake responses, randomized delays"
---

# Bot & Abuse Defense

## Philosophy

<!-- Active defense beyond rate limiting. Waste attacker time rather than just blocking. Defense in depth. -->

## Conventions

<!-- Tarpit with semaphore-limited streaming, per-IP connection caps, path-based malicious request detection -->

## Language-Specific

### Rust

<!-- Semaphore-per-tier via tokio::sync::Semaphore, RAII permit retention in stream::unfold closures, DashMap for per-IP tracking -->

## Anti-Patterns

<!-- Only blocking (gives attacker immediate signal to try elsewhere), unbounded connection acceptance, no logging of abuse patterns -->

## Open Questions

<!-- Fingerprinting ethics, honeypot legal considerations, interaction with upstream CDN rate limiting -->
