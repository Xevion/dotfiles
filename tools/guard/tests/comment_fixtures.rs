//! Fixture-driven tests for the comment-lint subsystem.
//!
//! A fixture is a real source file under `tests/fixtures/<language>/`, driven
//! through the production [`guard::lang::analyze`] and
//! [`guard::comment::evaluate_trusted`] pipeline. Expectations live inline as
//! directive lines, blanked rather than deleted before parsing so line numbers
//! never shift.
//!
//! ```text
//! @fixture: clean                 assert zero findings anywhere
//! @fixture: untrusted             assert evaluate_trusted() abstains
//! @fixture: degraded              assert ParseQuality::Degraded
//! @expect: BLOCK <category-id>    attaches to the next non-directive line
//! @expect: NUDGE <category-id>    optional trailing `+N` skips N more lines
//! @doc: free text                 authoring note, never becomes source
//! ```
//!
//! Directives use one syntax across every language because they are removed
//! before tree-sitter sees them, so they never need to be valid in the target
//! language.
//!
//! A fixture declaring neither a file-level mode nor any `@expect:` line is an
//! authoring error, not a silent pass. That distinction is what stops a
//! negative fixture that is empty *by accident* from reading as one that is
//! empty *by intent*.
//!
//! Files under `tests/fixtures/` are data, not test targets: Cargo only builds
//! top-level `tests/*.rs`, so deliberately unparseable fixtures never reach the
//! compiler.

