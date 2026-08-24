//! `PostToolUse` handling for `Write`/`Edit`. Content rules (comment-lint,
//! etc.) land here in a later iteration; for now every call is a no-op.

use crate::payload::Payload;

/// Stub entry point for post-edit rules. Always allows.
pub fn main(_payload: &Payload) -> i32 {
    0
}

#[cfg(test)]
mod tests;
