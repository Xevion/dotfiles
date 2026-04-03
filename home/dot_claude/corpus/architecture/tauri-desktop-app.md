---
name: tauri-desktop-app
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: local/novel
    path: src-tauri/ + src/
    note: Tauri 2.x with Pest parser, Svelte 5 frontend, ts-rs type generation
---

# Tauri Desktop App

## Philosophy

Thin Rust backend, rich web frontend. The Rust layer owns state, parsing, and system access; the frontend owns presentation and interaction. The IPC boundary is a typed contract — both sides agree on the shape of every command and event.

## Conventions

- **Typed command errors**: implement `serde::Serialize` on a dedicated error enum (or the domain error type) so Tauri commands return `Result<T, CommandError>` instead of `Result<T, String>`. This preserves typed errors at the IPC boundary and lets the frontend match on error variants
- **ts-rs for IPC contracts**: apply ts-rs to all types crossing the IPC boundary. See [cross-language-type-generation](../dx/cross-language-type-generation.md) for detailed conventions
- **Tracing instrumentation**: Tauri backends benefit from `tracing` even in desktop apps. Add `#[instrument(skip(state, handle))]` on commands and initialize `tracing_subscriber` in the Tauri `setup()` callback before `run()`. Gives structured visibility into command execution without a log server
- **State management**: `Arc<Mutex<T>>` behind Tauri's `State` extractor. Drop the lock before any `.await` point. Emit events via `app_handle.emit("event", payload)?` for real-time frontend updates

## Language-Specific

### Rust (Backend)

- Tauri 2.x with `#[tauri::command]` derive
- Custom DSL parsing via Pest PEG grammars — define grammar in `.pest` file, `pest_derive` generates typed rule enums, walk the parse tree to build a typed AST
- `cargo nextest` for Rust tests, no CLI tests (thin wrapper pattern)

### Svelte (Frontend)

- Svelte 5 runes for component state, but migrate global stores from `writable()`/`derived()` to module-scoped `$state` (see [svelte](../languages/svelte.md))
- Tauri IPC via `invoke()` from `@tauri-apps/api/core`, event listening via `listen<T>()` from `@tauri-apps/api/event`
- Types imported from auto-generated bindings (`$lib/bindings`), never hand-maintained

## Anti-Patterns

- `Result<T, String>` on Tauri commands — erases error types, forces string matching on the frontend
- Per-field `#[serde(rename)]` instead of struct-level `rename_all` — unnecessary maintenance burden
- Migration history sections in CLAUDE.md — once migration is complete, they provide no forward-looking context
- Manual "Sync Types" instructions in CLAUDE.md when ts-rs already automates generation — contradicts the actual workflow and misleads AI agents

## Open Questions

- WebKitGTK performance constraints (e.g., `box-shadow` is slow) and workarounds
- Tauri plugin ecosystem maturity for system integration (notifications, tray, deep links)
