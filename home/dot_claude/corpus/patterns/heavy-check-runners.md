---
name: heavy-check-runners
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: local/kk-engine
    path: kk-dev crate (Check, Context, Steps, census/clipping/replay subcommands)
    note: "The first of the three: per-module CHECKS slices, one process, typed shared resources on Context, sampling flags, sweeps as subcommands"
  - repo: local/game-hacking
    path: dev crate (check subcommand, Context::once, --affected)
    note: "Adopted from kk-engine: same Check/skip/select/run, a string-keyed shared cache, an OWNERS table mapping checks to workspace crates"
  - repo: local/idakit
    path: idakit-runner, idakit-runner-macros (Runner, Scheduler, #[kernel_test])
    note: "Pool of warm worker processes leasing affinity groups, inventory self-registration, declared isolation, per-case timeout with kill-and-replace"
---

# Heavy Check Runners

Experimental. The concept is settled, the implementations are not: they work and save real wall-clock time, but they are not clean or idiomatic, they are duplicated between repositories, and extracting the idea into a reusable crate is the stated direction.

## Philosophy

Plain `cargo test` and nextest assume that tests are cheap, independent and cheap to start. For work that loads an asset install, decodes formats, opens a GPU device or fuzzes, the cost is the shared state, and a process per test re-pays it every time. A purpose-built runner schedules checks by what they need, shares expensive resources across them, filters easily, and rejects the assumption that the default runner is required at all. The needs of the project decide most of the design.

## Conventions

- **Keep the default runner for fast, hermetic, machine-independent tests.** Everything that reads an install, opens a GPU, spawns a process or takes seconds is a check in the runner. The nextest default profile excludes the heavy binaries and cases with a `default-filter`, and GPU tests sit in a throttled group
- **A check is a name and a function that fails by panicking** (`Check { name, run }`), so the body reads like a test body (assert2 `check!` and `assert!`). Each module exports a `CHECKS` slice. Names are `area/what`. A check that cannot run ends with `skip(reason)`, which is counted and named, never a silent pass
- **Prefer self-registration over hand-concatenated slices** (an attribute plus `inventory`), so adding a check touches one place. Where registration is by list, validate unique names at startup, because a silent drop reads as a pass. idakit does this; kk-dev and game-hacking's dev crate keep a `mod` line, an entry in `all()` and, in the second, an `OWNERS` entry
- **Share resources in a typed context.** Lazy `OnceLock` slots for the install, the card index, the GPU device and stock loaded bodies, each printing `(X in 1.2s, shared by every check)` the first time. A string-keyed `once(key, init) -> Arc<dyn Any>` map is more flexible and loses type safety; prefer a typed handle per resource
- **Group sub-steps that share one expensive load into one check** with a `Steps` helper that times each step, does not stop on a failed step, and asserts none failed at the end
- **Sampling is a flag, not a second test.** `Sample::{Default, Every(n), Full}` and a check's own `stride`, so each check declares its default stride and `--every N` and `--full` override it. A sweep's default is a sample unless the whole list is fast, and every sweep carries a control (mechanism on and off)
- **Filtering**: name substrings, `--list`, and `--affected` for checks whose owning crates changed (the gate task translates the changed package set into it)
- **Sweeps are subcommands of the dev binary with live output** (rate and ETA, one line per item, TSV and a summary file), never an `#[ignore]` test. A panic is caught per item, the worker is rebuilt, and the row is recorded
- **Isolation: pick it by risk.** One process with `catch_unwind` suffices when checks are cheap and trusted; log progress per check so a hang is visible. When a check can wedge, crash or leak state, use a pool of warm worker processes with a per-case timeout, kill-and-replace, and a lifetime tie to the runner (`PR_SET_PDEATHSIG` on Linux, a kill-on-close job object on Windows)
- **Warm workers lease affinity groups.** The runner spawns the same binary in worker mode; each worker pays kernel bring-up once. Cases carry a group (the fixture), and the scheduler picks the next group by pending cases divided by (workers already on it + 1). A declared `ReadOnly` or `Writes` isolation lets read-only cases share one open resource; the default is `Writes`, so a forgotten attribute costs a reopen but cannot corrupt neighbours
- **Report skips by name and count**, with a required-environment switch (`KK_REQUIRE_INSTALL`, `KK_REQUIRE_GPU`) that turns a skip into a failure so CI cannot go green by checking nothing
- **Use a real argument parser** for the runner's own surface, with environment variables only where no argument channel exists (mutation testing cannot pass arguments), parsed strictly. An unrecognised value is an error, never a fallback to a wider or narrower run

## Known warts

- The runner core is copied between repositories (`Check`, `Skipped`, `skip`, `select`, `Sample` and even its doc comment), and the copies have drifted: only one has `Steps`, they list at different points relative to filtering, and error and exit-code behavior differ
- The generic harness is welded to domain resources: a new shared resource means editing the `Context` type
- The macro is tied to a project-specific registry module and a domain-named attribute (`kernel_test`)
- Two invocation surfaces per repository with no unified result; neither check harness emits JUnit or JSON. A coverage run needs a workaround to see both runners
- Nextest coexistence is by filter strings that go stale when a test moves
- In-process runners have no timeout and no containment for an abort or a segfault
- Fuzzing sits outside every harness

## Anti-Patterns

- Running heavy shared-state work through a process-per-test runner
- A sweep written as an ignored test
- A skip that does not count in the summary
- A second copy of the runner core in each repository

## Related Topics

- [testing-quality](./testing-quality.md)
- [differential-oracle-testing](./differential-oracle-testing.md)
- [untrusted-input-parsers](./untrusted-input-parsers.md): fuzz targets

## Open Questions

- The reusable crate: where the generic runner ends and domain resources begin
- A shared result format (JUnit or JSON) across the runner and nextest
- Scheduling by declared resource needs rather than by name
