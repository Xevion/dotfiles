---
name: linux-process-memory-injection
category: architecture
last_audited: 2026-04-10
exemplars:
  - repo: local/game-hacking
    path: crates/game-core/src/mem.rs + crates/injector/
    note: "process_vm_readv/writev external reads, ptrace+dlopen injection, Pod-boundary glam conversion, SIGUSR1 hot-reload"
---

# Linux Process Memory Injection

## Philosophy

Runtime process manipulation is a distinct discipline from static reverse engineering. [Binary reverse engineering](../patterns/binary-reverse-engineering.md) asks *what does this code do?*; memory injection asks *what is this process doing right now, and how do I change it?* The Linux kernel exposes a rich surface for this — `/proc/<pid>/mem`, `process_vm_readv/writev`, `ptrace`, `LD_PRELOAD`, `memfd_create` — each with different tradeoffs around speed, stealth, and failure modes. A well-structured memory-injection project separates the *external* toolchain (scanners, injectors, signature finders) from the *internal* payloads (injected `.so` shared objects with hooked functions) because the two contexts have fundamentally different constraints: external tools have a full Rust/Zig runtime and can crash freely; internal payloads run under the host process's allocator, signal handlers, and thread scheduler.

## Conventions

- **External vs internal split at the crate/module boundary** — `crates/external/` for tools that read the target process from the outside (memory scanners, pointer-chain resolvers, ELF parsers, pattern scanners). `crates/internal/` for `.so` payloads loaded via `dlopen` into the target (hooks, ESP overlays, aimbots). Never share binary crates between the two — internal payloads must be `cdylib` only, and linking in external dependencies pulls in `std` startup code that can deadlock in `.init_array`
- **`process_vm_readv`/`process_vm_writev` for bulk external I/O** — faster than `/proc/<pid>/mem` (no open/seek/read cycle) and doesn't require the target to be stopped. Requires `CAP_SYS_PTRACE` or same UID + `prctl(PR_SET_DUMPABLE, 1)` on the target. Validate the return value: partial transfer indicates partial region accessibility and should be treated as an error, not silent truncation
- **`/proc/<pid>/maps` region enumeration with safety filters** — before bulk-scanning memory, enumerate regions via `proc_maps::get_process_maps` (or equivalent). Filter out: regions larger than a sanity threshold (64 MB catches most runaway mappings), `/dev/*` (GPU buffers, may hang on read), `[vvar]`/`[vsyscall]` (kernel virtual areas), and anonymous executable mappings that look like JIT code you don't want to touch
- **`ptrace` for register/instruction-level operations** — `PTRACE_ATTACH` + `PTRACE_PEEKDATA`/`POKEDATA` is word-at-a-time (slow for bulk) but required for register manipulation, single-stepping, and writing to regions `process_vm_writev` can't reach. Use for the *injection bootstrap* (remote `mmap` call, shellcode write, `dlopen` invocation); switch to `process_vm_readv/writev` for subsequent bulk memory work
- **`dlopen` shellcode via remote syscall** — the canonical injection path: `PTRACE_ATTACH`, find `libc.so`'s `__libc_dlopen_mode` or `dlopen` symbol in the target, write a small shellcode blob that calls it with the path to your `.so`, `PTRACE_POKEUSER` to jump to the blob, single-step until return, restore registers, `PTRACE_DETACH`. `LD_PRELOAD` is the zero-effort alternative when you control process launch; `memfd_create` + exec is the modern stealth alternative when the target binary is itself the launcher
- **Pointer-chain resolution for ASLR-resilient addresses** — direct addresses change every launch (ASLR). Store pointer chains as `[base_offset, deref_1, deref_2, ..., final_offset]`, resolve by reading the base module's load address from `/proc/<pid>/maps` and walking. Cache resolved pointers within a session; re-resolve on process restart
- **Pod-boundary math type conversion** — raw memory reads use `[f32; 3]` `bytemuck::Pod` structs that exactly match the target's struct padding. Immediately convert to `glam::Vec3`/`Mat4` at the read helper (e.g., `VmMem::read_vec3()`). Downstream code NEVER touches raw arrays — all game-logic math goes through glam types. See [rust](../languages/rust.md) Pod-boundary convention
- **SIGUSR1 hot-reload for internal payloads** — the injected `.so` installs a `SIGUSR1` handler that re-`dlopen`s the updated library from disk. External build tooling rebuilds the `.so` and sends `kill -USR1 <pid>`. Keeps dev iteration loop at `cargo build && kill -USR1` without reattach
- **Injected `.so` tracing: bypass Rust's stderr lock** — Rust's `std::io::Stderr` takes a global lock that can deadlock inside `.init_array` (the dynamic loader holds the linker lock, and the first `println!` takes the stderr lock). Write a `RawStderr` writer that calls `libc::write(2, ...)` directly. Avoid `EnvFilter` in `.init_array` — regex compilation touches TLS that isn't set up yet. See [logging-observability](../patterns/logging-observability.md) injected-library section

## Internal Payload Constraints

When writing the injected `.so`:

- **`cdylib` crate type only** — never `rlib` (pulls in full `std` startup), never `dylib` (uses Rust ABI)
- **`#[no_mangle] extern "C"` exports** — any function the injector needs to find via `dlsym` must be C-ABI
- **No `panic = "unwind"`** — set `panic = "abort"` in the profile; unwinding across the C++ host's stack is undefined behavior
- **Zero allocation in signal handlers** — host signal handlers run on the host's stack; Rust's allocator may not be safe to call
- **`thread_local!` is fragile** — the host's TLS layout may not have space for new slots; prefer global `Mutex<T>` or `atomic::*`
- **`.init_array` constructors run during `dlopen`** — they hold the linker lock, so no `dlopen`/`dlsym` inside them. Defer initialization to the first exported-function call

## Anti-Patterns

- **Shared dependencies between external and internal crates** — the internal crate pulls in the external crate's `std` startup code and deadlocks
- **Bulk memory scans without `/proc/pid/maps` filtering** — hangs or SIGBUS on `/dev/*` and `[vvar]` regions
- **Hardcoded absolute addresses** — ASLR breaks them on every launch; pointer chains or signature scans are required
- **`println!` in injected `.so` `.init_array`** — deadlocks on stderr lock during load
- **`LD_PRELOAD` for targets you don't launch** — only works at process start; use `dlopen` injection for running processes
- **Detaching with `PTRACE_DETACH` without restoring registers** — the target resumes at the shellcode address and crashes

## Legality and Scope

This pattern belongs in legitimate contexts: your own software, CTF challenges, open-source game modding (e.g., AssaultCube, ioquake3), authorized security research, and RE education. Applying it to anti-cheated multiplayer games violates their terms of service; commercial binary analysis requires license review. The corpus documents the technique; scope is the caller's responsibility.

## Open Questions

- `ptrace` restrictions under YAMA (`/proc/sys/kernel/yama/ptrace_scope`) — canonical handling for scope=1 (only-children) and scope=2 (admin-only)?
- `io_uring` as an alternative to `process_vm_readv` for batched async reads across many target PIDs?
