@doc: bug 1 regression - history_pattern blocks on the present-tense verb
@doc: "Removed" even when the sentence describes current behavior, not a
@doc: past refactor. comment/tests.rs currently asserts the opposite
@doc: (enshrines this as correct); this fixture asserts the fix.
@fixture: clean
// Removed entries are tombstoned here, not deleted
fn f() {}
