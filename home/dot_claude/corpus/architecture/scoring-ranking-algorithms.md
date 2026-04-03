---
name: scoring-ranking-algorithms
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/relatives
    path: packages/core/src/scoring.ts
    note: "Composite proximity+relatability+accuracy score with configurable weights, nice-number multiplier"
  - repo: local/rekuma
    path: internal/data/priority.go
    note: "Multi-factor refresh priority from pin status, staleness, catalog activity, archive state"
  - repo: local/maestro
    path: common pathfinding module
    note: "Multi-factor edge cost (terrain + turn angle + velocity + time-decaying penalties), epsilon escalation for anytime A*"
---

# Scoring & Ranking Algorithms

## Philosophy

Weighted multi-factor scoring for ranking items by relevance, freshness, or user preference. Scores are composable from independent factors, each with a clear semantic meaning. Weights are configurable — hardcoded weights are acceptable for internal ranking but should be tunable when exposed to users.

## Conventions

- **Composite scoring with named factors**: each factor (closeness, relatability, accuracy, staleness) is computed independently and combined via weighted sum. Factor functions are pure and independently testable
- **Log-scale proximity for ratio-based comparisons**: when comparing magnitudes across orders of magnitude, use logarithmic proximity scoring (e.g., `1 / (1 + log(ratio))`) to prevent large values from dominating
- **Behavioral invariant testing for scores**: test ordering properties (at1 > at2 > at5), symmetry (ratio 2 ≈ ratio 0.5), and floor/ceiling constraints rather than pinning exact float values. More resilient to algorithm tuning
- **Multi-factor priority for scheduling**: combine boolean flags (pinned, archived), temporal staleness (time since last update), and activity signals (post count, bump status) into a single numeric priority score for queue ordering
- **Time-adaptive scoring (epsilon escalation)**: for anytime algorithms that must trade quality for speed under time pressure, use a dynamic scoring modifier that grows with elapsed time (e.g., `epsilon = 1.0 + k*t²`). The algorithm starts optimal and progressively relaxes. Generalizes to any exploration-vs-exploitation tradeoff where the scoring model is updated by execution context, not just static factors
- **Feedback-driven cost adjustment**: when execution outcomes (failures, slowness) provide signal, feed them back into the scoring model as time-decaying penalties. Penalties are keyed by position/ID, compound additively at the same key, and decay linearly based on failure category (transient=60s, persistent=5m, permanent). The penalty weight is then summed into the base cost at search/ranking time

## Anti-Patterns

- Hardcoding weights without documenting their semantic meaning
- Testing exact float outputs instead of ordering/invariant properties
- Single-factor ranking when multiple signals are available

## Open Questions

- When to expose scoring weights to end users vs keeping them internal
- A/B testing infrastructure for ranking algorithm variants
