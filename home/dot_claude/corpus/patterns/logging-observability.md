---
name: logging-observability
category: patterns
last_audited: 2026-03-26
exemplars: []
---

# Logging & Observability

## Philosophy

<!-- Structured logging always, tracing spans for request lifecycle, metrics for dashboards not logs -->

## Conventions

<!-- JSON log format in production, human-readable in dev, log levels with intent, correlation IDs -->

## Language-Specific

### Rust
<!-- tracing crate with spans, tracing-subscriber for formatting, instrument attribute macro -->

### TypeScript
<!-- pino/winston structured logging, OpenTelemetry integration, request ID middleware -->

### Go
<!-- slog for structured logging, context-based logger propagation -->

## Anti-Patterns

<!-- printf debugging in production, logging sensitive data, log-and-throw (double reporting) -->

## Open Questions

<!-- OpenTelemetry maturity per language, log aggregation preferences -->
