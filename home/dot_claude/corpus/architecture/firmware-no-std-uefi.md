---
name: firmware-no-std-uefi
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/ferrite-uefi
    path: ferrite-hal, ferrite-xhci, ferrite-testing, ferrite-dev, clippy.toml
    note: "no_std exactly when a crate links into firmware, HAL traits, typed registers and addresses, single-home API bans, host simulator for drivers, no hakari by design"
---

# Firmware & no_std (UEFI)

Source: one project, ferrite-uefi. Treat as observed practice.

## Philosophy

Make hardware invariants unrepresentable in types, give each hardware operation exactly one implementation, and test drivers on the host against a simulated device before any real machine sees them.

## Conventions

- **A crate is `no_std` exactly when it links into a firmware binary.** Shared crates use plain `#![no_std]` with no `std` feature, because Cargo feature unification would leak `std` into the firmware build. Std-only helpers go in a separate crate. This is also why the workspace does not use cargo-hakari
- **No default build target.** A `default-members` list plus `cargo efi-*` aliases (with a `[package.metadata] target` in each app manifest) split host and target builds. Apps built for `x86_64-unknown-none` are converted from ELF to PE by the dev tool
- **Platform backends sit behind HAL traits** so a host simulator runs the same driver. Time is `Duration` and `Instant`, never `_ms: u64`
- **Distinct address types with private fields and checked arithmetic** (`PhysAddr`, `BusAddr`, `BusAddr32`). Registers are `Reg<T, Access>` constants with sealed access markers, and write-1-to-clear registers accept only an explicit `Rw1cWrite`. Hardware-defined ranges are validating newtypes. Codes from hardware or the wire are enums with an `Unknown(raw)` catch-all that is logged, not a panic
- **Single-home API bans** through `clippy.toml` (raw volatile access, TSC reads, stalls, resets, truncating writes, `println`), each ban naming the sanctioned home. See [rust](../languages/rust.md)
- **Dev profile**: own code at `opt-level = 1` (0 gives huge stack frames on the small firmware stack), hot dependencies at 2, each named with a reason
- **Test drivers on the host against a simulated device**: a register file plus a mock HAL, clock and DMA in a std-only test-double crate. Drive the state machine end to end and assert invariants such as byte-count balance (`bytes_queued == bytes_sent + bytes_dropped + buffered`). Slow generator-drift tests are `#[ignore = "reason"]` and mirrored by a `just <x> --check` recipe. Record real-hardware results in docs together with the run that showed them
- **Vendored C**: fetched pinned and snapshotted, configure results verified, stubs generated; see [build-time-codegen](../dx/build-time-codegen.md) and [ffi-facade-and-unsafe-containment](./ffi-facade-and-unsafe-containment.md)
- **A remote real-hardware loop** (deploy helper, one-shot boot entries, watchdog-guarded loader, log channel out over a debug link) is described in [reloadable-rust-payloads](./reloadable-rust-payloads.md)

## Anti-Patterns

- A `std` feature on a shared firmware crate
- cargo-hakari in a workspace that mixes host and firmware targets
- Hand-rolled volatile access outside the one sanctioned module
- Busy-waiting the whole app instead of waiting on an event or using the HAL delay

## Related Topics

- [rust](../languages/rust.md)
- [linux-hardware-interfaces](./linux-hardware-interfaces.md)
- [custom-binary-protocol](./custom-binary-protocol.md)
