---
name: visual-regression-testing
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: local/kk-engine
    path: blink.toml, renders.toml, fixture Scenarios
    note: "Accepted renders in a shared content-addressed store with only hashes in git, every shot drawn in both looks with a control, UI panels driven offscreen from named scenarios"
  - repo: local/game-hacking
    path: Blaze emulator UI (e2e fixtures, shots script, blink.toml)
    note: "Stub hub served from recorded and anonymised fixtures, frozen clock, any console error fails a test"
  - repo: local/Kessa
    path: egui_kittest harness, fixture crate, panels recipe
    note: "UI tests drive the real app, a fixture crate builds a made-up game install, named window states rendered to PNG for review"
---

# Visual Regression Testing

`blink` is a personal render-comparison tool. It is functional and used in several projects, and still immature.

## Philosophy

Rendering work is verified by looking at the output. Numbers (pixel percentages, triangle counts) are proxies. Accept a picture only after seeing it, and keep the accepted pictures out of git history.

## Conventions

- **Accepted pictures live in a shared content-addressed store.** Git holds only pixel hashes and file hashes (`blink.toml`, `renders.toml`). A `verify` run compares fresh renders against them
- **The agent runs `verify` after any change that can touch a render, opens the changed pictures, accepts small explained changes, and escalates only unexplained or large ones.** Never report on a render that was not looked at
- **Draw every shot in both looks with a control.** For a mechanism that is on or off, render both and keep a `differ` control between them, so a change that does nothing cannot pass
- **Drive UI panels offscreen from named scenarios.** A fixture module builds scenarios (a made-up install, a named window state), panels render to PNG, and layout tests flip one thing and assert that no labelled widget moved
- **Make renders deterministic.** A frozen clock, no animation, fixed fonts. Fixtures for a web UI come from a stub hub served from fixtures recorded against the live hub and anonymised, and never edited by hand; never commit a recording the recorder refused
- **A console error fails a test** unless the test declares it expected
- **Build into a scratch directory** when a build publishes into a bind-mounted `dist/` in place, so the mounted directory is never deleted or recreated

## Anti-Patterns

- Reporting on a render without opening it
- Accepting everything at once ("accept all") after a rename or move
- Hand-edited recorded fixtures
- Pixel-percent thresholds as the only check

## Related Topics

- [headless-gpu-rendering](../dx/headless-gpu-rendering.md)
- [native-gpu-ui](../architecture/native-gpu-ui.md)
- [testing-quality](./testing-quality.md)
