@doc: sanity guardrail alongside the bug-1 regressions above - once
@doc: history_pattern stops over-firing on ordinary present tense, it must
@doc: still catch unambiguous history like this one.
@expect: BLOCK history
// Previously this used a linked list; a Vec is simpler and faster here
fn f() {}
