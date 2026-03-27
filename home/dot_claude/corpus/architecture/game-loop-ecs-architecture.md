---
name: game-loop-ecs-architecture
category: architecture
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/Pac-Man
    path: pacman/src/systems/
    note: "Bevy ECS without renderer, fixed-tick scheduling, SDL2 graphics, ghost AI state machines"
  - repo: Xevion/borde.rs
    path: crates/borders-core/src/game/
    note: "GameBuilder fluent API, restartable session loop, with_systems(false) for unit tests, rstest parameterized ECS tests"
---

# Game Loop & ECS Architecture

## Philosophy

Entity-Component-System for data-driven game logic. Fixed-tick simulation for determinism. Systems operate on component queries, not object hierarchies. The ECS World is the single source of truth for game state.

## Conventions

- **Bevy ECS standalone**: use `bevy_ecs` without the Bevy renderer. Manual update loop at fixed frame time enables WASM worker compatibility and custom rendering pipelines (Canvas2D, SDL2)
- **GameBuilder fluent API**: compose game instances via a builder pattern accepting map, players, bots, network mode, and frontend transport. The same builder serves production, test fixtures, and replay
- **Restartable session loop**: game sessions are message-driven (StartGame/QuitGame). The loop waits for a start signal, runs simulation, returns to waiting. Enables replay and reconnection without process restart
- **Fixed-tick scheduling**: simulation advances at a fixed timestep (e.g., 16ms/60fps). Rendering is decoupled from simulation rate. Use `Duration::from_millis(16)` in the manual loop, not wall-clock delta time

## Language-Specific

### Rust

- Bevy ECS `World`, `Schedule`, component bundles, `Event` types for inter-system communication
- `GameBuilder::with_systems(false)` disables the Bevy scheduler for unit tests while retaining the real World and resources. Isolates state-transition tests from system ordering
- **Fluent assertion struct**: `game.assert().nation_owns(tile, id).has_territory_changes()` — a `GameAssertExt` trait returning a builder with domain-aware assertion methods. Makes AAA tests readable and resilient to system reorganization
- **rstest parameterized tests**: use `#[case::descriptive_name(value)]` for tick-rate independence, boundary conditions, and regression tests

## Anti-Patterns

- God-object game state (use ECS components for decomposition)
- Mixing rendering and simulation in the same system
- Testing system execution order instead of observable state transitions — too brittle, breaks on scheduler refactors
- Monolithic systems — prefer many small systems with clear query boundaries

## Open Questions

- Bevy ECS without Bevy renderer vs custom ECS for non-game simulations
- ECS performance at scale (>100K entities, component storage strategies)
