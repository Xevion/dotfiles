---
name: reloadable-rust-payloads
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/game-hacking
    path: Factorio lab loader (slot, control), lab-dtors DtorGate, lab runtime
    note: "Permanent cdylib loader, per-generation private copy of the runtime, socket-triggered swap, lab_eject must return 0, /proc/self/maps check, thread-local destructor gate"
  - repo: local/stardew-bot
    path: sdv-native cdylib, StardewBot.Bridge NativeDriver, Payload::stage
    note: "Content-hash-named staged copy, fingerprint and struct-size handshake, load new before shutting old down, old build keeps running if the new is refused"
  - repo: local/ferrite-uefi
    path: ferrite-loader, ferrite-dev push and loader, ferrite-wire link
    note: "A resident loader replaces whole .efi images over a debug link; one-shot boot entries and watchdogs recover into a known-good OS"
---

# Reloadable Rust Payloads

## Philosophy

Iterate on injected or embedded code without restarting the target. A tiny, stable, resident part owns the contract; the payload is a replaceable library. Compatibility is checked at load time, never assumed. The only state that survives a reload is state the resident side or the host program owns. A failed swap is visible and leaves a working system.

## Three mechanisms

| | Factorio (game-hacking) | stardew-bot | ferrite-uefi |
|---|---|---|---|
| Resident part | tiny cdylib loader preloaded into the game | C# mod plus a host-callback table | the loader EFI app, owner of the debug link |
| Payload | runtime `.so` | `sdv-native` cdylib | a whole `.efi` image (app or loader) |
| Trigger | Unix socket | control-socket reload request | frames over the debug link (begin, data, end) |
| Compatibility check | ABI stamp | xxh3 fingerprint of generated bindings plus struct-size table | protocol version plus table revision plus CRC |
| Quiesce | `lab_eject` must return 0 | unconditional shutdown, coroutine force-unwound | cooperative stop, then the image returns |
| Verify unload | `/proc/self/maps` | debug-only | not applicable |
| On refusal | old runtime put back | old build keeps running | old loader resumes; a hung replacement resets via watchdog |
| State | discarded | discarded, rebuilt from the game or a scenario | discarded, the loader keeps only the link |

## Conventions

- **Prefer a fresh file path per load.** Copy the payload to a per-generation private path (`runtime-N.so`, or a content-hash name renamed into place) and load the copy, so a rebuild can overwrite the original while the old copy is mapped. The platform's loader caches by path and hands back an already loaded library for a path it has seen, so a rebuild with identical bytes needs a fresh name (a pid suffix in tests)
- **Prefer a content-derived fingerprint of the generated boundary types, plus a runtime `size_of` table, over a hand-bumped ABI number** when two sides are built by different tools at different times. Name the older side by file modification time in the mismatch error
- **Prefer loading the new payload before shutting down the old one**, and keep the old one running if the new is refused, when the host must stay alive
- **Make `dlclose` real.** Require an explicit eject that returns 0 only after hooks, threads and probes are gone, then check `/proc/self/maps` and report `Unmapped` or `StillMapped`. Thread-local destructors that the C library registered pin the mapping: route them through a loader-owned gate that can be held and drained at eject (a destructor running at that moment refuses the close)
- **Prefer a payload with no threads of its own** when it must be unloadable. If it needs parallelism, join the threads before the job returns
- **Prefer discarding payload state on reload** and rebuilding it from the authoritative host (the game, the world) over serializing it across generations
- **Probes are a separate layer** that load, run and unload while the runtime stays up, with typed args and report structs
- **Panic isolation**: each side catches its own panics, because two copies of std cannot catch each other's. A panic poisons the slot; a reload clears it. See [linux-process-memory-injection](./linux-process-memory-injection.md)
- **In firmware**: prefer one-shot, self-expiring boot entries plus a firmware watchdog over any permanent boot configuration, so a remote hang always recovers into a known-good OS. Require a per-run watchdog budget that the protocol refuses to be zero, with a keepalive that proves the main loop is alive. Put identity in the handshake (a random instance id plus a boot count) when the same resident component can be replaced, restarted or resumed

## Sharp edges

- A cdylib with a `thread_local!` that has `Drop` can get `DF_1_NODELETE` and stay mapped (inferred in stardew-bot, not reproduced)
- Dropping a suspended coroutine force-unwinds its stack, so bot code must never `catch_unwind` around game calls
- A host that starts a runtime once per process (a hosted .NET) cannot reload
- A stale payload can kill the host at launch; build both sides in order from one recipe
- A UEFI hang at high TPL or with interrupts off defeats the firmware watchdog (EDK2's fires at `TPL_NOTIFY`)
- Watchdog reset paths were verified only under QEMU for some cases; real-hardware results are recorded in docs

## Anti-Patterns

- Overwriting a mapped library in place
- Unloading with live threads or an unreleased destructor gate
- Serializing payload state across generations
- A permanent boot entry for a development loader

## Related Topics

- [linux-process-memory-injection](./linux-process-memory-injection.md)
- [firmware-no-std-uefi](./firmware-no-std-uefi.md)
- [cross-language-type-generation](../dx/cross-language-type-generation.md): fingerprints

## Open Questions

- Whether a PCH TCO watchdog can be a hardware backstop for hangs the firmware watchdog cannot see
- A production `/proc/self/maps` unload check for stardew-bot's reload path
