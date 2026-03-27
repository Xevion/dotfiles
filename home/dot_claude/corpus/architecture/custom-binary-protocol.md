---
name: custom-binary-protocol
category: architecture
last_audited: 2026-03-26
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
- **Dispatch tables as enums**: map protocol function blocks and operators to Rust enums with `TryFrom<u8>`. Each enum variant carries its display name for logging. Unknown values get a catch-all variant rather than an error — protocols evolve and the parser should not reject packets it doesn't understand yet
- **Newtype-enforced invariants**: `DeviceId(0-3)`, `Port(0-3)`, `BmapPayload(0-255 bytes)`. Constructors validate bounds at creation time; all downstream code trusts the invariant without re-checking
- **Transport abstraction**: separate the protocol parsing (sync, pure) from the transport layer (async, I/O). The protocol crate is `#[no_std]`-compatible where possible; the transport layer handles RFCOMM/BLE/serial specifics
- **Roundtrip testing with proptest**: `parse(serialize(x)) == x` for all generated packets, plus `parse(arbitrary_bytes)` never panics. These two properties together ensure both correctness and robustness

## Language-Specific

### Rust

- `thiserror` + `miette::Diagnostic` for protocol errors with machine-readable codes (`bose::packet::too_short`)
- `#[source]` on struct variants wrapping transport errors (e.g., `bluer` RFCOMM errors)
- Proptest strategies for each protocol component (function blocks, operators, full packets) composed into a top-level `arb_bmap_packet()` strategy

## Anti-Patterns

- Stringly-typed protocol fields — every field that has a fixed set of values should be an enum
- Panicking on unknown values — protocols evolve; use a catch-all variant
- Mixing parse logic with I/O — the parser should work on `&[u8]`, not on streams

## Open Questions

- Protocol versioning strategies when reverse-engineering an evolving target
- Automated protocol grammar extraction from decompiled code
