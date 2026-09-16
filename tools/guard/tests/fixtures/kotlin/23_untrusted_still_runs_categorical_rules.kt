@doc: the payoff of splitting the trust gate. This file has the catch-clause
@doc: trailing comma that tree-sitter-kotlin-ng cannot parse (see 20_), so the
@doc: parse is untrusted and the structural nudge tier is skipped - but the
@doc: banner divider is still caught, where previously the whole file was
@doc: silently exempt from every rule.
@fixture: untrusted
@expect: BLOCK banner
// ==========================
fun f() {
    try {
        g()
    } catch (
        error: Throwable,
    ) {
        h(error)
    }
}
