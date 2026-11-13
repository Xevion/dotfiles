---
name: deterministic-simulation
category: architecture
last_audited: 2026-11-12
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
- **Enforce determinism with tooling, not only a seeded RNG.** Ban std `HashMap` and `HashSet` (randomly seeded, so iteration order varies per run) and transcendental float methods (use `libm`) through `clippy.toml`. Pin golden hashes of generated terrain in tests, and run a separate workflow that checks those goldens on Windows, macOS and headless Chrome on a weekly cron and before releases, because those runners cost more than Linux
- **Float results that must be identical across machines**: use only per-lane IEEE operations, forbid FMA contraction and reassociated sums, and do not enable relaxed-SIMD, because its FMA rounds once or twice depending on the host. A division by a power of two may become a reciprocal multiply, and nothing else may. See [performance](../patterns/performance.md)
- **Ids hash their text**, not a process-seeded hash, whenever any crate might enable a feature that seeds per process. Record each tick with a digest so replay pins the first diverging tick, and have the journal refuse to replay across a different boundary fingerprint or game build
- **A search with unstable tie-breaks: attack the order, do not protect it.** Expose a `tieBreakSeed` to re-roll tie-breaks, treat a course that passes under only one seed as a coin flip, and do not accept "bit-identical plans" as the bar for a search change. Replay plans as literal per-tick input sequences, and check the physics model against recorded vanilla traces with a conformance oracle
- Earlier form: a manual update loop at fixed frame time (`Duration::from_millis(16)`) rather than Bevy's built-in runner, for WASM worker compatibility. borde.rs now orders its schedule by named system sets and registers messages with bevy

## Anti-Patterns

- Using `thread_rng()` or any non-seeded RNG in simulation logic
- Relying on system time for game state decisions
- Sending bot decisions over the network (introduces non-deterministic ordering)

## Open Questions

- Replay file format and versioning strategy beyond a fingerprinted journal
