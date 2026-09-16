@doc: bug 5 guardrail - modern Kotlin idioms must parse Clean, not fall
@doc: back to Untrusted and silently disable every rule. This one is
@doc: empirically verified clean via examples/kotlin_probe.rs.
sealed class Result<out T> {
@expect: BLOCK bare-label
    // Handlers
    data class Success<T>(val value: T) : Result<T>()
    data class Failure(val error: Throwable) : Result<Nothing>()
}

fun <T> handle(r: Result<T>): String = when (r) {
    is Result.Success -> "ok: ${r.value}"
    is Result.Failure -> "err: ${r.error.message}"
}
