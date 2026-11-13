---
name: differential-oracle-testing
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: local/mc-seed-preview
    path: msp-worldgen version sweeps, sweep recipe, data-driven test matrix
    note: "A reference implementation running in-process is the oracle; sweeps over every cached artifact, one case per artifact through a custom harness"
  - repo: local/steamlib
    path: steamlib-proto codec agreement tests and the emit_oracle build step
    note: "A hand-written codec checked against the reference generator's output for the same schema, oracle generated at build time from descriptors"
  - repo: local/cargo-sweep-ng
    path: refactor equivalence gates and legacy-implementation diff
    note: "A rewrite diffed against the retained legacy implementation over a generated matrix, with a fault-injecting platform wrapper"
  - repo: local/assrip
    path: decoder validation
    note: "Decoders checked against known plaintext, not against each other"
---

# Differential & Oracle Testing

## Philosophy

A test that checks an implementation against itself proves consistency, not correctness. Where a trustworthy reference exists (the original implementation, a generator's output, known plaintext), compare against it, and compare against it in both directions.

## Conventions

- **Pick an oracle that is independent of the code under test.** An in-process reference implementation, the reference generator's output for the same schema (bytes must agree and round-trip, with the oracle generated at build time from the same descriptors), or known plaintext. Two decoders agreeing with each other proves only that they share an error
- **Round trips are not an oracle.** A round trip over a variants list is a tautology. Check against the source of truth: a C++ emitter that names the SDK's own enumerators, compared with the Rust discriminants
- **The agreement predicate is held true in both directions and never edited to make a sweep pass.** A sweep that goes red is a finding
- **One case per artifact.** Generate a case per cached artifact through a custom harness (`libtest-mimic`). An empty cache fails rather than passes, and naming a version is treated as a request to test it, so a filter that narrows cases cannot produce silent passes
- **Sweeps run apart from the fast suite** because they share one expensive process. See [heavy-check-runners](./heavy-check-runners.md). Exclude them from nextest through a profile filter and run them under plain `cargo test` or the dev runner
- **Verify a behaviour-preserving refactor with equivalence gates.** Equal sets of test names, and identical output from a frozen baseline binary on a frozen snapshot, with the baseline captured before any file moves. Test counts alone are not enough
- **Diff a rewrite against the retained legacy implementation** over a generated matrix, with a fault-injecting platform wrapper that seams in I/O failures, and assert invariants per matrix cell
- **Fixture tests report every divergence at once** and write artifacts for CI upload, instead of stopping at the first

## Anti-Patterns

- Collapsing a broken manifest into "no corpus" (it once let a whole matrix run zero cases and report green)
- Comparing two backends and calling agreement correctness
- Editing the predicate to make a sweep pass

## Related Topics

- [untrusted-input-parsers](./untrusted-input-parsers.md)
- [testing-quality](./testing-quality.md)
- [custom-binary-protocol](../architecture/custom-binary-protocol.md)
