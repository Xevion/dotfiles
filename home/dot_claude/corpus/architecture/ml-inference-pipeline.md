---
name: ml-inference-pipeline
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: local/topaz-video-ai-re
    path: inference/src/pipeline.rs
    note: Tiled ONNX inference with cosine-window blending, parallel workers, EMA outlier detection
---

# ML Inference Pipeline

## Philosophy

Local inference as a first-class deployment target. The pipeline should handle arbitrary input sizes via tiling, adapt to available hardware via runtime execution provider selection, and surface performance characteristics through structured observability.

## Conventions

- **Tiled spatial inference**: for models with fixed spatial input sizes, decompose the full frame into overlapping tiles, run inference per tile, and reconstruct via weighted blending (cosine/Hann window). Pre-compute weight maps once per pipeline configuration to avoid per-frame overhead
- **Execution provider selection**: auto-detect GPU capabilities at startup, preferring TensorRT > CUDA > CPU. Make backend selectable via CLI flag for debugging. Log the selected provider and device at startup
- **CPU-bound thread pool for inference**: use `std::thread` + `crossbeam-channel` (bounded work queue, unbounded result queue) rather than Tokio tasks. Named threads via `thread::Builder::new().name(...)` for traceability. Inference calls are long-running and blocking — async would add overhead without benefit
- **Dual-channel error separation**: separate result and error into distinct crossbeam channels for the thread pool. The error channel carries fatal worker state (session init failure, GPU OOM); the result channel stays clean. The coordinator uses `crossbeam::select!` to race both with a timeout arm
- **EMA-based outlier detection**: use an exponential moving average (α=0.05) as a running baseline in the inference loop and emit a structured warning when a measurement exceeds 5×EMA. Lighter than a sliding window and naturally adapts to warm-up. Treat the first tile specially — skip outlier detection and use a longer timeout (e.g., 600s for TensorRT engine builds vs 10s steady state)
- **Frame pipeline**: FFmpeg decode (subprocess pipe) → raw RGB frames → tile splitting → ONNX inference (per-tile, parallel workers) → tile blending → RGB output → FFmpeg encode (subprocess pipe)

## Language-Specific

### Rust

- `ort` crate for ONNX Runtime bindings (dynamic linking, no download-binaries feature)
- Extension trait (`OrtResultExt`) to convert ort's opaque errors to `anyhow::Error` with a single `.ort()` call
- `Arc<AtomicBool>` quit flag with `Ordering::Relaxed` for cooperative cancellation in sync thread loops — simpler than `CancellationToken` when polling cost is negligible inside a per-frame loop

## Anti-Patterns

- Running inference in async Tokio tasks — GPU inference calls block for tens to hundreds of milliseconds
- Single global timeout instead of per-tile adaptive timeouts
- Hardcoded tile sizes without measuring the tiling overhead vs GPU utilization tradeoff

## Open Questions

- Automatic tile size selection based on available VRAM
- fp16 vs fp32 precision selection based on GPU architecture (fp16 broken on some architectures with certain ORT versions)
