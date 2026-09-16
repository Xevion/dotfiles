@doc: contrasts with 20_'s real grammar gap. This shape (an unbalanced
@doc: brace, no other damage) also goes Untrusted, but it is NOT
@doc: Kotlin-specific: examples/kotlin_probe.rs shows an equivalently tiny
@doc: unbalanced Rust snippet (rust_tiny_mid_edit_unbalanced) does too.
@doc: Kept as its own fixture because a PostToolUse hook fires after every
@doc: Write/Edit, so a multi-step edit can genuinely be captured mid-brace
@doc: - this is an expected, correct abstention, not a bug, and is worth
@doc: pinning down separately from 20_'s real grammar defect.
@fixture: untrusted
class Foo {
    fun bar() {
        if (true) {
            doThing()