use assert2::assert;
use guard::comment::{self, Category};
use guard::lang::{self, Analysis, ParseQuality};
use std::path::Path;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Tier {
    Block,
    Nudge,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct Expectation {
    line: usize,
    category: String,
    tier: Tier,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum FixtureMode {
    Normal,
    Clean,
    Untrusted,
    Degraded,
}

struct ParsedFixture {
    stripped_source: String,
    mode: FixtureMode,
    expectations: Vec<Expectation>,
}

/// Split a fixture's raw text into blanked source plus declared expectations.
/// Errors are authoring mistakes: bad directive syntax, an unknown category,
/// `clean` combined with `@expect:` lines, or no expectations at all.
fn parse_fixture(raw: &str) -> Result<ParsedFixture, String> {
    let raw_lines: Vec<&str> = raw.split('\n').collect();
    let mut out_lines: Vec<String> = raw_lines.iter().map(|l| l.to_string()).collect();
    let mut mode = FixtureMode::Normal;
    let mut expectations = Vec::new();
    let mut pending: Vec<(String, Tier, usize)> = Vec::new();

    for (i, line) in raw_lines.iter().enumerate() {
        let trimmed = line.trim_start();

        if trimmed.starts_with("@doc:") {
            out_lines[i] = String::new();
            continue;
        }

        if let Some(rest) = trimmed.strip_prefix("@fixture:") {
            let value = rest.trim();
            mode = match value {
                "clean" => FixtureMode::Clean,
                "untrusted" => FixtureMode::Untrusted,
                "degraded" => FixtureMode::Degraded,
                other => return Err(format!("line {}: unknown @fixture: mode '{other}'", i + 1)),
            };
            out_lines[i] = String::new();
            continue;
        }

        if let Some(rest) = trimmed.strip_prefix("@expect:") {
            let mut parts = rest.split_whitespace();
            let tier = match parts.next() {
                Some("BLOCK") => Tier::Block,
                Some("NUDGE") => Tier::Nudge,
                other => {
                    return Err(format!(
                        "line {}: @expect: tier must be BLOCK or NUDGE, got {other:?}",
                        i + 1
                    ));
                }
            };
            let category = parts
                .next()
                .ok_or_else(|| format!("line {}: @expect: missing category id", i + 1))?
                .to_string();
            if category_from_id(&category).is_none() {
                return Err(format!("line {}: unknown category id '{category}'", i + 1));
            }
            // `+N` targets a line deeper inside a multi-line comment than the
            // directive itself sits.
            let extra_skip = match parts.next() {
                None => 0,
                Some(tok) => tok
                    .strip_prefix('+')
                    .and_then(|n| n.parse::<usize>().ok())
                    .ok_or_else(|| {
                        format!("line {}: expected a `+N` line offset, got '{tok}'", i + 1)
                    })?,
            };
            pending.push((category, tier, extra_skip));
            out_lines[i] = String::new();
            continue;
        }

        for (category, tier, extra_skip) in pending.drain(..) {
            expectations.push(Expectation {
                line: i + extra_skip,
                category,
                tier,
            });
        }
    }

    if !pending.is_empty() {
        return Err("trailing @expect: directive(s) with no following line to attach to".into());
    }
    if mode == FixtureMode::Clean && !expectations.is_empty() {
        return Err("`@fixture: clean` cannot be combined with `@expect:` lines".into());
    }
    if mode == FixtureMode::Normal && expectations.is_empty() {
        return Err(
            "fixture declares no expectations - add `@fixture: clean` for a negative \
                    fixture, or at least one `@expect:` line"
                .into(),
        );
    }

    Ok(ParsedFixture {
        stripped_source: out_lines.join("\n"),
        mode,
        expectations,
    })
}

fn category_from_id(id: &str) -> Option<Category> {
    [
        Category::Banner,
        Category::BareLabel,
        Category::History,
        Category::LongProse,
        Category::VerboseTrailing,
    ]
    .into_iter()
    .find(|c| c.id() == id)
}

/// Run one fixture and describe the failure, or return `None` if it passed.
fn run_fixture(path: &Path, fixture: &ParsedFixture) -> Option<String> {
    let Some(analysis) = lang::analyze(path, &fixture.stripped_source) else {
        return Some(format!(
            "{}: guard::lang::analyze() returned None - extension not recognized",
            path.display()
        ));
    };
    check_fixture(path, fixture, analysis)
}

fn check_fixture(path: &Path, fixture: &ParsedFixture, analysis: Analysis) -> Option<String> {
    match fixture.mode {
        FixtureMode::Untrusted => {
            if analysis.quality != ParseQuality::Untrusted {
                return Some(format!(
                    "{}: expected ParseQuality::Untrusted, got {:?}",
                    path.display(),
                    analysis.quality
                ));
            }
            // Categorical rules still run on an untrusted parse; only the
            // structural nudge tier is dropped.
            let report = comment::evaluate_trusted(&analysis);
            if !report.nudges.is_empty() {
                return Some(format!(
                    "{}: nudges must not run on an untrusted parse, got {} of them",
                    path.display(),
                    report.nudges.len()
                ));
            }
            diff_report(path, fixture, &analysis)
        }
        FixtureMode::Degraded => {
            if analysis.quality != ParseQuality::Degraded {
                return Some(format!(
                    "{}: expected ParseQuality::Degraded, got {:?}",
                    path.display(),
                    analysis.quality
                ));
            }
            diff_report(path, fixture, &analysis)
        }
        FixtureMode::Clean | FixtureMode::Normal => {
            // An unmarked abstention is a silent loss of every rule in
            // production, so it fails loudly here rather than reading as clean.
            if analysis.quality == ParseQuality::Untrusted {
                return Some(format!(
                    "{}: parse unexpectedly abstained. If intentional, mark the fixture \
                     `@fixture: untrusted`.",
                    path.display()
                ));
            }
            diff_report(path, fixture, &analysis)
        }
    }
}

fn diff_report(path: &Path, fixture: &ParsedFixture, analysis: &Analysis) -> Option<String> {
    let report = comment::evaluate_trusted(analysis);

    let mut actual: Vec<Expectation> = report
        .categorical
        .iter()
        .map(|f| Expectation {
            line: f.line,
            category: f.category.id().to_string(),
            tier: Tier::Block,
        })
        .chain(report.nudges.iter().map(|f| Expectation {
            line: f.line,
            category: f.category.id().to_string(),
            tier: Tier::Nudge,
        }))
        .collect();
    actual.sort_by_key(|e| (e.line, e.category.clone()));

    let mut expected = fixture.expectations.clone();
    expected.sort_by_key(|e| (e.line, e.category.clone()));

    if actual == expected {
        return None;
    }

    let missing: Vec<&Expectation> = expected.iter().filter(|e| !actual.contains(e)).collect();
    let extra: Vec<&Expectation> = actual.iter().filter(|e| !expected.contains(e)).collect();

    let mut msg = format!(
        "{}: expectations did not match actual report\n",
        path.display()
    );
    for (label, set) in [
        ("MISSING (expected, not found)", &missing),
        ("EXTRA (found, not expected)", &extra),
    ] {
        if set.is_empty() {
            continue;
        }
        msg.push_str(&format!("  {label}:\n"));
        for e in set.iter() {
            msg.push_str(&format!(
                "    line {} [{:?}] {}\n",
                e.line + 1,
                e.tier,
                e.category
            ));
        }
    }
    Some(msg)
}

/// Collect every failure in a language directory rather than stopping at the
/// first, so one run reports the whole picture.
fn run_language_dir(dir: &str) -> Vec<String> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/fixtures")
        .join(dir);
    let mut entries: Vec<_> = std::fs::read_dir(&root)
        .unwrap_or_else(|e| panic!("reading fixture dir {}: {e}", root.display()))
        .filter_map(|e| e.ok())
        .map(|e| e.path())
        .filter(|p| p.is_file())
        .collect();
    entries.sort();
    assert!(
        !entries.is_empty(),
        "no fixtures found in {}",
        root.display()
    );

    let mut failures = Vec::new();
    for path in entries {
        let raw = std::fs::read_to_string(&path)
            .unwrap_or_else(|e| panic!("reading fixture {}: {e}", path.display()));
        match parse_fixture(&raw) {
            Err(e) => failures.push(format!("{}: PARSE ERROR: {e}", path.display())),
            Ok(fixture) => failures.extend(run_fixture(&path, &fixture)),
        }
    }
    failures
}

fn assert_language_clean(dir: &str) {
    let failures = run_language_dir(dir);
    assert!(
        failures.is_empty(),
        "\n{} {dir} fixture(s) failed:\n\n{}\n",
        failures.len(),
        failures.join("\n")
    );
}

#[test]
fn rust_fixtures() {
    assert_language_clean("rust");
}

#[test]
fn kotlin_fixtures() {
    assert_language_clean("kotlin");
}
