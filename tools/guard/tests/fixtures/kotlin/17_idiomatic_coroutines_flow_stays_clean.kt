@doc: bug 5 guardrail - suspend/Flow syntax must not push the parse into
@doc: Untrusted. Empirically verified clean via examples/kotlin_probe.rs.
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow

@expect: BLOCK bare-label
// Handlers
suspend fun fetchAll(): Flow<Int> = flow {
    for (i in 1..10) {
        emit(i)
        kotlinx.coroutines.delay(10)
    }
}
