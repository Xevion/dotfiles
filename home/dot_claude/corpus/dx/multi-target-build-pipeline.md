---
name: multi-target-build-pipeline
category: dx
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/borde.rs
    path: Justfile (lint platform=native|windows|macos, gate) + CI cross-lint job
    note: "Cargo-only pipeline: one check and lint recipe covering native and browser, Windows and macOS cross-linted from Linux, platform-specific code gated in build.rs so every platform still lints clean. (The earlier WASM + Tauri + Vite build-mode pipeline was replaced)"
  - repo: Xevion/dynamic-preauth
    path: Dockerfile
    note: "Dual-target cross-compilation (Linux + Windows x64) in Docker with cargo-chef per target"
---

# Multi-Target Build Pipeline

## Philosophy

Coordinate a single codebase that compiles to multiple distinct targets (WASM browser, Tauri desktop, native server) with shared source and per-target CI. Build-time constants and separate entry points select target-specific code paths without runtime branching.

## Conventions

- **Cross-lint other platforms from one host.** `cargo xwin clippy --target x86_64-pc-windows-msvc` for Windows and `cargo zigbuild clippy --target aarch64-apple-darwin` for macOS (with `MACOSX_DEPLOYMENT_TARGET` set and a stub `xcrun` first on PATH for build scripts that probe it). Gate platform code the host cannot compile in `build.rs` so each platform still lints clean. A `gate` recipe composes fmt-check, lint, hakari-check, machete and test
- **Vite build modes for target selection** (the web-and-Tauri form): use `define` in Vite config to inject compile-time constants (`__DESKTOP__`) that select platform-specific imports. Each target gets a separate `tsconfig` (browser, desktop, node) with appropriate lib and module settings
- **Per-target Cargo profiles**: define separate Cargo profiles for each WASM build mode (wasm-dev, wasm-release, wasm-debug) with target-appropriate settings (opt-level, lto, strip, panic=abort). Native builds use the standard dev/release profiles
- **Mtime-based incremental build detection**: compare artifact timestamps before/after `cargo build` to skip redundant postprocessing steps (wasm-bindgen, wasm-opt) during incremental development
- **Multi-platform CI matrix**: build for all target platforms (Linux/macOS x86+arm/Windows x86+arm64) plus browser WASM as separate parallel jobs. Gate deployment jobs on build success with `needs:` + `if: github.event_name == 'push'`

- **Cross-compilation in Docker with cargo-chef**: for projects that build the same Rust crate for multiple targets (e.g., `x86_64-unknown-linux-gnu` + `x86_64-pc-windows-gnu`), use a single Docker stage with cargo-chef dependency caching per target. Apply `strip` to the final binaries. This adapts the WASM dual-target pattern to cross-compilation without requiring separate build environments

## Anti-Patterns

- Duplicating source code per target instead of sharing via platform abstraction
- Running all postprocessing steps unconditionally on incremental rebuilds
- Platform-specific shell in Justfile (`set shell := ["powershell"]`) breaking CI portability

## Open Questions

- Cargo workspace vs separate crates for per-target code
- Universal binary strategies (fat binaries, multi-arch containers)
