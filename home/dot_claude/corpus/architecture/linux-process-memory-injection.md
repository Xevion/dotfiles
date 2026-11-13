---
name: linux-process-memory-injection
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/game-hacking
    path: game-core memory module, injector, lab loader and runtime, AC internal host
    note: "process_vm_readv/writev external reads, ptrace+dlopen injection, glam-in-Pod structs, a permanent loader with per-generation payload reload, catch_unwind at every extern boundary"
---

# Linux Process Memory Injection

## Philosophy

Runtime process manipulation is a distinct discipline from static reverse engineering. [Binary reverse engineering](../patterns/binary-reverse-engineering.md) asks *what does this code do?*; memory injection asks *what is this process doing right now, and how do I change it?* The Linux kernel exposes a rich surface for this (`/proc/<pid>/mem`, `process_vm_readv/writev`, `ptrace`, `LD_PRELOAD`, `memfd_create`), each with different tradeoffs around speed, visibility and failure modes. A well-structured project separates the *external* toolchain (scanners, injectors, signature finders) from the *internal* payloads (shared objects with hooked functions), because the two contexts have different constraints: external tools have a full runtime and can crash freely; internal payloads run under the host process's allocator, signal handlers and thread scheduler.

## Conventions

- **External and internal split at the crate boundary.** External tools read the target from outside (memory scanners, pointer-chain resolvers, pattern scanners). Internal payloads are `cdylib` crates loaded into the target (hooks, overlays). A plain library crate of domain types and math may be shared between the two. What must not be shared is a crate that pulls external-only dependencies or runtime startup code into the payload
- **`process_vm_readv`/`process_vm_writev` for bulk external I/O**: faster than `/proc/<pid>/mem` and does not require the target to be stopped. Needs `CAP_SYS_PTRACE` or same UID plus `prctl(PR_SET_DUMPABLE, 1)` on the target. Treat a partial transfer as an error, not silent truncation
- **`/proc/<pid>/maps` region enumeration with safety filters** before bulk scanning. Filter out regions larger than a sanity threshold (64 MB catches most runaway mappings), `/dev/*` (GPU buffers may hang on read), `[vvar]`/`[vsyscall]`, and anonymous executable mappings that look like JIT code
- **`ptrace` for register and instruction-level operations**: `PTRACE_ATTACH` plus `PEEKDATA`/`POKEDATA` is word-at-a-time and slow for bulk, but required for registers, single-stepping and regions `process_vm_writev` cannot reach. Use it for the injection bootstrap, then switch to `process_vm_readv/writev`
- **`dlopen` shellcode via remote syscall**: `PTRACE_ATTACH`, find `dlopen` in the target's libc, write a small blob that calls it with the path to your `.so`, jump to it, single-step until return, restore registers, `PTRACE_DETACH`. `LD_PRELOAD` is the zero-effort alternative when you control process launch; `memfd_create` plus exec is the alternative when the target binary is itself the launcher
- **Pointer-chain resolution for ASLR-resilient addresses**: store chains as `[base_offset, deref_1, ..., final_offset]`, resolve from the base module's load address in `/proc/<pid>/maps`, cache within a session, re-resolve on restart
- **glam types inside Pod structs**: with glam's `bytemuck` feature, `Vec3` and `Mat4` can sit directly inside `bytemuck::Pod` `#[repr(C)]` structs. Declare offsets as attributes on the fields of one class-level layout struct, with no separate offset constants. A raw `[f32; 3]` in a Pod struct that is meant to be a `Vec3` is the mistake. See [rust](../languages/rust.md)
- **Hot reload through a permanent loader.** Keep the injected loader tiny and never unload it; the runtime is a replaceable library loaded as a fresh generation. See [reloadable-rust-payloads](./reloadable-rust-payloads.md). A `SIGUSR1` handler that re-`dlopen`s the library from disk is the simpler form for a payload with no threads or thread-local destructors
- **Injected `.so` tracing: bypass Rust's stderr lock.** Rust's `Stderr` takes a global lock that can deadlock inside a constructor (the loader holds the linker lock). Write a `RawStderr` that calls `libc::write(2, ...)` directly, and avoid `EnvFilter` early because regex compilation touches TLS that is not set up yet. See [logging-observability](../patterns/logging-observability.md)

## Internal Payload Constraints

- **`cdylib` crate type** for the payload itself
- **`#[no_mangle] extern "C"` exports** for anything the injector finds via `dlsym`
- **Unwinding is acceptable if every function the host calls is wrapped.** A panic that crosses an `extern "system"` boundary is undefined behavior, so wrap every handler the host calls into in `catch_unwind` and isolate faults per module so one faulty module cannot take down the host. Reserve `panic = "abort"` for the public or distribution profile. Two copies of std cannot catch each other's panics, so each side catches its own
- **Constructors may start initialization if they hand the work to a new thread** and never take the loader lock (no `dlopen` or `dlsym` inside them). Keep an explicit init export as a fallback, and make init idempotent with an atomic swap. A payload that acts only when the process name and environment match is safe to preload
- **Zero allocation in signal handlers**: host signal handlers run on the host's stack
- **`thread_local!` with a destructor is fragile**: the C library registers the destructor and keeps the library mapped. For a reloadable payload, route destructors through a loader-owned gate. See [reloadable-rust-payloads](./reloadable-rust-payloads.md)
- **A per-hook fault budget** (for example 5 faults in 60 seconds disables the hook and ends its stream) and drop-newest event streams with a `Lagged(n)` count, so the game thread never waits on a consumer
- **Hook at a safe point.** Prefer observing hooks; replace a function only where the argument and return contract are known

## Anti-Patterns

- **Bulk memory scans without `/proc/pid/maps` filtering**: hangs or SIGBUS on `/dev/*` and `[vvar]`
- **Hardcoded absolute addresses**: ASLR breaks them on every launch
- **`println!` in an injected `.so` constructor**: deadlocks on the stderr lock during load
- **`LD_PRELOAD` for targets you do not launch**: only works at process start
- **Detaching with `PTRACE_DETACH` without restoring registers**: the target resumes at the shellcode address and crashes
- **A panic crossing a host-facing `extern` boundary without `catch_unwind`**

## Legality and Scope

This pattern belongs in legitimate contexts: your own software, CTF challenges, open-source game modding, authorized security research, RE education, and community servers whose rules are very relaxed. On such servers, analysing the anti-cheat components themselves (PunkBuster, FairFight and their hooks) was within scope, because those servers still run them and the anti-cheat there is weak. Elsewhere, check the target's terms of service, and commercial binary analysis requires license review. The corpus documents the technique; scope is the caller's responsibility.

## Open Questions

- `ptrace` restrictions under YAMA (`/proc/sys/kernel/yama/ptrace_scope`): canonical handling for scope 1 (only children) and scope 2 (admin only)?
- `io_uring` as an alternative to `process_vm_readv` for batched async reads across many target PIDs?
