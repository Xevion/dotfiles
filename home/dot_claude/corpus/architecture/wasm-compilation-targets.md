---
name: wasm-compilation-targets
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/Pac-Man
    path: pacman/src/platform/
    note: "Dual-target Rust (native + Emscripten WASM), platform-gated modules, extern C FFI"
---

# WASM Compilation Targets

## Philosophy

<!-- Write once, compile to multiple targets. Platform abstraction at the module boundary. WASM is a compilation target, not a rewrite. -->

## Conventions

<!-- #[cfg(target_os = "emscripten")] for platform gating, extern "C" FFI to Emscripten APIs, ASYNCIFY for browser game loop -->

## Language-Specific

### Rust

<!-- wasm32-unknown-emscripten target, emscripten_set_main_loop_arg for browser game loop, platform module with cfg-gated impls -->

## Anti-Patterns

<!-- Duplicating platform-specific code instead of abstracting, blocking the browser event loop, ignoring WASM binary size -->

## Open Questions

<!-- wasm32-unknown-emscripten vs wasm32-unknown-unknown + wasm-bindgen tradeoffs, WASI adoption timeline -->
