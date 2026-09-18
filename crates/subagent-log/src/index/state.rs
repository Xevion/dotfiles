//! Finished/unfinished classification. No live task registry exists on disk
//! to confirm this, so it's a heuristic: whether the last turn ended with
//! text, and how long ago the file was touched.

use std::time::{Duration, SystemTime};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RunState {
    /// Last-observed turn closed with text, and stayed closed for at least
    /// `SETTLE_THRESHOLD`: a real, if not necessarily successful, report
    /// exists and nothing new has landed since.
    Done,
    /// Touched very recently, either mid tool-call or a just-closed turn
    /// still inside its settle window: probably still running.
    Active,
    /// Trailing tool call, touched a while ago. Ambiguous: could be a slow
    /// single tool call, or could be quietly dead.
    Stalled,
    /// Trailing tool call, untouched for a long time: very likely dead.
    Interrupted,
}

pub const ACTIVE_THRESHOLD: Duration = Duration::from_secs(90);
pub const STALLED_THRESHOLD: Duration = Duration::from_mins(5);

/// 94% of closed-with-text turns get more activity within 15s (surveyed
/// against real transcripts), so `Done` needs to sit quiet this long first.
pub const SETTLE_THRESHOLD: Duration = Duration::from_secs(15);

impl RunState {
    #[must_use]
    pub fn classify(last_turn_closed: bool, mtime: SystemTime, now: SystemTime) -> Self {
        let elapsed = now.duration_since(mtime).unwrap_or(Duration::ZERO);
        if last_turn_closed {
            return if elapsed < SETTLE_THRESHOLD {
                Self::Active
            } else {
                Self::Done
            };
        }
        if elapsed < ACTIVE_THRESHOLD {
            Self::Active
        } else if elapsed < STALLED_THRESHOLD {
            Self::Stalled
        } else {
            Self::Interrupted
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;
    use rstest::rstest;

    fn at(secs: u64) -> SystemTime {
        SystemTime::UNIX_EPOCH + Duration::from_secs(secs)
    }

    #[rstest]
    #[case(0, RunState::Active)]
    #[case(14, RunState::Active)]
    #[case(15, RunState::Done)]
    #[case(1_000_000, RunState::Done)]
    fn closed_turn_settles_into_done(#[case] elapsed_secs: u64, #[case] expected: RunState) {
        let now = at(1_000_000);
        let mtime = now - Duration::from_secs(elapsed_secs);
        check!(RunState::classify(true, mtime, now) == expected);
    }

    #[rstest]
    #[case(0, RunState::Active)]
    #[case(89, RunState::Active)]
    #[case(90, RunState::Stalled)]
    #[case(299, RunState::Stalled)]
    #[case(300, RunState::Interrupted)]
    #[case(10_000, RunState::Interrupted)]
    fn open_turn_buckets_by_age(#[case] elapsed_secs: u64, #[case] expected: RunState) {
        let now = at(1_000_000);
        let mtime = now - Duration::from_secs(elapsed_secs);
        check!(RunState::classify(false, mtime, now) == expected);
    }
}
