@doc: "Changed from" sat directly after a backslash line-continuation in a
@doc: raw string, so it was never a live alternative until the pattern was
@doc: rebuilt with concat!.
@expect: BLOCK history
// Changed from a BTreeMap to a Vec after the key space turned dense
fn f() {}
