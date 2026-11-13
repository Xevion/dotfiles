---
name: untrusted-input-parsers
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: local/steamlib
    path: kv crate (Limits, Error) and the fuzz workspace
    note: "Limits as a required argument at every entry point, explicit stack instead of recursion, one cargo-fuzz target per parser, cfg(fuzzing) hook for private parsers"
  - repo: local/assrip
    path: dagor Reader::check_count, oodle Limits
    note: "One checked chokepoint for file-supplied counts, explicit output-size limits threaded from open time, fuzz runs with an RSS cap"
  - repo: local/ferrite-uefi
    path: ferrite-wire link decoder
    note: "Resync by dropping one byte, bad lengths and CRCs reported as events"
---

# Untrusted-Input Parsers

## Philosophy

A parser of bytes from a server or a file must bound what those bytes can make it allocate, scan or recurse into. A corrupt length that reaches the allocator is not a panic: allocator aborts cannot be caught by `catch_unwind` and are invisible to proptest.

## Conventions

- **Limits are a required argument at every entry point**, with no default applied implicitly. A caller-supplied `Limits` struct bounds depth, element counts and output size. Thread explicit output limits from open time into decompressors
- **Funnel every file-supplied count through one checked chokepoint.** Multiply the count by the record size and compare with the bytes actually left, before anything reserves capacity (`Reader::check_count`). Reserving a count directly lets a corrupt length abort in the allocator
- **Walk an explicit stack instead of recursing.** A stack overflow aborts and no caller can catch it
- **One fuzz target per parser** (cargo-fuzz), run through a Justfile recipe with RSS and malloc caps so an abort surfaces as a reportable out-of-memory (`-rss_limit_mb=4096`). Reach private parsers through a `cfg(fuzzing)` hook rather than making them public
- **Resync, do not stall.** A decoder that sees a bad length or CRC drops one byte and scans for the next sync marker, and reports the dropped bytes as an event
- **A proptest pair for codecs**: `parse(serialize(x)) == x` plus `parse(arbitrary_bytes)` never panics. See [testing-quality](./testing-quality.md). Neither covers allocator aborts; the chokepoint and fuzzing do
- **Fail closed on unknowns that a rule depends on**, and keep unknown values that only need to round-trip. See [custom-binary-protocol](../architecture/custom-binary-protocol.md)
- **Record the aborts you find.** Project notes that list each allocator abort found (one by a fuzzer) keep the rule honest

## Anti-Patterns

- Reserving capacity from a count read out of the input
- Recursive descent over untrusted structure
- A parser default that is permissive, applied silently
- Relying on `catch_unwind` to contain a bad parse

## Related Topics

- [differential-oracle-testing](./differential-oracle-testing.md)
- [heavy-check-runners](./heavy-check-runners.md): fuzzing sits outside the current harnesses
