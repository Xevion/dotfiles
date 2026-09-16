@doc: a single atomic block comment spanning 4+ lines is its own
@doc: CommentBlock (never merges with anything else) and must still be
@doc: subject to long-prose on its own span.
fn pad1() {}
fn pad2() {}
fn pad3() {}
fn pad4() {}
fn pad5() {}
fn pad6() {}
@expect: NUDGE long-prose
/* line one
 * line two
 * line three
 * line four */
fn f() {}
