---
name: object-storage-patterns
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: local/inkwell
    path: internal/storage/ + docker-compose.yml
    note: "S3Client with imgproxy URL construction, MinIO + imgproxy compose for local dev"
  - repo: Xevion/xevion.dev
    path: src/r2.rs
    note: "opendal S3 adapter with OnceCell graceful degradation, best-effort delete with orphan logging"
  - repo: Xevion/WebSAM
    path: scripts/upload.ts
    note: "R2 upload with HeadObject idempotency check, immutable cache headers"
---

# Object Storage Patterns

## Philosophy

S3-compatible APIs as the storage abstraction. Local dev parity via MinIO. DB record is authoritative for existence; storage is best-effort.

## Conventions

- **Local dev parity via MinIO + imgproxy compose**: use a one-shot init container (`minio/mc`) to automate bucket creation and ACL setup after the MinIO health check passes. Define imgproxy presets as environment variable config so callsites only pass a preset name. Enable `IMGPROXY_AUTO_WEBP=true` for format negotiation without client-side format selection
- **imgproxy URL construction**: wrap imgproxy URL construction in a method on the storage client that accepts a preset name parameter (defaulting to a sensible preset). Fallback to direct public URL when imgproxy is unconfigured — zero branching at the callsite. Use `s3://bucket/key` source URIs for imgproxy to keep URL construction independent of the storage endpoint
- **Best-effort delete with orphan logging**: on delete, attempt storage cleanup before removing the DB record. If storage deletion fails, log a warning with the storage prefix for later reconciliation but continue to delete the DB record — the DB record is the authoritative existence check
- **Graceful degradation when storage is unconfigured**: use `OnceCell::get_or_try_init` returning `Option<Arc<Client>>` — the app starts without media caching, logs a warning on init failure, and callers handle `None`. Prefer over panicking on missing credentials when storage is not critical to the core request path

## Language-Specific

### Rust

- **opendal over aws-sdk-s3**: opendal's S3 service adapter provides a unified `Operator` API for put/delete/list with significantly lighter build weight than `aws-sdk-s3`. Prefer opendal when only basic S3 operations are needed

### Go

- **aws-sdk-go-v2 with startup connectivity check**: wrap the S3 client with a `HeadBucket` check at startup and an `IsConfigured` guard for degraded-mode operation

### TypeScript

- **Idempotent upload via HeadObject size-match**: before uploading large objects (model weights, binary assets), issue a `HeadObjectCommand` check. If the object exists and `ContentLength` matches the expected size, skip the upload entirely. Safe for immutable objects with `Cache-Control: public, max-age=31536000, immutable`. Prevents re-uploading multi-hundred-MB files on script re-runs

## Anti-Patterns

- Storing blobs in the database
- Hardcoded bucket names or imgproxy URLs at callsites
- No orphan cleanup strategy — always log storage prefix on failed cleanup for reconciliation
- Panicking on missing storage credentials when storage is an optional feature

## Open Questions

- R2 vs S3 cost tradeoffs
- Image format negotiation (WebP/AVIF) strategies
- CDN cache invalidation patterns
