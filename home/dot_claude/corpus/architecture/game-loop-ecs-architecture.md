---
name: game-loop-ecs-architecture
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/Pac-Man
    path: pacman/src/systems/
    note: "Bevy ECS without renderer, fixed-tick scheduling, SDL2 graphics, ghost AI state machines"
---

# Game Loop & ECS Architecture

## Philosophy

<!-- Entity-Component-System for data-driven game logic. Fixed-tick simulation for determinism. Systems operate on component queries, not object hierarchies. -->

## Conventions

<!-- Bevy ECS standalone (without renderer), fixed-tick scheduling, system ordering, ECS-driven state machines, component-based entity spawning -->

## Language-Specific

### Rust

<!-- Bevy ECS World, Schedule, SystemStage, component bundles, Event types for inter-system communication -->

## Anti-Patterns

<!-- God-object game state, mixing rendering and simulation, non-deterministic game loops, monolithic systems -->

## Open Questions

<!-- Bevy ECS without Bevy renderer vs custom ECS, deterministic replay for testing, ECS performance at scale -->
