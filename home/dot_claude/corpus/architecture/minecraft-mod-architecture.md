---
name: minecraft-mod-architecture
category: architecture
last_audited: 2026-03-26
exemplars: []
---

# Minecraft Mod Architecture

## Philosophy

<!-- Architectury for cross-loader abstraction. All logic in common module, platform modules are pure glue. Kotlin-first with Java reserved for Mixin/Accessor annotations. -->

## Conventions

<!-- Mixin-based game injection (Java only, Mojang mappings), tick-driven state machines for long-running ops, reflection-based optional mod integration (Iris, Sodium), CompletableFuture for off-thread work in tick loop -->

## Anti-Patterns

<!-- Blocking the game thread, platform-specific logic in common module, hardcoded mod detection instead of reflection -->

## Open Questions

<!-- Architectury vs direct multi-loader, Mixin alternatives, mod compatibility testing strategies -->
