---
name: image-processing-pipeline
category: architecture
last_audited: 2026-04-10
exemplars:
  - repo: local/inkwell
    path: internal/server/generate_stream.go + web/src/lib/stores/generation.svelte.ts
    note: "SSE streaming generation with per-phase write deadlines, detached context for S3 upload"
  - repo: Xevion/xevion.dev
    path: src/media_processing.rs + src/og.rs
    note: "Upload-time variant generation (thumb/medium/full + blurhash), SSR-rendered OG images via internal endpoint"
  - repo: Xevion/WebSAM
    path: src/lib/inference/
    note: "Encode-once decode-many SAM pipeline with iterative mask feedback, cached logits for interactive re-threshold"
  - repo: local/stashapp-rapid-tag
    path: src/lib/server/frames.ts + src/lib/server/vtt.ts
    note: "WebVTT sprite ingestion, sharp extract, composite quality score (blur+exposure+entropy), dHash Hamming dedup, WebP upload to MinIO"
---

# Image Processing Pipeline

## Philosophy

Capture once, transform on demand. Store originals, derive variants via CDN/proxy or at ingest time. Progressive loading with blur placeholders.

## Conventions

- **SSE streaming for multi-phase generation**: emit distinct SSE phase events (generating with preview, uploading, complete) with per-phase write deadline extensions. Use per-phase timeout constants rather than a single stream timeout to match varying latency
- **Detached context for persistence**: when persisting the final result (S3 upload + DB insert), detach from the request context so the operation survives browser disconnect
- **Upload-time variant generation**: produce multiple WebP size variants (thumb/medium/full) plus the preserved original, with a blurhash for progressive loading. Extract blurhash from a downscaled thumbnail (32x32) for efficiency
- **SSR-rendered OG images**: a typed spec enum defines all page variants; the backend calls an internal SSR endpoint (not a public route) to render the image, then stores the bytes in object storage under a deterministic key derived from the spec. Avoids headless browser dependencies by reusing the SSR runtime

## Sprite-Sheet Ingestion with Quality-Score Filtering

When the source of frames is a video sprite sheet (thumbnail grid + WebVTT timing manifest — Stash, Plex, Jellyfin, chapter thumbnails), the pipeline is ingestion-oriented rather than per-upload.

- **Parallel VTT + sprite fetch**: fetch the WebVTT manifest and the sprite JPEG in parallel; the VTT gives `timestamp → #xywh` crop descriptors, the sprite provides the pixels. Parse the VTT with a forgiving hand-rolled parser — browser `vtt-parser` libraries assume the cue format but sprite sheets use `#xywh=x,y,w,h` annotations that are technically invalid
- **Sharp `extract()` per entry**: for each VTT cue, call `sharp(sprite).extract({ left, top, width, height })` and pipe to a composite quality scorer. Sharp's extract is zero-copy for JPEG sources
- **Composite quality score (blur + exposure + entropy)**: `qualityScore = 0.4 * blurNorm + 0.3 * exposureScore + 0.3 * entropyNorm`, where `blurNorm` is Laplacian variance normalized to `[0, 1]`, `exposureScore` penalizes extreme means (< 30 or > 220), and `entropyNorm` is Shannon entropy over the grayscale histogram. All three are computed from a single downscaled grayscale pass for efficiency. Threshold at `0.3` as a lower-bound filter; use the exact score as the evidence weight in downstream Bayesian classification
- **Perceptual-hash deduplication via dHash**: compute a 64-bit difference hash (9×8 grayscale resize → horizontal gradient → binary string). Compare against existing hashes via Hamming distance; threshold ≤ 10 bits indicates a near-duplicate. Dedup within a single video removes bursts of static frames; dedup across videos can identify recurring scenes
- **WebP upload with quality=80**: after filtering, `sharp().webp({ quality: 80 })` and `putFrame()` to S3/MinIO. WebP at 80 gives ~5-10× size reduction vs the original JPEG with perceptually negligible loss
- **Pair with Bayesian classification**: quality score flows through as the evidence weight in [scoring-ranking-algorithms](./scoring-ranking-algorithms.md) Bayesian pipeline — high-quality frames update the posterior more confidently than low-quality ones

## Language-Specific

### Kotlin

- Framebuffer capture, ARGB pixel array sharing — to be populated from future project audits

### Rust

- Image decode/resize/encode operations are CPU-bound and must be wrapped in `tokio::task::spawn_blocking` when called from async handlers
- **Tiled ML inference**: for models with fixed spatial input sizes, decompose the full frame into overlapping tiles, run inference per tile, and reconstruct via weighted blending (cosine/Hann window). Pre-compute weight maps once per pipeline configuration. See [ml-inference-pipeline](./ml-inference-pipeline.md) for the full pattern

### TypeScript

- Model multi-phase image generation as a discriminated union state type where each phase carries its data (preview, step, dimensions). Carry preview data forward across transitions to prevent UI blank flashes. Use `AbortSignal.timeout` + `Promise.race` on each stream chunk read to detect stalled connections
- **Encode-once, decode-many pipeline**: for segmentation models (SAM), separate the expensive encode phase (image→embedding, cached per image) from the cheap decode phase (embedding+prompt→masks). The decoder accepts a `lowResMasks` input from the previous result for iterative refinement — each click refines the previous mask. Cache raw logits so threshold/smooth re-processing can run without re-inference

### Go

- For diffusion model streaming, emit distinct SSE phases with per-phase write deadline extensions via `http.NewResponseController.SetWriteDeadline`. Detach the persistence context from the request context

## Anti-Patterns

- Storing derived variants alongside originals without a cleanup strategy
- Client-side image resizing for upload
- Synchronous image processing in async request handlers (use `spawn_blocking`)
- Single global stream timeout instead of per-phase deadlines

## Open Questions

- AVIF adoption timeline
- Video loop captures (WebM)
- Perceptual hash for deduplication
