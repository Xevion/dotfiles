---
name: claude-code-hook-binaries
category: dx
last_audited: 2026-11-12
exemplars:
  - repo: local/guard
    path: payload parsing and dispatch, executor, logging module, DESIGN.md
    note: "A Rust hook binary that parses shell commands: per-field lenient parsing with a typed abstain, exit-code contract, best-effort logging, startup budget"
---

# Claude Code Hook Binaries

Source: one project, guard. Observed practice for a hook that runs on every tool call.

## Philosophy

A hook sits in the path of every command, so it must be fast, must never break the tool it guards because of its own bug, and must record what it did not examine. Heuristics ship quiet and earn their way to loud.

## Conventions

- **The contract is stdin JSON in, an exit code and JSON out.** Exit 2 means block only for a pre-tool-use hook; for a post-tool-use hook exit 2 is an alert, because the write has already landed. Reserve the word "block" for the case where the exit code really stops the tool
- **Parse per field and record a typed abstain.** Each field is read leniently (`#[serde(default, deserialize_with = "lenient")]`), so an upstream shape change costs one field. A payload that cannot be examined returns `Outcome::abstain()`, which is logged as its own decision label, distinct from "looked and allowed", so the log shows which decisions were not examined
- **Fail open, and treat it as a liability to track.** For a third-party hook whose input can drift, running the command when it cannot be judged is the chosen policy, and the design doc lists it under known gaps. This is the opposite of a zero-trust default, which suits data the system owns
- **Logging is best-effort.** Discard the write result at one boundary function (`let _ = try_record(..)`), so a broken log path never changes the exit code or the verdict
- **Count spawns as the real cost.** The binary starts on every call, so replacing a `git rev-parse` subprocess with an in-process `.git` walk cut the path from 3.5 ms to 1.9 ms with a quarter of the variance. Use OS threads and no async runtime; see [concurrency-async](../architecture/concurrency-async.md)
- **Rewriting a command must be fail-open and never approximate.** The rewriter and the runner share one parser and one predicate, splicing uses single-quote escaping (`'\''`), and the quoting path is property-tested
- **Calibrate heuristics against your own code.** A rule ships as a quiet nudge and moves to a loud tier only when a census of your own code shows its precision and repeat rate warrant it (a census over 1,291 findings in 224 files drove the tiers)
- **Fixture tests with directives**: real source fixtures run through the production pipeline, with `@fixture` and `@expect` directive lines blanked before parsing so line numbers do not shift. A fixture with no expectation is an error, not a pass, and a known-defect fixture carries a `_BUG` suffix and is expected to fail until fixed
- **Sibling `tests.rs` modules** (`mod tests;`) for unit tests next to the code

## Anti-Patterns

- A hook whose own failure blocks the tool
- Logging failures that change the verdict
- Folding "could not examine" into "allowed"

## Related Topics

- [rust](../languages/rust.md)
- [ai-assisted-dev](./ai-assisted-dev.md)
- [documentation-naming](../project-structure/documentation-naming.md): the design doc with a known-gaps section
