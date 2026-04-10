---
name: platform-abstraction-layer
category: architecture
last_audited: 2026-04-10
exemplars:
  - repo: Xevion/borde.rs
    path: frontend/src/shared/api/
    note: "Transport interface with sendJson/onBinary/onJson, separate WASM worker and Tauri IPC implementations"
  - repo: local/toriix
    path: src/fs.rs + src/app.rs
    note: "Rust cfg-gated module re-export for WASM/native bifurcation — alternative to runtime trait injection"
  - repo: local/ts-chan
    path: src/core/platform.ts + src/core/storage.ts
    note: "Runtime capability detection for userscript environments (typeof GM vs chrome.storage), StorageBackend interface abstraction"
  - repo: local/game-hacking
    path: crates/external/ + crates/internal/
    note: "External tooling vs injected-payload crate split — different runtime constraints force two separate toolchains"
---

# Platform Abstraction Layer

## Philosophy

Interface-first design that decouples application logic from runtime environment. The application code programs against a Transport/Platform interface; platform-specific implementations (WASM worker, Tauri IPC, native TCP) are injected at startup. Build-time constants select the implementation without runtime branching in hot paths.

## Conventions

- **Transport interface**: define a minimal interface (sendJson, onBinary, onJson, close) that captures the communication contract. Platform implementations provide the concrete transport (Web Worker postMessage, Tauri invoke/listen, WebSocket)
- **Build-time platform selection**: use Vite define (`__DESKTOP__`) or similar compile-time constants to select the Transport implementation at build time. Avoids runtime feature detection overhead and enables tree-shaking of unused implementations
- **Separate entry points per target**: each platform gets its own entry point that wires the correct Transport implementation into the shared application code
- **Rust `cfg`-gated module re-export as an alternative to interface injection**: when the platform boundary is coarse (one or two modules, not cross-cutting) and runtime switching is not needed, `#[cfg(not(target_family = "wasm"))] mod native;` + `#[cfg(target_family = "wasm")] mod mock;` + `pub use` to unify the interface is a zero-cost Rust-native alternative to trait injection. The module re-exports appear as a single symbol at the consumer; only one implementation is compiled per build. Use when: (a) exactly two targets exist, (b) runtime switching is not needed, (c) the platform boundary is one to three modules. Escalate to a trait-injected Transport interface when the boundary fans out across many call sites
- **Runtime capability detection for userscript-style environments**: when a single built artifact must run under multiple host environments (Tampermonkey vs Violentmonkey vs Chrome extension vs fallback), build-time constants cannot disambiguate — the same `.user.js` loads in all four. Detect capabilities via `typeof` guards (`typeof GM_getValue !== "undefined"`, `typeof chrome?.storage?.sync !== "undefined"`) evaluated once at module init and cached. Back the result with a `StorageBackend`-style interface that callers depend on. See [userscript-extension-dual-target](./userscript-extension-dual-target.md)
- **Crate-split for divergent runtime constraints (Rust FFI payloads)**: when one binary needs a full Rust runtime (external tools) and another must run under a foreign process's allocator and signal handlers (injected `.so`, embedded library), split into separate crates with different `crate-type` settings. The external crate links `std` normally; the internal crate is `cdylib` only with `panic = "abort"` and careful `.init_array` handling. Never share dependencies — internal payloads must pull in only what's safe to run in the host process. See [linux-process-memory-injection](./linux-process-memory-injection.md) for the injected-payload constraints

## Anti-Patterns

- Duck-typed capability detection (`"method" in obj`) with inline casts — indicates an incomplete interface. Either add the method to the interface or define an extended interface
- Runtime platform detection in hot paths when build-time selection is possible
- Duplicating application logic per platform instead of abstracting the platform boundary

## Open Questions

- Graceful degradation when a platform capability is genuinely optional (not just missing from the interface)
