---
name: build-systems
category: project-structure
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: Justfile
    note: Thin Justfile delegating all orchestration to Bun TypeScript scripts
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
- **mise for tool versions**: pin tool versions per-project via `.mise.toml`

## Anti-Patterns

- Makefiles for non-C projects (unless trivially simple)
- Shell scripts as build systems (untyped, hard to test, no error handling)
- Undocumented build steps that aren't in the Justfile
- Inline orchestration logic in Justfile recipes (> 3 lines = extract to a script)

## Open Questions

- Bazel/Buck2 for large multi-language projects
- Nix for fully reproducible builds
