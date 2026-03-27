---
name: linux-hardware-interfaces
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/ferrite
    path: src/phys.rs + src/edac.rs + src/smbios.rs
    note: /proc/self/pagemap batch pread, EDAC sysfs counters, SMBIOS Type 17 DIMM topology
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
