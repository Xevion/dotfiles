---
name: headless-gpu-rendering
category: dx
last_audited: 2026-11-12
exemplars:
  - repo: local/currents-render
    path: test-display recipe, CI test job, windows-smoke recipe, viewer instance construction
    note: "Window tests ignored behind an env guard under xvfb-run -a, lavapipe forced with WGPU_BACKEND=vulkan, cross-compiled build checked by rendering the same frame under Wine"
  - repo: local/assrip
    path: render headless module (Headless::new, Headless::blocking)
    note: "Windowless wgpu instance on the primary backends with GL excluded, caller-supplied closure for optional features"
  - repo: local/kk-engine
    path: nextest gpu test group, KK_REQUIRE_GPU
    note: "GPU tests throttled in their own group, a required-GPU switch that turns a skip into a failure"
---

# Headless GPU Rendering

## Philosophy

GPU code must be testable on machines with no display and no GPU, without a fixed display number and without silently testing less than it claims. Use a virtual display for tests that open real windows, a software Vulkan driver where there is no GPU, and a windowless device for everything that only renders.

## Conventions

- **Window tests are ignored by default and guarded.** Mark tests that open real windows `#[ignore = "reason"]`, make them refuse to run unless an env variable set by the recipe is present (`CURRENTS_TEST_DISPLAY=1`), and run them through `xvfb-run -a -s "-screen 0 1920x1080x24" cargo nextest run ... --run-ignored only`. `-a` picks a free display, so no fixed display number is assumed and the real desktop is never touched
- **Force software Vulkan on GPU-less runners.** Set `WGPU_BACKEND=vulkan` so Mesa's lavapipe is used instead of llvmpipe's OpenGL, which would otherwise be picked. Build the wgpu instance with the env-reading constructor (`new_with_display_handle_from_env`, or `new_without_display_handle().with_env()`) so the variable reaches adapter choice. Keep the window tests off CI if the runner cannot host them, and say so, rather than letting CI look like it covers them
- **A windowless device for pure rendering.** Create the instance on the primary backends only, with GL excluded because its EGL probe unwraps a null display off the main thread. Take a closure for optional features, always request block-compression, and map a missing adapter to a named no-GPU error instead of a panic
- **Verify a cross-compiled build by rendering the same frame natively and under its target runtime** (Wine or Proton on a virtual display), then comparing the mean per-channel difference against a fixed threshold (3.0 out of 255). Stop the runtime's server in a `finally`. Disable the accessibility helper (`PROTON_USE_XALIA=0`) because it stalls on a display with no desktop
- **GPU tests live in their own test group.** Throttle them (a `gpu` nextest group with a small `max-threads`), and select them by binary id plus name regex. A required-GPU switch (`KK_REQUIRE_GPU=1`) turns "no GPU" from a skip into a failure when you mean to cover it. Move a test's filter when the test moves crate
- **Record GPU faults and show them, do not exit.** A render fault goes to a recorded fault state surfaced in the UI

## Anti-Patterns

- Assuming a GPU is present with no fallback
- A hardcoded display number
- GPU tests that silently skip and let a run look green
- Picking llvmpipe's OpenGL when software Vulkan is what the real code path uses
- No health check for GPU state before a long render

## Related Topics

- [gpu-shader-pipeline](../architecture/gpu-shader-pipeline.md): validating shaders without a GPU
- [native-gpu-ui](../architecture/native-gpu-ui.md): GPUI and egui apps
- [visual-regression-testing](../patterns/visual-regression-testing.md): comparing renders against accepted baselines

## Open Questions

- EGL headless rendering vs Xvfb for the window tests
- Containerized GPU passthrough (NVIDIA runtime) for CI that must exercise a real device
- Multi-GPU scheduling
