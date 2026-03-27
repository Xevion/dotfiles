---
name: multi-target-build-pipeline
category: dx
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/borde.rs
    path: Justfile + .github/workflows/builds.yml
    note: "WASM + Tauri desktop + server from one codebase, 5-platform CI matrix, Vite build modes"
---

# Multi-Target Build Pipeline

## Philosophy

Coordinate a single codebase that compiles to multiple distinct targets (WASM browser, Tauri desktop, native server) with shared source and per-target CI. Build-time constants and separate entry points select target-specific code paths without runtime branching.

## Conventions

- **Vite build modes for target selection**: use `define` in Vite config to inject compile-time constants (`__DESKTOP__`) that select platform-specific imports. Each target gets a separate `tsconfig` (browser, desktop, node) with appropriate lib and module settings
- **Per-target Cargo profiles**: define separate Cargo profiles for each WASM build mode (wasm-dev, wasm-release, wasm-debug) with target-appropriate settings (opt-level, lto, strip, panic=abort). Native builds use the standard dev/release profiles
- **Mtime-based incremental build detection**: compare artifact timestamps before/after `cargo build` to skip redundant postprocessing steps (wasm-bindgen, wasm-opt) during incremental development
- **Multi-platform CI matrix**: build for all target platforms (Linux/macOS x86+arm/Windows x86+arm64) plus browser WASM as separate parallel jobs. Gate deployment jobs on build success with `needs:` + `if: github.event_name == 'push'`

## Anti-Patterns

- Duplicating source code per target instead of sharing via platform abstraction
- Running all postprocessing steps unconditionally on incremental rebuilds
- Platform-specific shell in Justfile (`set shell := ["powershell"]`) breaking CI portability

## Open Questions

- Cargo workspace vs separate crates for per-target code
- Universal binary strategies (fat binaries, multi-arch containers)
