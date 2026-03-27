---
name: image-processing-pipeline
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: local/inkwell
    path: internal/server/generate_stream.go + web/src/lib/stores/generation.svelte.ts
    note: "SSE streaming generation with per-phase write deadlines, detached context for S3 upload"
  - repo: Xevion/xevion.dev
    path: src/media_processing.rs + src/og.rs
    note: "Upload-time variant generation (thumb/medium/full + blurhash), SSR-rendered OG images via internal endpoint"
---

# Image Processing Pipeline

## Philosophy

Capture once, transform on demand. Store originals, derive variants via CDN/proxy or at ingest time. Progressive loading with blur placeholders.

## Conventions

- **SSE streaming for multi-phase generation**: emit distinct SSE phase events (generating with preview, uploading, complete) with per-phase write deadline extensions. Use per-phase timeout constants rather than a single stream timeout to match varying latency
- **Detached context for persistence**: when persisting the final result (S3 upload + DB insert), detach from the request context so the operation survives browser disconnect
- **Upload-time variant generation**: produce multiple WebP size variants (thumb/medium/full) plus the preserved original, with a blurhash for progressive loading. Extract blurhash from a downscaled thumbnail (32x32) for efficiency
- **SSR-rendered OG images**: a typed spec enum defines all page variants; the backend calls an internal SSR endpoint (not a public route) to render the image, then stores the bytes in object storage under a deterministic key derived from the spec. Avoids headless browser dependencies by reusing the SSR runtime

## Language-Specific

### Kotlin

<!-- Framebuffer capture, ARGB pixel array sharing between analysis and WebP writer -->

### Rust

- Image decode/resize/encode operations are CPU-bound and must be wrapped in `tokio::task::spawn_blocking` when called from async handlers

### TypeScript

- Model multi-phase image generation as a discriminated union state type where each phase carries its data (preview, step, dimensions). Carry preview data forward across transitions to prevent UI blank flashes. Use `AbortSignal.timeout` + `Promise.race` on each stream chunk read to detect stalled connections

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
