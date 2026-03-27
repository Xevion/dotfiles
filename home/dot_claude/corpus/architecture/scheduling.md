---
name: scheduling
category: architecture
last_audited: 2026-03-27
exemplars:
  - repo: local/rekuma
    path: internal/scheduler.go
    note: "Priority-queue scheduler with per-second tick, optimistic next_refresh_after stamping, adaptive intervals"
---

# Scheduling

## Philosophy

Background job scheduling with priority-driven dispatch and activity-adaptive intervals. Schedulers tick on a fixed cadence, query work items ordered by a composite priority score, and dispatch with optimistic locking to prevent duplicate processing.

## Conventions

- **Priority-queue dispatch**: query pending work items ordered by a multi-factor priority score (staleness, pin status, activity level). Dispatch the top-N items per tick based on available worker capacity
- **Optimistic dispatch stamping**: stamp `next_refresh_after` before dispatching work to prevent other scheduler ticks from re-dispatching the same item. If the worker fails, the stamp expires naturally and the item re-enters the queue
- **Activity-adaptive polling intervals**: assign refresh cadence based on item state — recently active items poll frequently (15min), dormant items less often (60min), archived items rarely (weekly). The interval function switches on observable state signals (recency, pin status, archive flag, bump-limit)
- **Generic BatchWorker[T] pattern**: extract the fetch→process loop into a reusable worker that accepts a fetch function, a process function, and context cancellation. Keeps scheduling logic separate from business logic

## Anti-Patterns

- Fixed polling intervals for all items regardless of activity level
- Scheduling without duplicate-dispatch prevention (leads to thundering herd)
- Mixing scheduling logic with business logic in the same function

## Open Questions

- Distributed scheduler coordination (leader election, partition assignment)
- Backpressure strategies when workers can't keep up with dispatch rate
