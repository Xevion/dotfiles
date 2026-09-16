@doc: A trailing comma after a catch-clause parameter is valid, ktfmt-idiomatic
@doc: Kotlin, but tree-sitter-kotlin-ng cannot parse it. Removing only the comma
@doc: makes this file Clean (see the contrast fixture 22). A real grammar gap
@doc: rather than a mid-edit artifact, and it silently disables every
@doc: comment-lint rule for the whole file.
@fixture: untrusted
fun f() {
    try {
        g()
    } catch (
        error: Throwable,
    ) {
        h(error)
    }
}
