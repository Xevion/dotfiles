---
name: object-storage-patterns
category: architecture
last_audited: 2026-03-26
exemplars: []
---

# Object Storage Patterns

## Philosophy

<!-- S3-compatible APIs as the storage abstraction. Local dev parity via MinIO. Two-phase uploads for reliability. -->

## Conventions

<!-- Two-phase upload (initiate → direct upload → confirm), key naming conventions, orphan cleanup reconciliation, imgproxy/cdn-cgi for on-the-fly transforms -->

## Language-Specific

### Rust

<!-- aws-sdk-s3 or s3 crate, presigned URLs for direct upload, background cleanup tasks -->

### TypeScript

<!-- Frontend direct-to-R2 upload via presigned URL, progressive image loading with blur placeholders -->

## Anti-Patterns

<!-- Storing blobs in the database, hardcoded bucket names, no orphan cleanup strategy -->

## Open Questions

<!-- R2 vs S3 cost tradeoffs, image format negotiation (WebP/AVIF), CDN cache invalidation strategies -->
