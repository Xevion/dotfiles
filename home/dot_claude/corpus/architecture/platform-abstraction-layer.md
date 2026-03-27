---
name: platform-abstraction-layer
category: architecture
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/borde.rs
    path: frontend/src/shared/api/
    note: "Transport interface with sendJson/onBinary/onJson, separate WASM worker and Tauri IPC implementations"
---

# Platform Abstraction Layer

## Philosophy

Interface-first design that decouples application logic from runtime environment. The application code programs against a Transport/Platform interface; platform-specific implementations (WASM worker, Tauri IPC, native TCP) are injected at startup. Build-time constants select the implementation without runtime branching in hot paths.

## Conventions

- **Transport interface**: define a minimal interface (sendJson, onBinary, onJson, close) that captures the communication contract. Platform implementations provide the concrete transport (Web Worker postMessage, Tauri invoke/listen, WebSocket)
- **Build-time platform selection**: use Vite define (`__DESKTOP__`) or similar compile-time constants to select the Transport implementation at build time. Avoids runtime feature detection overhead and enables tree-shaking of unused implementations
- **Separate entry points per target**: each platform gets its own entry point that wires the correct Transport implementation into the shared application code

## Anti-Patterns

- Duck-typed capability detection (`"method" in obj`) with inline casts — indicates an incomplete interface. Either add the method to the interface or define an extended interface
- Runtime platform detection in hot paths when build-time selection is possible
- Duplicating application logic per platform instead of abstracting the platform boundary

## Open Questions

- Graceful degradation when a platform capability is genuinely optional (not just missing from the interface)
