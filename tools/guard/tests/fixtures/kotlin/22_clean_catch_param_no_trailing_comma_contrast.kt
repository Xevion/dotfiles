@doc: contrastive pair for 20_ - the exact same catch clause without the
@doc: trailing comma parses Clean. Pins down that the comma alone is the
@doc: trigger, so a future tree-sitter-kotlin-ng upgrade that fixes this
@doc: will make 20_ start failing (in the good direction) without this
@doc: fixture also silently drifting.
@expect: BLOCK bare-label
// Handlers
fun f() {
    try {
        g()
    } catch (error: Throwable) {
        h(error)
    }
}
