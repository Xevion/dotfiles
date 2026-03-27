---
name: deterministic-simulation
category: architecture
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/borde.rs
    path: crates/borders-core/src/game/
    note: "Seeded RNG with per-turn per-context prime-multiplier seeds, deterministic client-side bot execution"
---

# Deterministic Simulation

## Philosophy

Reproducibility across clients. Game simulation must produce identical results given the same inputs, regardless of execution environment (native, WASM, different machines). Randomness is seeded and context-keyed, never truly random.

## Conventions

- **Seeded RNG with context-keyed streams**: derive per-turn, per-context seeds from a base seed using deterministic transformations (e.g., prime multipliers). Each RNG consumer (nation AI, tile generation, combat resolution) gets its own stream derived from the shared seed, preventing cross-system interference
- **Deterministic client-side execution**: bot actions are calculated locally on each client rather than sent over the network. This prevents non-deterministic divergence from network timing or ordering. Only human player intents cross the network boundary
- **Restartable session loop**: game sessions are message-driven (StartGame/QuitGame). The game loop waits for a start signal, runs the simulation, and returns to waiting state. Enables replay, reconnection, and spectating without process restart
- **GameBuilder fluent API**: compose game instances via a builder pattern that accepts map, players, bots, network mode, and frontend transport. The same builder serves both production game construction and test fixture setup

## Language-Specific

### Rust

- Use a `DeterministicRng` resource in Bevy ECS that derives per-context seeds
- Manual update loop at fixed frame time (`Duration::from_millis(16)`) rather than Bevy's built-in runner, enabling WASM worker compatibility

## Anti-Patterns

- Using `thread_rng()` or any non-seeded RNG in simulation logic
- Relying on system time for game state decisions
- Sending bot decisions over the network (introduces non-deterministic ordering)

## Open Questions

- Deterministic floating-point across architectures (IEEE 754 strictness)
- Replay file format and versioning strategy
