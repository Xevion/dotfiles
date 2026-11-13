---
name: build-systems
category: project-structure
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/xevion.dev
    path: tempo.config.ts
    note: "Current-model tempo config: flat tasks with tags and needs edges, presets.rust() spread into tasks, commands as tag selectors, codegen tasks with inputs/outputs, dev tasks with watch interrupt"
  - repo: Xevion/banner
    path: tempo config (sqlx prepare task)
    note: "sqlx offline metadata prepared against a migrations-only database so CI and local agree"
  - repo: local/Kessa
    path: Justfile (gate, lint, hakari recipes)
    note: "Rust-only workspace: plain Justfile that calls cargo directly and composes a gate recipe, no orchestrator"
  - repo: local/kk-engine
    path: Justfile + .config/nextest.toml
    note: "Justfile gate plus a dev binary for heavy checks; nextest only for fast hermetic tests"
  - repo: local/stardew-bot
    path: Justfile (decompile, unpack recipes)
    note: "One-off maintenance recipes as bash-shebang recipes with set -euo pipefail, everything else one-line delegation"
  - repo: local/maestro
    path: buildlock Gradle plugin
    note: "Cross-process file lock taken only around compile and test tasks, so a check can compile beside a live client"
  - repo: Xevion/doujin-ocr-summary
    path: scripts/lib/commands.ts
    note: (subsystem, action) -> CommandDef registry with typed enums (Bun-script era, not tempo)
---

# Build Systems

## Philosophy

A task runner at the root, language-native build tools underneath. Reproducible builds. Fast feedback loops. The task runner is an entry point, not a place to hide logic that needs tests.

There is no single answer yet for the runner itself. Plain Justfiles work for a Rust-only workspace but are showing their limits (no dependency graph, no staleness, no process supervision); tempo covers those needs for multi-language projects but its development has stalled. Treat both as current practice, not as settled doctrine.

## Conventions

- **Pick the runner by the project shape.**
  - Rust-only or single-runtime: a `Justfile` of one-line recipes calling cargo (or the language tool) directly, plus a `gate` aggregate recipe such as `gate: fmt-check lint test hakari-check`. No orchestrator
  - Multi-language or generated-artifact projects (Rust plus a web frontend, codegen, database): tempo, with the Justfile as a thin passthrough (`check *args: bunx tempo check {{args}}`)
  - A single-runtime web app whose `package.json` scripts cover every command without multi-line logic may use `package.json` alone
- **Language-native builds**: Cargo for Rust, Bun for TypeScript, Gradle KTS for Kotlin
- **Justfile recipes are one-liners that delegate.** Move logic past a few lines into a typed script. A one-off maintenance recipe with no other caller (decompiling an install, unpacking an archive) may stay as a short bash-shebang recipe with `set -euo pipefail`
- **mise for tool versions**: pin per project via `mise.toml`. Pin only tools not managed by the project's primary package manager; for Node/Bun projects use `package.json` `engines` for runtime constraints. Use `"latest"` for primary runtimes where drift is benign, exact pins for tools whose output changes frequently (linters, formatters)
- **mise tool installation patterns**: `"cargo:cargo-nextest" = "latest"` for Rust dev tools, `"npm:@xevion/tempo" = "latest"` for npm-based tools, with `[settings.npm] package_manager = "bun"`. Pin binaryen, wasm-bindgen-cli and zig exactly when a build depends on them matching a crate or ABI version
- **CI/local tool version drift**: tools pinned in CI must match `mise.toml`. See [dependency-management](../dx/dependency-management.md)
- **Pre-commit partial-staging detection**: build the set of partially-staged files before formatting; abort if the formatter modifies files in that set. Only re-stage files from the original staged set
- **`set dotenv-load` in Justfile** for projects that need environment-specific paths (GPU library locations, SDK roots)

### tempo (current model)

tempo is a task-graph engine. See [project-automation](../dx/project-automation.md) for the model in detail. In short: a flat list of tasks, each with a name, a body, tags, `needs` and `after` edges, optional `inputs` and `outputs` globs, and a `requires` list; subsystems are name prefixes (`web:`, `rust:`); a command is a tag selector (`check` runs everything tagged `check`).

- **Spread a preset, then override.** `presets.rust()` goes into `tasks`; override tasks that need workspace flags and add more (cargo-deny, cargo-machete, `RUSTDOCFLAGS=-D warnings`)
- **Derived artifacts are tasks, not hooks.** Codegen is a tagged task with `inputs` and `outputs`; dependents pull it in through `needs`. An aggregate `codegen` command regenerates everything
- **Separate verify tasks** (`bindings:verify`) diff against git with `git diff --exit-code`, so CI checks the same regeneration without writing to the tree
- **Dev servers are persistent tasks tagged `dev`**; the command uses `exitBehavior: "first-exits"`. A cargo-built server is `dev-build` then `dev`, split by a `needs` edge, with `watch: { interrupt: true }` so the old process releases its port before the new one binds
- **Database lifecycle** (`db start|reset|rm`) is a custom task that reads back the Docker-assigned host port and rewrites `.env`

### Other build patterns

- **Serialize concurrent builds with a cross-process lock** only around compile and test tasks (a reference-counted build service), so everything after the compile phase runs unguarded and a check can compile beside a live client
- **Prepare sqlx offline metadata against a migrations-only database.** The planner's nullability inference reads table statistics, so a populated dev database bakes row counts into `.sqlx` and CI, which migrates an empty one, rejects it
- **Randomized infrastructure ports**: random high-numbered ports (10000-60000) for Docker Compose services to avoid conflicts between stacks. Hardcode them in `docker-compose.yml` and expose via `.env` as `PORT`, `DB_PORT`, etc.

### Justfile micro-patterns

- **Alias shortcuts**: single-letter aliases for frequent recipes (`c := check`, `t := test`)
- **`[script("bun")]` recipes** for medium-complexity logic that does not warrant a script file
- **Recipe prerequisites that fetch what the build needs** (`just vendor` before check, lint and test) so a fresh clone works

## Anti-Patterns

- Makefiles for non-C projects (unless trivially simple)
- Shell scripts as the build system (untyped, hard to test, no error handling)
- Undocumented build steps that are not in the Justfile
- Orchestration logic inline in a Justfile when it has callers or needs a test
- **Platform-specific shells in a Justfile**: `set shell := ["powershell"]` breaks CI portability. Omit the override or detect the platform
- A recipe that mutates in a way its name does not say (a `check` recipe that runs `cargo fmt` without `--check`)
- **Taskfile (go-task)** is acceptable for Go-centric projects, with the same rule against compound `&&` chains

## Open Questions

- What replaces plain Justfiles when they run out of room, now that tempo's development has slowed
- Bazel/Buck2 for large multi-language projects
- Nix for fully reproducible builds
