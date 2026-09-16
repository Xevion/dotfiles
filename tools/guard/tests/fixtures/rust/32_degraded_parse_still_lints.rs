@doc: a small localized syntax error keeps the file Degraded, not
@doc: Untrusted - unlike an untrusted parse, degraded parses still run
@doc: every rule. This confirms comment-lint doesn't abstain too eagerly.
@fixture: degraded
@expect: BLOCK bare-label
// Handlers
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn pad() -> i32 { 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 }
fn broken(
