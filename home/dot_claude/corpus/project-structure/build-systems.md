---
name: build-systems
category: project-structure
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/banner
    path: Justfile
    note: Thin Justfile delegating all orchestration to Bun TypeScript scripts
  - repo: Xevion/tempo
    path: Justfile + .mise.toml
    note: Justfile self-delegation, mise scoped to non-npm tools
  - repo: local/maestro
    path: scripts/check.ts + scripts/pre-commit.ts
    note: Parallel check with auto-fix loop, partial-staging safety
  - repo: Xevion/doujin-ocr-summary
    path: scripts/lib/commands.ts
    note: (subsystem, action) → CommandDef registry with typed enums
  - repo: local/bose-re
    path: Justfile + tempo.config.ts
    note: "Justfile as pure passthrough to tempo, preset-override-extend pattern"
  - repo: Xevion/rustdoc-mcp
    path: Justfile + tempo.config.ts
    note: "tempo preset-override-extend with dep-check (cargo-machete) and deny (cargo-deny), fix-first autofix"
---

# Build Systems

## Philosophy

Just as the task runner. Language-native build tools underneath. Justfile is the CLI entry point — a thin wrapper, not the orchestration engine. Reproducible builds. Fast feedback loops.

## Conventions

- **Justfile for project commands**: every project gets a `Justfile` (capital J) at the root. Recipes are one-liners that delegate to language-native scripts or tools
- **Delegation pattern**: Justfile delegates to typed scripts (Bun/TypeScript, Python, etc.) for any logic beyond a single command. Keeps Justfile minimal (< 100 lines) while putting orchestration in a testable, type-safe language

```just
# Pattern: Justfile as thin wrapper
check *args:
    bun scripts/check.ts {{args}}
format *targets:
    bun scripts/format.ts {{targets}}
test *args:
    bun scripts/test.ts {{args}}
```

- **Language-native builds**: Cargo for Rust, Bun for TypeScript, Gradle KTS for Kotlin
- **mise for tool versions**: pin tool versions per-project via `.mise.toml`. Pin only tools not managed by the project's primary package manager — for Node/Bun projects use `package.json` `engines` for runtime constraints and mise for the rest
- **mise tool installation patterns**: `"cargo:cargo-nextest" = "latest"` for Rust dev tools, `"npm:@xevion/tempo" = "latest"` for npm-based tools. Use `[settings.npm] package_manager = "bun"` to force mise to use Bun for npm: tool installations. Use `"latest"` for primary runtimes (rust, bun) but exact pins for CI-sensitive tools whose output changes frequently
- **mise env injection**: `[env] RUSTUP_TOOLCHAIN = "nightly"` for project-scoped nightly Rust — cleaner than `rustup override` or `rust-toolchain.toml` when the nightly requirement is development-only
- **CI/local tool version drift**: CI-sensitive tools (Go linters, formatters) must be pinned to the same version in `mise.toml` and CI config. `"latest"` resolves at install time — if CI resolves it a week later than local, lint rules may differ. See [dependency-management](../dx/dependency-management.md#cilocal-tool-version-synchronization) for the full convention
- **Typed TypeScript orchestrator config**: own subsystem definitions, autoFix pairing, dev process specs, and preflight checks in a config file (like `tempo.config.ts`). Justfile as pure pass-through. Enables IDE completion and cross-subsystem coordination
- **Justfile self-delegation**: tools that manage dev workflows should use themselves for their own development. Validates the tool in real use
- **Pre-commit partial-staging detection**: build the set of partially-staged files before formatting; abort if the formatter modifies files in that set. Only re-stage files from the original staged set
- **Delegated check scripts**: parallel execution of independent checks (format, compile, lint, test) with auto-fix loop — if only formatting failed and all peers passed, apply the formatter and re-verify
- **tempo as the primary dev workflow tool**: `@xevion/tempo` is the strongly recommended default for all new projects. Provides typed subsystem definitions, check/test/dev orchestration, preflight hooks, custom commands with flag parsing, and auto-fix pairing — all via `tempo.config.ts`. Justfile serves as a thin passthrough only (`check *args: bunx tempo check {{args}}`)
- **tempo subsystem architecture**: define subsystems (frontend, backend, infra) with aliases, per-subsystem commands (`format-check`, `format-apply`, `lint`, `type-check`, `build`), auto-fix mappings (`format-check` → `format-apply`), and `requires` for optional tool dependencies. Each subsystem declares its directory and runtime
- **tempo preset-override-extend pattern**: in `tempo.config.ts`, spread a typed preset (`presets.rust()`), selectively override commands that need workspace flags, and extend with additional checks (cargo-deny, cargo-machete, doc-check with `RUSTDOCFLAGS=-D warnings`). The `requires` field on each check communicates optional tool dependencies and skips gracefully when absent
- **tempo preflights**: pre-check hooks that run before commands — node_modules existence, PandaCSS codegen staleness, ts-rs bindings generation, SQLx query preparation. Preflights ensure derived artifacts are fresh before checks run
- **tempo custom commands**: extend beyond check/test/dev with project-specific commands (`deploy`, `db`, `migrate`, `bindings`, `docker-build`) with typed flag definitions. Commands can have hooks (`before:dev`, `before:check`) for conditional setup
- **`set dotenv-load` in Justfile**: standard header for projects that need environment-specific paths (GPU library locations, custom SDK roots). Auto-loads `.env` variables into recipe execution

### Justfile Micro-Patterns

- **Alias shortcuts**: map single-letter aliases to frequent recipes (`c := check`, `d := dev`, `t := test`, `l := lint`). Saves keystrokes in daily use
- **`[script("bun")]` for complex recipes**: embed TypeScript directly in Justfile recipes for medium-complexity logic that doesn't warrant a separate script file. Use for benchmark harnesses, dynamic port allocation, or conditional workflows
- **Randomized infrastructure ports**: use random high-numbered ports (10000-60000) for Docker Compose services to avoid conflicts when multiple compose stacks run in parallel. Ports are hardcoded in `docker-compose.yml` and exposed via `.env` as `PORT`, `FRONTEND_PORT`, `DB_PORT`, etc. for Justfile/tempo consumption

## Anti-Patterns

- Makefiles for non-C projects (unless trivially simple)
- Shell scripts as build systems (untyped, hard to test, no error handling)
- Undocumented build steps that aren't in the Justfile
- Inline orchestration logic in Justfile recipes (> 3 lines = extract to a script)
- **Platform-specific shells in Justfile**: `set shell := ["powershell"]` breaks CI portability. For cross-platform projects, either omit the shell override or provide a platform-detected fallback. Complex recipes requiring platform-specific shells should be extracted to a script
- **No Justfile threshold**: for single-runtime web apps where `package.json` scripts cover all commands without multi-line logic, `package.json` as the sole task runner is acceptable. Add a Justfile when commands require orchestration logic, multi-step delegation, or cross-tool coordination beyond `&&`-chained npm scripts
- **Taskfile (go-task) as alternative**: acceptable for Go-centric projects. Note that inline `echo` + actual command pairs and compound `&&` chains in Taskfile recipes are the same anti-pattern as in Justfile — extract to scripts

## Open Questions

- Bazel/Buck2 for large multi-language projects
- Nix for fully reproducible builds
