@doc: sanity guardrail alongside 25_ - once strip_markers correctly
@doc: consumes the whole "///<" marker, a genuinely 11-word explanation
@doc: after it must still fire.
@expect: NUDGE verbose-trailing
let x = 1; ///< one two three four five six seven eight nine ten eleven
