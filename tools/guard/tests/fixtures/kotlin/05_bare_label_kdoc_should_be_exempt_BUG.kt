@doc: bug 4 regression, Kotlin variant - a terse KDoc comment reading only
@doc: as a bare label should be exempt the same way a Rust `///` is meant
@doc: to be, but bare-label checks no CommentKind at all today.
@fixture: clean
/** Config */
fun f() {}
