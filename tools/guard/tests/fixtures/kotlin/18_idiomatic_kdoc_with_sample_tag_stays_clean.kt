@doc: bug 5 guardrail - KDoc with @param/@return/@sample tags and a
@doc: fenced code block must not confuse the parser or the comment
@doc: extractor. Empirically verified clean via examples/kotlin_probe.rs.
@fixture: clean
/**
 * Formats a value.
 *
 * Example:
 * ```
 * format(1) // "1"
 * ```
 *
 * @sample com.example.Samples.formatSample
 */
fun format(x: Int): String = x.toString()
