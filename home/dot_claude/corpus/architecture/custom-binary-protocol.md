---
name: custom-binary-protocol
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/bose-re
    path: crates/bose-protocol/
    note: BMAP protocol with typed function blocks, newtype-enforced invariants, proptest roundtrip verification
---

# Custom Binary Protocol Implementation

## Philosophy

Encode the protocol specification into the type system so invalid packets are unrepresentable. Every enum variant, every newtype bound, every parse function is a statement about the protocol's invariants.

## Conventions

- **Packet framing**: fixed-width header (function block, function ID, operator, length) followed by variable-length payload. Parse returns the consumed length so the caller can advance a stream buffer containing multiple packets per frame
- **Dispatch tables as enums**: map protocol function blocks and operators to Rust enums with `TryFrom<u8>`. Each enum variant carries its display name for logging. Unknown values are kept, not rejected: protocols evolve and the parser should not refuse packets it does not understand yet. Two forms. A catch-all variant is fine when the number is not needed again. When a vendor extends a code space without notice (Steam's result and message codes), model each value as a newtype over its integer with named associated constants and a known table (a `wire_enum!` macro): unknown values keep their number, compare by number and re-encode unchanged, so a parse-then-serialize round trip loses nothing. Where a rule needs the value, the consumer must refuse an unknown with a named error carrying the raw text, rather than waiting, guessing or skipping
- **Newtype-enforced invariants**: `DeviceId(0-3)`, `Port(0-3)`, `BmapPayload(0-255 bytes)`. Constructors validate bounds at creation time; all downstream code trusts the invariant without re-checking
- **Transport abstraction**: separate the protocol parsing (sync, pure) from the transport layer (async, I/O). The protocol crate is `#[no_std]`-compatible where possible; the transport layer handles RFCOMM/BLE/serial specifics
- **Roundtrip testing with proptest**: `parse(serialize(x)) == x` for all generated packets, plus `parse(arbitrary_bytes)` never panics. These two properties together ensure both correctness and robustness

## Language-Specific

### Rust

- snafu (with `miette::Diagnostic` where users see the errors) for protocol errors with machine-readable codes (`bose::packet::too_short`). See [rust](../languages/rust.md)
- A `source` on struct variants wrapping transport errors (e.g., `bluer` RFCOMM errors)
- **Bound every file-supplied count.** Pass counts through one checked chokepoint that multiplies by record size and compares with the bytes actually left, before anything reserves capacity. See [untrusted-input-parsers](../patterns/untrusted-input-parsers.md)
- **Resync by dropping one byte.** On a bad length or CRC, the decoder drops a byte, scans for the next sync marker, and reports the dropped bytes as an event. Frame as `sync | type | len | payload | CRC-32`, carry a protocol version and a per-start instance id in the handshake, and put the frame layout and message tables in the module's rustdoc. Put the format crate in `no_std` and share it by both ends so one implementation serves them
- Proptest strategies for each protocol component (function blocks, operators, full packets) composed into a top-level `arb_bmap_packet()` strategy

## Anti-Patterns

- Stringly-typed protocol fields — every field that has a fixed set of values should be an enum
- Panicking on unknown values — protocols evolve; use a catch-all variant
- Mixing parse logic with I/O — the parser should work on `&[u8]`, not on streams
- **Raw index fields without newtype bounds**: using `usize` fields for protocol region boundaries (start, end) without a newtype that validates `start < end <= data.len()` at construction. Callers must re-check bounds on every use. Wrap in a newtype (e.g., `KeyRegion { start: usize, end: usize }`) with a validating constructor

## Open Questions

- Protocol versioning strategies when reverse-engineering an evolving target
- Automated protocol grammar extraction from decompiled code
