@doc: the old pattern anchored the keyword immediately after the comment
@doc: marker, so history phrased mid-sentence slipped through entirely.
@expect: BLOCK history
// The tokenizer is generated now; we used to hand-roll it by character
fn f() {}
