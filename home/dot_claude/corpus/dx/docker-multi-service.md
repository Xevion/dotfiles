---
name: docker-multi-service
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/doujin-ocr-summary
    path: Dockerfile + docker-compose.yml
    note: Multi-stage Go build with UPX compression, Bun runtime base, Compose with Postgres/MinIO/sidecar
  - repo: Xevion/instant-upscale
    path: docker/
    note: Deps-only ML base image with hot-sync for model code iteration
---

# Docker & Multi-Service Orchestration

## Philosophy

<!-- Docker for reproducible deploys and local dev parity. Compose for multi-service local development. Layer caching as a first-class optimization concern. -->

## Conventions

<!-- Multi-stage builds, deps-only layers, runtime base matching entrypoint, Compose for local dev -->

## Language-Specific

### Go

<!-- go mod download as cached layer, CGO_ENABLED=0 for static binaries, UPX for size reduction -->

### Rust

<!-- cargo-chef for dependency caching (planner → cook → build), minimal runtime image -->

### Python

<!-- uv for fast installs, multi-stage with build deps separated from runtime -->

## Anti-Patterns

<!-- Fat images with build toolchains, rebuilding heavy ML deps for code changes, hardcoded secrets in Dockerfiles -->

## Open Questions

<!-- Docker BuildKit cache mounts vs layer caching, Podman compatibility, rootless containers -->
