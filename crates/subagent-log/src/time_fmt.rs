//! Timestamp display: relative and absolute shown together, since either one
//! alone is either too vague ("3h ago") or too disconnected from "now" to
//! place quickly ("2026-09-18 14:32").

use std::time::{Duration, SystemTime};

use time::OffsetDateTime;
use time::macros::format_description;

pub trait TimestampExt {
    fn relative(&self, now: SystemTime) -> String;
    fn absolute(&self) -> String;
    fn display(&self, now: SystemTime) -> String;
}

impl TimestampExt for SystemTime {
    fn relative(&self, now: SystemTime) -> String {
        let elapsed = now
            .duration_since(*self)
            .unwrap_or(Duration::ZERO)
            .as_secs();
        match elapsed {
            0..=4 => "just now".to_string(),
            5..=59 => format!("{elapsed}s ago"),
            60..=3599 => format!("{}m ago", elapsed / 60),
            3600..=86399 => format!("{}h ago", elapsed / 3600),
            86400..=604_799 => format!("{}d ago", elapsed / 86400),
            _ => format!("{}w ago", elapsed / 604_800),
        }
    }

    /// "Sep 18, 2:32 PM", local time, year appended only if not current.
    fn absolute(&self) -> String {
        let utc = OffsetDateTime::from(*self);
        let now_local = OffsetDateTime::now_local().unwrap_or(utc);
        let local = utc.to_offset(now_local.offset());

        if local.year() == now_local.year() {
            let format = format_description!(
                "[month repr:short] [day padding:none], [hour repr:12]:[minute] [period]"
            );
            local.format(&format).unwrap_or_else(|_| local.to_string())
        } else {
            let format = format_description!(
                "[month repr:short] [day padding:none] [year], [hour repr:12]:[minute] [period]"
            );
            local.format(&format).unwrap_or_else(|_| local.to_string())
        }
    }

    fn display(&self, now: SystemTime) -> String {
        format!("{} ({})", self.relative(now), self.absolute())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;
    use rstest::rstest;

    #[rstest]
    #[case(0, "just now")]
    #[case(4, "just now")]
    #[case(5, "5s ago")]
    #[case(59, "59s ago")]
    #[case(60, "1m ago")]
    #[case(3599, "59m ago")]
    #[case(3600, "1h ago")]
    #[case(86399, "23h ago")]
    #[case(86400, "1d ago")]
    #[case(604_799, "6d ago")]
    #[case(604_800, "1w ago")]
    fn relative_formats_expected_buckets(#[case] elapsed_secs: u64, #[case] expected: &str) {
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(1_000_000);
        let when = now - Duration::from_secs(elapsed_secs);
        check!(when.relative(now) == expected);
    }

    #[test]
    fn relative_clamps_future_timestamps_to_now() {
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(1_000_000);
        let when = now + Duration::from_mins(1);
        check!(when.relative(now) == "just now");
    }

    #[test]
    fn display_combines_relative_and_absolute() {
        let now = SystemTime::UNIX_EPOCH + Duration::from_secs(1_000_000);
        let when = now - Duration::from_mins(1);
        let combined = when.display(now);
        check!(combined.starts_with("1m ago ("));
        check!(combined.ends_with(')'));
    }
}
