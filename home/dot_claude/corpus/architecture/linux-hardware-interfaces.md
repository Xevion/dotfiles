---
name: linux-hardware-interfaces
category: architecture
last_audited: 2026-04-10
exemplars:
  - repo: Xevion/ferrite
    path: src/phys.rs + src/edac.rs + src/smbios.rs
    note: /proc/self/pagemap batch pread, EDAC sysfs counters, SMBIOS Type 17 DIMM topology
  - repo: local/game-hacking
    path: crates/game-core/src/mem.rs
    note: "Cross-process process_vm_readv/writev, /proc/pid/maps region enumeration with safety filters, Pod→glam conversion at read boundary"
---

# Linux Hardware Interfaces

## Philosophy

Build a coherent hardware view by composing multiple low-level kernel interfaces. Each interface (proc, sysfs, SMBIOS) provides a narrow slice of truth; the application layer merges them into a unified model.

## Conventions

- **Virtual-to-physical address resolution**: read `/proc/self/pagemap` with batch `pread` (8 bytes per page entry) to resolve virtual addresses to physical page frame numbers. Pair with `/proc/kpageflags` for page metadata. Requires `CAP_SYS_ADMIN` or root
- **EDAC sysfs monitoring**: read `/sys/devices/system/edac/mc*/` counters for correctable/uncorrectable ECC error deltas between test phases. Parse the directory hierarchy (memory controller → CSROW → channel) to map errors to physical DIMM locations
- **SMBIOS Type 17 parsing**: read DMI entries for DIMM identification (manufacturer, part number, speed, size). Merge with EDAC topology to produce a unified "failing module" report
- **mlock for physical memory control**: `mmap` anonymous pages then `mlock` to prevent swapping and ensure physical residency. Requires sufficient `RLIMIT_MEMLOCK` — document the privilege escalation options (`root`, `ulimit -l unlimited`, `CAP_IPC_LOCK`)
- **Page mapping stability verification**: re-read `/proc/self/pagemap` between test passes to verify the kernel hasn't migrated physical pages. Migration would invalidate physical-address-based error correlation

## Cross-Process Memory I/O

Reading/writing memory *of another process* (as opposed to self-inspection like pagemap) is a distinct interface family. The same project may use both: self-inspection for diagnostics, cross-process for manipulation.

- **`process_vm_readv`/`process_vm_writev` for bulk I/O**: faster than `/proc/<pid>/mem` (no open/seek/read cycle) and doesn't require stopping the target. Requires `CAP_SYS_PTRACE` or same UID + `/proc/sys/kernel/yama/ptrace_scope ≤ 0`. **Validate the return value** — partial transfer indicates partial region accessibility and must be treated as an error, not silent truncation. The local/remote iovec API allows scatter-gather in a single syscall
- **`/proc/<pid>/maps` region enumeration with safety filters**: before any bulk scan, enumerate readable regions and filter. Skip: regions larger than a sanity threshold (64 MB catches runaway mappings), `/dev/*` (GPU buffers, may hang on read or SIGBUS), `[vvar]`/`[vsyscall]` (kernel virtual areas), anonymous executable regions that look like JIT code you don't own. Use `proc-maps` (Rust) or parse directly
- **`ptrace` PEEKDATA/POKEDATA for register-level work**: word-at-a-time and slow for bulk, but required for register reads/writes, single-stepping, and writing to pages `process_vm_writev` can't reach. Use `ptrace` for injection bootstrap (remote `mmap` call, shellcode write, `dlopen` invocation); switch to `process_vm_readv/writev` for subsequent bulk work once the target library is loaded
- **Pod→glam conversion at the read boundary**: helpers like `VmMem::read_vec3(addr)` read a raw `[f32; 3]` Pod struct matching target memory layout and convert immediately to `glam::Vec3`. Downstream code never touches raw arrays. See [rust](../languages/rust.md) Pod-boundary convention
- **YAMA ptrace_scope awareness**: document the scope policy in CLAUDE.md/README. Scope 0 = any-same-UID allowed, 1 = children-only (default on Ubuntu), 2 = admin only, 3 = disabled. Tools that depend on cross-process I/O need either root, `CAP_SYS_PTRACE`, or `prctl(PR_SET_PTRACER, target_pid)` called from within the target

See [linux-process-memory-injection](./linux-process-memory-injection.md) for the full injection/manipulation architecture.

## Language-Specific

### Rust

- `nix` crate for `mmap`/`mlock`/`pread` syscall wrappers. Wrap in a typed `AllocError` enum with `#[source] nix::Error`
- AVX-512 non-temporal stores (`_mm512_stream_si512` + `_mm_sfence`) for cache-bypassing memory writes. Gate behind `is_x86_feature_detected!("avx512f")` with scalar fallback
- Rayon `par_chunks_mut` / `par_chunks` for parallel write-then-verify sweeps. Rayon's join barrier guarantees sfence visibility before the verify phase

## Anti-Patterns

- Reading pagemap one entry at a time — batch with `pread` for performance
- Assuming physical page stability across test passes without verification
- Hardcoding DIMM topology instead of parsing SMBIOS dynamically

## Open Questions

- NUMA-aware allocation and per-node testing strategies
- Kernel huge page interaction with mlock and pagemap resolution
