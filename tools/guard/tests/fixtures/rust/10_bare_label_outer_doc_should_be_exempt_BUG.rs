@doc: bug 4 regression - bare-label never checks CommentKind::Doc, so a
@doc: terse `/// Config` blocks today though it is a legitimate doc
@doc: comment. Correct behavior mirrors long-prose's existing Doc exemption.
@fixture: clean
/// Config
fn f() {}
