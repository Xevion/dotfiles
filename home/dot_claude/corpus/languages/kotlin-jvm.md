---
name: kotlin-jvm
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: local/maestro
    path: common/src/
    note: "@JvmInline value classes for domain types, ReadWriteProperty config delegation, Kotest DescribeSpec with property-based pathfinding tests"
  - repo: Xevion/glint
    path: mod/common/src/
    note: "Tick-driven state machine with CompletableFuture polling, sealed ApiError with companion factory"
---

# Kotlin / JVM

## Philosophy

Kotlin-first, sealed classes for exhaustive state, extension functions for ergonomics, null safety as a core discipline, coroutines for structured concurrency. All business logic lives in Kotlin; Java is reserved for framework-required annotations (`@Mixin`, `@Accessor`, etc.). Cross-boundary naming conventions (e.g., singleton access via `INSTANCE.getX()`) should be documented explicitly at the boundary.

## Conventions

### @JvmInline value class for Java interop wrappers

When adding Kotlin extension overloads to a Java library type, name collisions with the Java member methods cause silent overload resolution conflicts. Wrapping the Java type in a `@JvmInline` value class routes all calls through Kotlin dispatch with zero runtime overhead.

```kotlin
@JvmInline
value class KLogger(val logger: org.slf4j.Logger) {
    fun debug(msg: () -> String) {
        if (logger.isDebugEnabled) logger.debug(msg())
    }
}
fun org.slf4j.Logger.asKLogger() = KLogger(this)
```

### @JvmInline for bit-packing domain types (situational)

Value classes can double as bit-packing containers, storing multiple fields in one primitive `Long` while preserving type safety. Useful for map keys in hot paths where boxing must be eliminated. The technique generalizes beyond JVM — any language with value types or newtypes can apply the same pattern for composite keys.

```kotlin
@JvmInline
value class ChunkKey(val packed: Long) {
    constructor(x: Int, z: Int) : this(x.toLong() shl 32 or (z.toLong() and 0xFFFFFFFFL))
    val x: Int get() = (packed shr 32).toInt()
    val z: Int get() = packed.toInt()
}
```

### ReadWriteProperty delegation for config

Encapsulate access-control invariants — thread safety via `@Volatile`, write guards, validation — inside a property's getter/setter via `ReadWriteProperty`. This keeps invariants co-located with the property definition rather than scattered at every call site.

```kotlin
fun <T> guardedWrite(initial: T, check: () -> Boolean): ReadWriteProperty<Any?, T> =
    object : ReadWriteProperty<Any?, T> {
        @Volatile private var value = initial
        override fun getValue(thisRef: Any?, property: KProperty<*>) = value
        override fun setValue(thisRef: Any?, property: KProperty<*>, value: T) {
            check() || error("Write rejected: invariant violated for ${property.name}")
            this.value = value
        }
    }

var config: String by guardedWrite("default") { isInitPhase }
```

### FailureContext with diagnostic snapshots

Error sealed classes should carry domain-specific diagnostic context rather than just a message string. A `reasonDisplay` computed property on the sealed interface unifies human-readable rendering and keeps display logic out of call sites.

```kotlin
sealed interface SearchFailure {
    val reasonDisplay: String

    data class NoPathFound(val stepsExplored: Int, val lastPosition: Vec2) : SearchFailure {
        override val reasonDisplay get() = "No path after $stepsExplored steps (last: $lastPosition)"
    }
    data class Timeout(val elapsedMs: Long) : SearchFailure {
        override val reasonDisplay get() = "Search timed out after ${elapsedMs}ms"
    }
}
```

### Kotest DescribeSpec + property-based testing

`DescribeSpec` provides nested `describe`/`it` grouping suited to algorithm tests with many scenarios. `checkAll`/`Arb` adds property-based coverage. Both `DescribeSpec` and JUnit `@Test` coexist in the same Gradle test suite without configuration changes.

```kotlin
class PathfinderSpec : DescribeSpec({
    describe("A* pathfinder") {
        it("finds direct path on open grid") { /* ... */ }

        it("handles arbitrary start/end pairs") {
            checkAll(Arb.int(0..15), Arb.int(0..15)) { sx, sz ->
                val result = findPath(Vec2(sx, 0), Vec2(sz, 0), emptyGrid)
                result.shouldBeInstanceOf<PathResult.Found>()
            }
        }
    }
})
```

### Programmatic Log4j2 appenders (JVM embedded apps)

When a framework initializes logging before your code runs (e.g., Minecraft mods, plugin systems), use programmatic appender construction for dual-format output (pretty console + JSONL file). This bypasses the framework's logging config without fighting it.

### Tick-driven state machine for game-loop environments

When the main loop cannot suspend (game ticks, UI event loops), use a `State` enum with a sealed `PendingOp` class to track async work. Each `PendingOp` variant types its `CompletableFuture` result, eliminating unsafe casts when advancing state. `tick()` checks `future.isDone` and advances only when the off-thread work completes.

```kotlin
private enum class State { FetchingWork, PreparingAssets, Capturing, Done }
private sealed class PendingOp {
    abstract val future: CompletableFuture<*>
    class FetchWork(override val future: CompletableFuture<Result<List<WorkItem>>>) : PendingOp()
    class PrepareAssets(override val future: CompletableFuture<PrepResult>) : PendingOp()
}
fun tick() {
    val op = pending ?: return
    if (!op.future.isDone) return
    // advance state based on result
}
```

### Sealed error class with companion factory

Sealed error classes should carry domain-specific state (retry timing, cause type) and expose a `userMessage` computed property. A companion `fromException()` factory centralizes boundary wrapping — call sites don't scatter `when` chains for exception-to-error conversion.

```kotlin
sealed class ApiError : Exception() {
    abstract val userMessage: String
    data class RateLimited(val retryAfterSeconds: Long) : ApiError() {
        override val userMessage = "Too many requests. Retrying in ${retryAfterSeconds}s..."
    }
    companion object {
        fun fromException(e: Exception): ApiError = when (e) {
            is ConnectException -> NetworkError(e.message ?: "Connection failed")
            else -> UnknownError(e.message ?: "Unknown error")
        }
    }
}
```

## Anti-Patterns

- **Java-style builders when data classes suffice** — use named parameters and `copy()` instead.
- **Blocking in coroutine scope** — always dispatch blocking I/O or CPU work with `Dispatchers.IO` or `withContext`; blocking the default dispatcher starves other coroutines.
- **Platform type leaks** — narrow Java interop types (`!` types) at the boundary with explicit null checks or `!!`; never let `String!` propagate into Kotlin-only code.
- **Exceptions for expected control flow** — return a sealed `Result`/`Either` type for expected failure states; reserve exceptions for truly unexpected conditions.

## Open Questions

- K2 compiler migration timing: when the toolchain and plugin ecosystem are stable enough to warrant migration, and which incremental benefits (faster compilation, improved type inference) justify the risk.
- Compose Multiplatform viability for non-Android targets: desktop and web stability, performance on JVM desktop vs native, and ecosystem readiness for production use.
- KSP vs KAPT migration path: which annotation processors have KSP support, how to handle those that don't, and the timeline for fully dropping KAPT to improve build times.
