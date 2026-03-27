---
name: image-processing-pipeline
category: architecture
last_audited: 2026-03-26
exemplars: []
---

# Image Processing Pipeline

## Philosophy

<!-- Capture once, transform on demand. Store originals, derive variants via CDN/proxy. Progressive loading with blur placeholders. -->

## Conventions

<!-- Framebuffer capture to WebP, thumbhash/dominant color extraction at ingest, imgproxy for dev transforms, cdn-cgi for production, progressive loading with blur-up -->

## Language-Specific

### Kotlin

<!-- Framebuffer capture, ARGB pixel array sharing between analysis and WebP writer -->

### Rust

<!-- Image analysis (hash, dominant color), storage routing, metadata extraction -->

### TypeScript

<!-- Progressive image loading, blur placeholder rendering, format negotiation via Accept header -->

## Anti-Patterns

<!-- Storing derived variants alongside originals, client-side image resizing, synchronous image processing in request handlers -->

## Open Questions

<!-- AVIF adoption timeline, video loop captures (WebM), perceptual hash for deduplication -->
