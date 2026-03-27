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
  - repo: Xevion/glint
    path: Dockerfile + docker/
    note: "cargo-chef dependency caching, Bun runtime co-hosting SvelteKit + Rust binary"
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

- **cargo-chef for dependency caching**: three-stage pattern — `planner` (generates `recipe.json` from dependency tree), `cook` (builds dependencies only from recipe), `build` (compiles application code). Dependency layer changes only when `Cargo.toml`/`Cargo.lock` change
- **Co-located services with shared runtime base**: when running a Rust binary alongside a Node/Bun frontend in one container, use the frontend runtime image (e.g., `oven/bun:1-slim`) as the final stage. Copy the compiled Rust binary in. A single entrypoint script orchestrates both processes
- **Console-logger preload**: inject a preload script that captures stray `console.*` calls from SSR code and reformats them to match the structured logging format

### Python

<!-- uv for fast installs, multi-stage with build deps separated from runtime -->

## Anti-Patterns

<!-- Fat images with build toolchains, rebuilding heavy ML deps for code changes, hardcoded secrets in Dockerfiles -->

## Open Questions

<!-- Docker BuildKit cache mounts vs layer caching, Podman compatibility, rootless containers -->
