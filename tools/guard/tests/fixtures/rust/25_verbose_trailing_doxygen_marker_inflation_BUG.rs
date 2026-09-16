@doc: bug 3 regression - strip_markers only strips the plain "///" prefix
@doc: and leaves the trailing "<" from a Doxygen trailing-doc marker
@doc: ("///<"). That stray "<" counts as an extra word, pushing an exactly
@doc: 10-word comment to 11 tokens and firing when it should stay clean.
@fixture: clean
let x = 1; ///< one two three four five six seven eight nine ten
