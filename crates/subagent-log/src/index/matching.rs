//! Sessions are filed by the cwd at start, but wander afterward, so this
//! matches on recorded `cwd` values instead. `~`/`~/projects` show up on
//! nearly every session, so they only count as a match when exact or when
//! the current dir is shallower than a more specific recorded one.

use std::path::{Path, PathBuf};

use super::Run;

impl Run {
    #[must_use]
    pub fn session_matches(recorded: &[PathBuf], cwd: &Path, generic_roots: &[PathBuf]) -> bool {
        recorded.iter().any(|r| {
            r == cwd || r.starts_with(cwd) || (cwd.starts_with(r) && !generic_roots.contains(r))
        })
    }

    #[must_use]
    pub fn generic_roots(home: &Path) -> Vec<PathBuf> {
        vec![home.to_path_buf(), home.join("projects")]
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use assert2::check;

    fn p(s: &str) -> PathBuf {
        PathBuf::from(s)
    }

    #[test]
    fn exact_match() {
        let recorded = vec![p("/home/xevion/projects/foo")];
        check!(Run::session_matches(
            &recorded,
            &p("/home/xevion/projects/foo"),
            &[]
        ));
    }

    #[test]
    fn session_worked_deeper_than_current_cwd() {
        // sitting at the project root, session went into a subdirectory
        let recorded = vec![p("/home/xevion/projects/foo/src")];
        check!(Run::session_matches(
            &recorded,
            &p("/home/xevion/projects/foo"),
            &[]
        ));
    }

    #[test]
    fn current_cwd_deeper_than_recorded_specific_root() {
        // the wt-explorer case: session recorded the project root, we're
        // now sitting somewhere inside it
        let recorded = vec![p("/home/xevion/projects/wt-explorer")];
        check!(Run::session_matches(
            &recorded,
            &p("/home/xevion/projects/wt-explorer/crates/dagor/src"),
            &[p("/home/xevion")]
        ));
    }

    #[test]
    fn generic_root_alone_does_not_match_unrelated_deep_cwd() {
        // every routed session touches ~ at some point, so that alone must
        // never match a specific, unrelated project directory
        let recorded = vec![p("/home/xevion")];
        check!(!Run::session_matches(
            &recorded,
            &p("/home/xevion/projects/some-other-project"),
            &[p("/home/xevion")]
        ));
    }

    #[test]
    fn unrelated_paths_do_not_match() {
        let recorded = vec![p("/home/xevion/projects/foo")];
        check!(!Run::session_matches(
            &recorded,
            &p("/home/xevion/projects/bar"),
            &[]
        ));
    }

    #[test]
    fn generic_root_still_matches_when_current_cwd_is_shallow() {
        // sitting at ~ with no more specific place to narrow to, so showing
        // everything is the correct degenerate behavior here
        let recorded = vec![p("/home/xevion/projects/foo")];
        check!(Run::session_matches(
            &recorded,
            &p("/home/xevion"),
            &[p("/home/xevion")]
        ));
    }
}
