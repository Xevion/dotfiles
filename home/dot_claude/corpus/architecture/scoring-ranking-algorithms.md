---
name: scoring-ranking-algorithms
category: architecture
last_audited: 2026-04-10
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
  - repo: local/stashapp-rapid-tag
    path: src/lib/server/confidence.ts
    note: "Full Bayesian pipeline — Beta-Binomial conjugate prior, SPRT stopping, Thompson sampling queue ordering, quality-weighted evidence, recency decay"
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

## Bayesian Classification Pipeline

For probabilistic binary classification with human-in-the-loop feedback (tagging, review queues, relevance judgments), a full Bayesian pipeline is a distinct alternative to weighted multi-factor scoring. It tracks *belief* about each item rather than a *fixed score*, stops naturally when confident, and balances exploration/exploitation explicitly.

- **Beta-Binomial conjugate prior per (item, label)**: track two counts per item — `alpha` (successes: label-applicable clicks) and `beta` (failures: not-applicable clicks). Posterior mean is `alpha / (alpha + beta)`; variance is a closed form. Start with `alpha=1, beta=1` (uniform prior). Updates are O(1): increment `alpha` on a positive click, `beta` on a negative. Store both in the item row; serialize as a tuple for transport
- **Quality-weighted evidence updates**: when observations vary in reliability (image quality, reviewer confidence), scale the increment by a quality weight in `[0, 1]` rather than always incrementing by 1. `alpha += quality * clicked ? 1 : 0`. High-quality observations move the posterior faster; low-quality ones barely budge it. This couples an upstream quality signal (e.g., Laplacian-variance blur score for images) into the Bayesian model without a separate scoring pass
- **SPRT for optimal stopping**: Wald's Sequential Probability Ratio Test. Accumulate log-likelihoods of the evidence under two hypotheses (p = p0 "not applicable" vs p = p1 "applicable") and stop when the cumulative log-likelihood ratio crosses `log((1-β)/α)` (accept p1) or `log(β/(1-α))` (accept p0). Tunable Type-I (α) and Type-II (β) error bounds — set α=β=0.05 for balanced 5% error, tighter for critical decisions. Optimal in the sense that no other test achieves the same error bounds with fewer samples on average
- **Thompson Sampling for queue ordering**: to decide which item to show the reviewer next, draw one sample from each item's Beta posterior and order by sample entropy (max entropy ≈ max uncertainty). This explore/exploit policy prioritizes items the model is least confident about while naturally down-weighting items it has already decided. Implement Beta sampling via the ratio-of-gammas method (Marsaglia-Tsang for gamma) — no external dependency needed
- **Recency half-life decay on sampling score**: multiply the Thompson entropy score by `1 - exp(-ln(2) * msSinceServed / halfLifeMs)` before ordering. Items served recently to the user get deprioritized for a bounded window without being permanently excluded. Tunable half-life (5–30 min typical) trades session repetition against exploration breadth
- **State storage**: `alpha`, `beta`, `sprt_accumulator`, `decision` (pending/accepted/rejected), `frames_seen`, `last_served_at` in a single row per (item, label). Flush accepted/rejected decisions to the downstream system (StashApp, Elasticsearch, etc.) in batches; keep the accumulator state for re-evaluation if the decision is reopened

## Anti-Patterns

- Hardcoding weights without documenting their semantic meaning
- Testing exact float outputs instead of ordering/invariant properties
- Single-factor ranking when multiple signals are available

## Open Questions

- When to expose scoring weights to end users vs keeping them internal
- A/B testing infrastructure for ranking algorithm variants
