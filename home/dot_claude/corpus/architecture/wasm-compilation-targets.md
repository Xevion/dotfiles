---
name: wasm-compilation-targets
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/Pac-Man
    path: pacman/src/platform/
    note: "Dual-target Rust (native + Emscripten WASM), platform-gated modules, extern C FFI"
  - repo: Xevion/borde.rs
    path: crates/borders-wasm/ + Justfile
    note: "wasm-bindgen target, three WASM Cargo profiles, mtime-based wasm-bindgen skip, cfg-gated deps"
  - repo: Xevion/WebSAM
    path: src/lib/inference/ + src/routes/wasm/
    note: "ONNX Runtime Web (consumer-side WASM), .ort pre-optimization, stable /wasm/ API route bypassing Vite hashing"
---

# WASM Compilation Targets

## Philosophy

Write once, compile to multiple targets. Platform abstraction at the module boundary. WASM is a compilation target, not a rewrite. Target-specific code is isolated behind cfg gates.

## Conventions

- **Two WASM pathways**: `wasm32-unknown-emscripten` (Pac-Man: SDL2 game loop via `emscripten_set_main_loop_arg`, extern "C" FFI) and `wasm32-unknown-unknown` with wasm-bindgen (borde.rs: Web Worker game loop via `gloo-timers`, JS interop). Choose based on whether you need Emscripten's POSIX emulation or prefer wasm-bindgen's lighter JS interop
- **cfg-gated dependency sections**: separate `[target.'cfg(target_arch = "wasm32")'.dependencies]` from native deps. WASM gets wasm-bindgen, gloo-timers, web-sys; native gets tokio full, reqwest native-tls
- **Three-tier WASM Cargo profiles**: `wasm-dev` (opt-level 1, debug info), `wasm-release` (opt-level z, LTO, strip, panic=abort, wasm-opt -Oz), `wasm-debug` (opt-level 0, full debug). Each inherits from the appropriate base profile
- **Mtime-based wasm-bindgen skip**: compare artifact timestamp before/after cargo build; skip wasm-bindgen when unchanged and pkg artifacts already exist. Avoids redundant postprocessing during incremental development
- **wasm-opt postprocessing**: run `wasm-opt -Oz` with `--enable-bulk-memory --enable-threads` on release builds for additional size reduction beyond what rustc/LTO achieves

## Language-Specific

### Rust

- `wasm32-unknown-emscripten`: `emscripten_set_main_loop_arg` for browser game loop, ASYNCIFY for async operations, platform module with cfg-gated impls
- `wasm32-unknown-unknown` + wasm-bindgen: manual update loop with `gloo-timers::future::sleep` for browser game loop in Web Worker context. `wasm-bindgen --target web` for ES module output

### Consumer-Side WASM (ONNX Runtime Web)

- **Stable API route for WASM file serving**: when WASM files are resolved by name at runtime (e.g., onnxruntime-web), Vite's content-hashing breaks the resolution. Serve WASM files through a SvelteKit API route (e.g., `/wasm/[filename]/+server.ts`) at stable paths, fetching from object storage (R2). This separates WASM runtime resolution from the build tool's asset pipeline
- **.ort pre-optimization for WebGPU EP**: convert `.onnx` models offline to `.ort` format via `python3 -m onnxruntime.tools.convert_onnx_models_to_ort` before deployment. Raw ONNX models with shape annotation mismatches crash the WebGPU EP's transpose optimizer. Baking runtime optimizations offline sidesteps the crash. Keep decoders as `.onnx` when they work without issues

## Anti-Patterns

- Duplicating platform-specific code instead of abstracting behind cfg gates
- Blocking the browser event loop (use ASYNCIFY or async sleep)
- Ignoring WASM binary size (always use wasm-opt on release builds)
- Running all postprocessing unconditionally on incremental rebuilds

## Open Questions

- WASI adoption timeline and when it replaces Emscripten for server-side WASM
- Shared memory and threads in WASM (atomics, SharedArrayBuffer requirements)
