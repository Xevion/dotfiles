---
name: gpu-shader-pipeline
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/kk-engine
    path: kk-render renderer wgsl module (gpu_struct!, attributes!, numbered!, Group/Binding)
    note: "Anything shared between Rust and WGSL is declared once in Rust and the WGSL text is generated; tests validate the assembled source with naga"
  - repo: Xevion/borde.rs
    path: borders-look build.rs, gpu snapshot tests
    note: "One WGSL source validated by naga and translated to HLSL, MSL and GLSL ES at build time, with Rust consts emitted for the layout"
  - repo: local/currents-render
    path: shader-check helpers, per-pass shader tests
    note: "naga validation plus translation to GLSL ES 3.00 the way the WebGL2 backend will, override constants checked with the real values"
---

# GPU Shader Pipeline

## Philosophy

A shader and the Rust that feeds it describe the same layout. Declare it once and generate the other side, so the two cannot drift, and validate every shader on a machine with no GPU.

## Conventions

- **Declare shared layout once, in Rust.** Bind groups, entry points, vertex layouts, uniform structs and shader constants come from Rust tables, and the WGSL text is generated (`gpu_struct!` writes the WGSL `struct` and fails to compile where WGSL would place a field elsewhere). Never mirror by hand, never test drift by matching strings
- **Name shader constants from Rust variant identifiers**, not from a `strum` string, so a rename is a compile error rather than a silent mismatch
- **Alternatively, make the WGSL the single source** and generate the Rust mirror: `build.rs` joins the parts, validates with naga, translates to HLSL, MSL and GLSL ES, and emits Rust consts for the uniform block, bindings and backend register numbers. Snapshot tests pin the translated binding interfaces so a backend drift shows up as a diff
- **Validate every pass in unit tests with naga** (`Validator::new(ValidationFlags::all(), Capabilities::default())`), on the assembled source
- **Translate to GLSL ES 3.00 in tests the way the WebGL2 backend will**, so a shader that works on one backend cannot silently break the other. Check pipeline override constants with the values the real pass uses. Add a reserved-name check (a WGSL identifier `gl` becomes `gl_1` in GLSL)
- **Embed shader source with `include_str!`** and keep a directory-read loader so edits apply without a rebuild
- **A multi-pass frame**: a coarse field pass and a fine shade pass with separate resolution knobs, temporal blending of frames shaded at shifted points in place of supersampling, settings applied only between frames, and GPU faults recorded and shown in the UI rather than exiting

## Anti-Patterns

- Hand-mirrored struct layouts in two languages
- Shader constants derived from display strings
- Validating shaders only by running them on a GPU

## Related Topics

- [headless-gpu-rendering](../dx/headless-gpu-rendering.md)
- [build-time-codegen](../dx/build-time-codegen.md)
- [native-gpu-ui](./native-gpu-ui.md)
