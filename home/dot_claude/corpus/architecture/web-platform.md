---
name: web-platform
category: architecture
last_audited: 2026-03-26
exemplars: []
---

# Web Platform

## Philosophy

Prefer native web platform APIs over library abstractions when browser support is sufficient. Progressive enhancement — features degrade gracefully when APIs are unavailable.

## Conventions

<!-- Placeholder for future findings: Web Components, Intersection Observer, etc. -->

## Anti-Patterns

- Polyfilling APIs that have >95% browser support
- Using JS libraries for what CSS or HTML can do natively
- Ignoring platform APIs because "we've always used library X"

## Open Questions
