---
name: native-gpu-ui
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/borde.rs
    path: borders-client
    note: "GPUI client for native and browser from one crate, vendored and patched GPUI, frame pacing and a heartbeat watchdog"
  - repo: local/currents-render
    path: viewer pacing and x11 modules
    note: "GPUI-based viewer, frame pacing as a pure function of power and fullscreen state, X11 desktop-layer window hints"
  - repo: local/Kessa
    path: kessa-app tasks and memo, egui_kittest fixtures
    note: "egui app: a task type that aborts on drop, revision-keyed memos, offscreen fixture rendering"
  - repo: local/kk-engine
    path: viewer worker lanes (loader, cards, poses)
    note: "Generation tickets that drop stale jobs, panics returned as typed failures, preview-then-restore"
---

# Native GPU UI

Rust GPU-drawn UIs: GPUI (borde.rs, currents-render, toriix) and egui (Kessa, kk-engine). GPUI is young, and the practice here moves with it.

## Philosophy

Keep the frame read-only and cheap, move work off the frame onto cancellable tasks, and make UI state testable by rendering fixtures offscreen.

## Conventions

- **A background task is a handle that aborts when dropped.** It wraps a `oneshot` receiver and an abort handle. A slot given a new task supersedes the old one. Workers call `request_repaint` when done. Tests run the same call inline so results are ready on return. Steam-style event streams use an `mpsc` channel plus a wake closure
- **Memoize derived views by a revision counter that every edit bumps**, so a search narrows in the frame it is typed. Each frame only reads
- **Worker lanes for a viewer**: a loader, a cards lane and a poses lane, each with generation `Ticket`s that drop stale jobs as `Superseded`, cancellable loads, and panics returned as typed `Done::Failed` instead of unwinding into the UI. A preview-then-restore stash keeps the replaced character whole
- **Drive the real app in tests** (`egui_kittest`), with a fixture crate that builds a made-up install and named window states rendered to PNG for review. Layout tests flip one thing and assert that no labelled widget moved. See [visual-regression-testing](../patterns/visual-regression-testing.md)
- **Frame pacing is a pure function** of power state, visibility and fullscreen, so it is unit-testable. Derive the frame budget from the monitor refresh rate
- **Embedding in a desktop layer** (X11): set below, sticky, skip-taskbar and skip-pager hints and follow a parent window's size through `ConfigureNotify`
- **A heartbeat watchdog** around a simulation worker in the browser, so a stalled worker is detected rather than frozen silently
- **One crate for native and browser** is possible; split platform code with cfg-gated module pairs. See [platform-abstraction-layer](./platform-abstraction-layer.md) and [wasm-compilation-targets](./wasm-compilation-targets.md)

## Anti-Patterns

- Doing loading or decoding on the frame
- A background task that keeps running after the thing that wanted its result is gone
- State derived on every frame instead of memoized by revision

## Open Questions

- **Vendoring and patching GPUI.** borde.rs and currents-render both vendor and patch GPUI extensively, as does toriix. It works today, but the maintenance cost and hackiness are acknowledged. One candidate is a single repository of the owner's own maintained fork that all projects depend on, in place of each project rolling its own vendoring. Nothing is settled, and this is not a preference against vendoring: do not pick a worse workaround to avoid it. Upstream (Zed, and the wider ecosystem) may reduce the need, and Cargo itself may make patching easier
- Whether GPUI's trajectory (more adoption, for instance from large products moving to it) changes the answer
