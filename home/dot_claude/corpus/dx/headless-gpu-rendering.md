---
name: headless-gpu-rendering
category: dx
last_audited: 2026-03-26
exemplars: []
---

# Headless GPU Rendering

## Philosophy

<!-- GPU-accelerated rendering in containerized environments. Virtual displays for headless OpenGL. Graceful fallback to software rendering. -->

## Conventions

<!-- Xvfb as virtual display, VirtualGL for GPU passthrough, NVIDIA runtime container, llvmpipe fallback, X11 window focus management, ffmpeg recording -->

## Anti-Patterns

<!-- Assuming GPU availability without fallback, hardcoded display numbers, no health checks for GPU state -->

## Open Questions

<!-- EGL headless rendering vs Xvfb, Vulkan compute as alternative, multi-GPU scheduling -->
