@doc: design-tension guardrail for whatever fix lands for bug 2 (see 22_).
@doc: These two 2-line comments are unrelated topics separated by a blank
@doc: line, not one paragraph-broken explanation. A naive bug-2 fix that
@doc: merges across ANY single blank line would wrongly combine these
@doc: into one 4-line block and start firing long-prose on unrelated
@doc: adjacent comments. Whatever heuristic replaces the current
@doc: "any blank line breaks the block" rule must keep this fixture
@doc: clean - it cannot just relax the line-adjacency check unconditionally.
fn pad1() {}
fn pad2() {}
fn pad3() {}
fn pad4() {}
fn pad5() {}
fn pad6() {}
@fixture: clean
// First topic, line one.
// First topic, line two.

// Second, unrelated topic, line one.
// Second, unrelated topic, line two.
fn f() {}
