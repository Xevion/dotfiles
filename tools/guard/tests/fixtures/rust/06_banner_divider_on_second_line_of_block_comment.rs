@doc: exercises the offset-to-absolute-line math for a divider that is not
@doc: the first line of a multi-line block comment. The +1 skips past this
@doc: comment's opening line to the divider's own line.
@expect: BLOCK banner +1
/* first line of the block, ordinary prose
 * ----------------------------------------
 * more prose after the divider */
fn f() {}
