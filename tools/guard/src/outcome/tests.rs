use super::*;
use assert2::check;

#[test]
fn allow_is_exit_zero_with_no_rules() {
    let o = Outcome::allow();
    check!(o.exit_code == 0);
    check!(o.rules.is_empty());
    check!(!o.abstained);
}

#[test]
fn abstain_is_exit_zero_but_marked_abstained() {
    let o = Outcome::abstain();
    check!(o.exit_code == 0);
    check!(o.abstained);
}

#[test]
fn block_carries_exit_two_and_given_rules() {
    let o = Outcome::block(vec!["comment-lint.banner".to_string()]);
    check!(o.exit_code == 2);
    check!(o.rules == vec!["comment-lint.banner".to_string()]);
    check!(!o.abstained);
}

#[test]
fn nudge_carries_exit_zero_and_given_rules() {
    let o = Outcome::nudge(vec!["comment-lint.long-prose".to_string()]);
    check!(o.exit_code == 0);
    check!(o.rules == vec!["comment-lint.long-prose".to_string()]);
    check!(!o.abstained);
}

#[test]
fn from_exit_code_preserves_arbitrary_codes() {
    check!(Outcome::from_exit_code(127).exit_code == 127);
}
