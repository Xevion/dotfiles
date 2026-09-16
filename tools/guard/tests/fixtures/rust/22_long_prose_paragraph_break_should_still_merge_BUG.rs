@doc: bug 2 regression - mergeable() breaks a block on ANY blank line. A
@doc: 6-line comment written as two 3-line paragraphs (a deliberate visual
@doc: break, not two unrelated comments) should still read as one 6-line
@doc: prose block and fire long-prose. Today the real blank source line
@doc: between the paragraphs (not a "//" line, an actual empty line)
@doc: fragments it into two 3-line blocks, each under the 4-line
@doc: threshold, so nothing fires.
fn pad1() {}
fn pad2() {}
fn pad3() {}
fn pad4() {}
fn pad5() {}
fn pad6() {}
@expect: NUDGE long-prose
// This explains the overall approach used by this module, covering
// the first major consideration in enough detail to matter, plus a
// second point that still belongs to the same paragraph.

// It then continues into a second paragraph covering the remaining
// considerations, still part of the very same explanation as above,
// just visually separated for readability.
fn f() {}
