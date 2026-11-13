---
name: scheduling
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/Applyhelm
    path: store schedule (succeed, fail, defer, spread, backoff), due_boards, sweep module
    note: "DB-persisted tiered schedule with jitter, a deferral outcome that is neither success nor failure, per-vendor drain loops under a global semaphore"
  - repo: Xevion/banner
    path: scraper worker retry_backoff
    note: "Exponential backoff with equal jitter written as execute_at on the job row"
  - repo: Xevion/xevion.dev
    path: GitHubScheduler, sync health columns
    note: "Poll health tracked apart from the data, ETag polls that cost no rate limit, typed RateLimited with retry_after"
  - repo: local/rekuma
    path: internal/scheduling/
    note: "Priority-queue scheduler package split (scheduler.go + worker.go + priority.go), DBBackoff, QueueThreshold backpressure, InjectJob for handler bypass"
  - repo: local/ts-chan
    path: src/services/polling.ts
    note: "Browser PollController — visibility pause/resume with catch-up poll, 2^failures exponential backoff, 404→stop semantic, platform-enforced MIN_INTERVAL"
---

# Scheduling

## Philosophy

Background job scheduling with priority-driven dispatch and activity-adaptive intervals. Schedulers tick on a fixed cadence, query work items ordered by a composite priority score, and dispatch with optimistic locking to prevent duplicate processing.

## Conventions

- **Priority-queue dispatch**: query pending work items ordered by a multi-factor priority score (staleness, pin status, activity level). Dispatch the top-N items per tick based on available worker capacity
- **Optimistic dispatch stamping**: stamp `next_refresh_after` before dispatching work to prevent other scheduler ticks from re-dispatching the same item. If the worker fails, the stamp expires naturally and the item re-enters the queue
- **Activity-adaptive polling intervals**: assign refresh cadence based on item state — recently active items poll frequently (15min), dormant items less often (60min), archived items rarely (weekly). The interval function switches on observable state signals (recency, pin status, archive flag, bump-limit)
- **Generic BatchWorker[T] pattern**: extract the fetch→process loop into a reusable worker that accepts a fetch function, a process function, and context cancellation. Keeps scheduling logic separate from business logic
- **Package split at scale**: once the scheduler grows past a single file, split into `scheduler.go` (tick loop + dispatch), `worker.go` (per-item processing), `priority.go` (scoring + tier → priority mapping). Keep the public surface narrow — the HTTP layer should only need `InjectJob(ctx, id)` for bypass and the lifecycle methods for startup/shutdown
- **`QueueThreshold` backpressure gate**: skip dispatch when `len(workQueue) >= QueueThreshold`. Prevents runaway growth when workers can't keep up and lets the existing backlog drain before new items pile on. Pair with housekeeping metrics so the gate being active is observable
- **Handler-bypass injection path (`InjectJob`)**: expose a synchronous path that pushes a single job directly into the work queue, bypassing the scheduler's next-tick wait. Used by HTTP handlers that must trigger immediate processing (pin/unpin, manual refresh). Must share the same `next_refresh_after` stamping to avoid duplicate-dispatch races with the tick loop

## Persisted-State Scheduling

An alternative to an in-memory priority queue: the schedule is a column, and the tick is a plain query (`WHERE enabled AND next_poll_at <= now ORDER BY next_poll_at, id LIMIT n`).

- **Tiered, jittered intervals**: a success sets the tier (Hot or Cold from the count of open, not ruled-out items) and `next_poll_at` to now plus the tier interval with about 10% jitter. A failure uses exponential backoff, with a server `Retry-After` hint taking the maximum, capped at a maximum
- **A deferral is a third outcome.** `Outcome::Deferred { until, reason }` moves only `next_poll_at`, adds up to 10% of the wait as jitter, and leaves health counters alone. It is produced when a host is blocked or its per-run request budget is spent, so a rate-limited host does not trip failure or disable logic. See [outbound-http-client](./outbound-http-client.md)
- **Two-level concurrency**: per-vendor drain loops (`join_all` over vendors, each draining due rows in rounds with a `FuturesUnordered` capped at `per_vendor`) under one global `Semaphore(total)`. Each task acquires a slot first and re-checks the stop token after acquiring. A slow vendor never blocks the others, and the total stays bounded
- **Retries with equal jitter** (half the capped delay plus a random amount up to half) guarantee a minimum spacing while de-synchronizing mass failures. Write the delay as `execute_at` on the job row instead of sleeping in the worker, and log warn while retries remain and error only on exhaustion
- **Track poll health separately from the data.** A last-success timestamp and a latest-error column (cleared on success) let an admin UI tell a quiet repository from a broken sync. A failure records only the error and leaves the success time alone. Activity advances through `GREATEST(...)` so it never regresses. Invalidate stale heap entries by registry epoch, and poll unchanged resources with an ETag so a 304 costs no rate limit
- **Leases instead of stamps**: a claim lease with a reclaim TTL and an attempts counter, rather than one-shot dispatch stamps. See [postgres-operations](./postgres-operations.md)
- **Persisted schedules have no in-memory queue and no stamping.** Whether a concurrent sweep could double-poll was not established; check before assuming it cannot

## Browser Polling Engine

Server-side schedulers manage a *work queue of many items*; browser polling manages a *single target with adaptive cadence*. Different problem, different patterns.

- **Page Visibility API pause/resume with catch-up poll**: listen for `document.visibilitychange`. On hidden, pause the polling timer; on visible, immediately call `tick()` once (catch-up poll) and schedule the next tick. Power and data savings when the tab is backgrounded, with no staleness penalty when the user returns
- **Exponential backoff via `2^failures`**: on error, compute `interval * 2^min(failures, cap)` (with a cap of 6 for a 64× multiplier) and clamp against a hard maximum (5 minutes is a reasonable default). Reset the failure counter on any successful response
- **404 → stop semantic**: a 404 response means the resource is gone; stop polling entirely rather than retrying. Distinguish from network errors (which back off) and transient 5xx (which also back off). Surface the "gone" state to the caller via an `onGone` callback so the UI can remove the polled entity
- **Platform-enforced minimum interval**: bake the platform's rate-limit floor into the controller as a constant (`MIN_INTERVAL = 10_000` for 4chan, e.g.). Clamp the configured interval so consumers can't accidentally set a value below the floor. This is a policy enforcement point, not a suggestion
- **`PollController` as the unit of composition**: wrap the timer, visibility handler, failure counter, and target callbacks into a single class with `start()`, `stop()`, and `tick()` methods. Consumers own the controller and call `stop()` during component cleanup

## Anti-Patterns

- Fixed polling intervals for all items regardless of activity level
- Scheduling without duplicate-dispatch prevention (leads to thundering herd)
- Mixing scheduling logic with business logic in the same function

## Open Questions

- Distributed scheduler coordination (leader election, partition assignment)
- Backpressure strategies when workers can't keep up with dispatch rate
