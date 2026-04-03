---
name: minecraft-mod-architecture
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/glint
    path: mod/common/src/ + mod/fabric/ + mod/neoforge/
    note: "Architectury multi-loader, reflection-based Iris/Sodium integration, device code auth, framebuffer capture"
  - repo: local/maestro
    path: common/src/ + codegen/
    note: "Tick-driven pathfinding state machines, KSP config codegen, in-game integration tests, programmatic Log4j2"
---

# Minecraft Mod Architecture

## Philosophy

Architectury for cross-loader abstraction. All game logic in the common module; platform modules are pure glue (10-20 lines each). Kotlin-first with Java reserved exclusively for Mixin/Accessor annotations. Development velocity over backwards compatibility — APIs evolve freely in alpha.

## Conventions

- **Architectury common/platform split**: all business logic lives in `mod/common/`. Fabric and NeoForge modules contain only entrypoints (`ClientModInitializer` + tick registration for Fabric, `@Mod` + event bus for NeoForge). Platform glue should be under 20 lines per loader

- **Java Mixins with Mojang mappings**: Mixins are Java-only (required by the Mixin library), placed under `mod/common/src/main/java/.../mixin/`. Use Mojang mappings for deobfuscation. `@Unique` fields use a project-prefixed name (e.g., `glint$fieldName`) to avoid conflicts with other mods. Verify method signatures against the mapped jar (`just mcjar` or equivalent)

- **Reflection-based optional mod integration**: detect optional mods (Iris, Sodium) via `runCatching { Class.forName("...") }` with validated method signatures. Log at debug level for expected missing classes, error level for API signature changes (indicates an incompatible mod update). Never hardcode mod presence — the integration must degrade gracefully

- **Tick-driven state machines for async work**: when the game thread cannot suspend, use a `State` enum with typed `CompletableFuture` or `Deferred` polling in `tick()`. Check `future.isDone` / `isCompleted` and advance state only when the off-thread work completes. See [kotlin-jvm](../languages/kotlin-jvm.md) for the full sealed PendingOp pattern

- **Off-thread pathfinding with coroutine scope**: dedicate a single-threaded `CoroutineScope` + `SupervisorJob` (named daemon thread, e.g., `"Maestro-Pathfinding"`) for CPU-intensive search. Expose live progress via `@Volatile` fields for cross-thread reads without locking. Use `CompletableDeferred<PathResult>` and poll `getCompleted()` on the game thread — never block the tick

- **KSP annotation processing for config codegen**: use KSP + KotlinPoet in a dedicated `codegen/` Gradle subproject to generate typed snapshot data classes, `snapshot()` extension functions, and TOML-compatible `@TomlComments` annotations from `@PropertyDoc`. Eliminates handwritten serialization glue. See [build-time-codegen](../dx/build-time-codegen.md) for the Kotlin section

- **Programmatic Log4j2 configuration**: when the host framework initializes logging before your code (Minecraft, plugin systems), construct appenders programmatically at class-init time. Support dual-format (pretty console + JSONL file) plus domain-specific outputs (in-game chat appender, event bus bridge for HUD/debug overlays). See [kotlin-jvm](../languages/kotlin-jvm.md) for details

- **In-game integration test framework**: drive test scenarios against a live Minecraft client via Fabric/NeoForge gametest hooks. Support configurable tick-rate multipliers (1x/5x/10x) for CI speed, PositionRecorder for trajectory capture, JUnit XML output for CI consumption, and filter DSLs (wildcards + exclusions) for scenario selection

## Anti-Patterns

- **Blocking the game thread**: never call `.get()` / `.await()` on futures from the tick thread. Poll `isDone` and advance state machines incrementally
- **Platform-specific logic in common module**: loader-specific code (Fabric events, NeoForge event bus) must stay in platform modules. Common code uses Architectury abstractions
- **Hardcoded mod detection**: checking for specific class names without `runCatching` fails silently or crashes when the optional mod is absent. Always use reflection with graceful fallback
- **Direct Log4j2 XML/JSON config**: in embedded environments where the host controls logging, XML config files are overridden by the host. Use programmatic appender construction instead

## Open Questions

- Architectury vs direct multi-loader: whether the abstraction overhead justifies the convenience for mods targeting only Fabric + NeoForge (no Forge)
- Mixin alternatives: whether Access Widener / Access Transformer cover enough use cases to reduce Mixin count
- Mod compatibility testing automation: CI testing against multiple mod combinations without a full Minecraft client
