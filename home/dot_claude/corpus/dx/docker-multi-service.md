---
name: docker-multi-service
category: dx
last_audited: 2026-04-03
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
  - repo: local/inkwell
    path: Dockerfile + web/entrypoint.ts
    note: "Bun-orchestrated co-hosting with health-gated startup, Go+SvelteKit in one container"
  - repo: Xevion/xevion.dev
    path: Dockerfile
    note: "6-stage build with placeholder frontend for Rust dep cache, SQLX_OFFLINE=true"
  - repo: Xevion/Pac-Man
    path: pacman-server/Dockerfile
    note: "9-stage dual-target (WASM + native) with emsdk base, artifact size verification"
  - repo: Xevion/dynamic-preauth
    path: Dockerfile
    note: "cargo-chef dual-target (Linux + Windows x64) demo build, non-root runtime user, stripped binaries"
---

# Docker & Multi-Service Orchestration

## Philosophy

<!-- Docker for reproducible deploys and local dev parity. Compose for multi-service local development. Layer caching as a first-class optimization concern. -->

## Conventions

<!-- Multi-stage builds, deps-only layers, runtime base matching entrypoint, Compose for local dev -->

## Language-Specific

### Go

- **`go mod download` as cached deps layer**: analogous to cargo-chef. `CGO_ENABLED=0` for static binary + UPX compression for size reduction
- Match runtime base image to the actual entrypoint runtime (e.g. Bun, not Alpine) when co-hosting

### Rust

- **cargo-chef for dependency caching**: three-stage pattern — `planner` (generates `recipe.json` from dependency tree), `cook` (builds dependencies only from recipe), `build` (compiles application code). Dependency layer changes only when `Cargo.toml`/`Cargo.lock` change
- **Co-located services with shared runtime base**: when running a Rust binary alongside a Node/Bun frontend in one container, use the frontend runtime image (e.g., `oven/bun:1-slim`) as the final stage. Copy the compiled Rust binary in. A single entrypoint script orchestrates both processes
- **Console-logger preload**: inject a preload script that captures stray `console.*` calls from SSR code and reformats them to match the structured logging format
- **Bun-orchestrated co-hosting entrypoint**: a TypeScript entrypoint that health-gates SSR before starting the backend binary, propagates shared env vars (`LOG_JSON`, `LOG_LEVEL`) to both processes, and monitors with `Promise.race` for first-to-exit shutdown
- **6-stage Dockerfile with placeholder frontend**: planner → builder (placeholder assets for dep cache) → frontend builder → final-builder (real assets + `SQLX_OFFLINE=true`) → runtime. The placeholder stage allows Rust dependency caching to work even though the final binary embeds frontend assets
- **WASM dual-target build**: for projects targeting both native and WASM, use a separate `emscripten/emsdk` base image as a parallel cargo-chef stage. Include build-artifact size verification (`test -f ./static/pacman.wasm && [ $(stat -c%s ...) -gt ... ]`) before proceeding — zero-byte WASM artifacts are a common silent failure mode

### Python

<!-- uv for fast installs, multi-stage with build deps separated from runtime -->

## Anti-Patterns

<!-- Fat images with build toolchains, rebuilding heavy ML deps for code changes, hardcoded secrets in Dockerfiles -->

## Open Questions

<!-- Docker BuildKit cache mounts vs layer caching, Podman compatibility, rootless containers -->
