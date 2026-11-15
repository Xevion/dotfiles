---
name: versioning
description: Decide, place and cut version bumps and git tags with judgment, per repository. Covers semver bump choice, release units in workspaces and monorepos, release commits, annotated tags, changelogs, publish triggers, retroactive versioning of untagged or stale history, and keeping the release record plausible as human work. Use when the user asks to bump, version, tag, release, cut or publish something, asks how a repo should be versioned, or wants a triage of versioning across projects. Also use, as a suggestion through the question tool and never silently, when a repo that versions is about to push a backlog, has just landed a user-visible feature or breaking change, or is wrapping up a substantial session with bump-worthy work since its last tag.
---

# Versioning

Version bumps keep not happening because every automated system tried so far failed on the odd
cases: crates that should release apart, crates that must release together, a release PR nobody
merges, a burst of releases while iterating on a pipeline. This skill replaces that with a
Claude-assisted manual process: you survey, judge and propose, and the user says one yes.
Cutting a release must cost the user one decision. A process that technically works but is
tedious enough to stall a project has failed.

No fixed procedure fits every repo. Gather the facts, find out how the repo has done it before,
judge whether that way is any good, then propose.

## Authority

- **Never bump silently.** A bump is either explicitly requested or proposed through the question
  tool and accepted.
- **A yes to a bump covers its local work**: the release commit, lockfile and changelog updates,
  annotated tags, and moving the release commit to its right place in **unpushed** history.
- **Pushing tags is asked once per scope.** If the user approved "cut v0.9.0 and push it", push it.
  Ask again only when the scope changes materially: the push will now publish to a registry, a
  retroactive tag will now be visible in a public repo, or pushed history must be rewritten. Small
  additions within the agreed scope are intended; re-asking for them is noise.
- **Rewriting pushed history always needs a yes**, with its cost stated (see "Cost of rewriting
  pushed history" below).
- **Deleting pushed tags or Releases always needs a yes**, with the exact list.
- Everything in the user's git rules still holds: no touching their staged work, no attribution
  trailers, commit by pathspec.

## When to nudge

Suggest a bump, through the question tool, at these moments in a repo that versions:

1. **Before a backlog is pushed** (including from `push-backlog`): history is still free to edit,
   so this is the cheapest point to place a release commit.
2. **After a user-visible feature or a breaking change lands.**
3. **On wrap-up** of a substantial session, if bump-worthy work happened since the last tag.

Limits, so the nudge never becomes nagging:

- At most one suggestion per repo per session, unless new bump-worthy work lands after a "not yet".
- A "no" holds for the rest of the session.
- A repo whose release doc says `versioning: none`, or that is clearly unversioned by nature
  (dotfiles, a personal site that only deploys, coursework, exercise repos, scratch), is never
  nudged.
- Dormant repos are not nudged until work resumes in them.
- Only nudge for real work: a feat, a fix users would notice, a breaking change. A run of chores,
  CI, refactors or tests is not a release on its own.

The suggestion names the version, the reason in a few words, what the release commit touches and
what pushing the tag would trigger, for example: "Bump banner to v0.9.0? 2 features, 5 fixes since
v0.8.4. Release commit touches Cargo.toml, Cargo.lock, CHANGELOG.md; pushing the tag runs
release.yml (binaries + GitHub Release)." Offer: that version now, the alternative size, not yet.

## Phase 1: Survey

Run `git fetch --tags` first: CI-made tags and tags from other machines are otherwise missing,
and you will propose a version that already exists. Then run `scripts/survey.py <repo>`. It
prints:

- remotes, visibility, fork status, unpushed count
- tags grouped by prefix, lightweight vs annotated, tag date vs commit date, bursts and dense
  days, tags only on the remote or only local
- every manifest with its version and `publish` setting, and exact path-dependency pins
- commits that changed a version line (the real bump history)
- unreleased commits since each prefix's latest tag, limited to that package's directory, by
  conventional type
- release workflows with their triggers, release-please or other tooling, changelogs, release docs

Then check what the script cannot see:

- **Registries.** `cargo info <crate>`, `npm view <pkg> versions time`, `pip index versions <pkg>`.
  Registry versions and publish times are permanent and public: they are the ground truth every
  tag must agree with. A name already owned by someone else (crates.io `ferrite`) means that
  package is not published under that name.
- **GitHub Releases**: `gh release list`. Which tag holds "Latest", and whether unpublishable
  units got Releases.
- **Open release PRs** (`gh pr list`). A release PR sitting for months is a failed workflow.
- **The release doc** (next section) and the past bump commits' shape (`chore: bump to v0.80.0`,
  `chore(release): ...`, a release-please merge).
- **Release units** (below) for any workspace or monorepo.

## The project's release doc

Every repo that versions gets one place that says how it releases. Look for it first: a
`## Releases` section in the repo's `CLAUDE.md`, or `docs/RELEASING.md`. Read it, then check it
against what the history actually shows. When they disagree, or the documented process is weak,
say so and propose a change rather than following it.

If none exists, write it the first time you handle the repo, after the user has settled the
choices. Prefer the `CLAUDE.md` section (untracked in most repos). Use `docs/RELEASING.md` only
where a human maintainer would plausibly have written one, such as a public library with
contributors, and write it as the user's own document. Contents:

- `versioning: semver` (or `none`), and the 0.x policy and what 1.0 would mean here
- visibility tier: public, private awaiting public, private
- release units: each unit's packages, tag format, and what links them
- files a release touches (manifests, lockfile, changelog, embedded version strings)
- what a pushed tag triggers, and how publishing happens
- changelog: yes or no, and where
- extraction candidates (modules likely to leave the repo and get their own version line)
- anything odd: foreign tags, pinned exact versions, registry names that are taken

## Phase 2: Judge the existing workflow

Signals that the current way is broken and should be changed, not followed:

| Signal | What it usually means |
| --- | --- |
| Manifest version ahead of every tag | Bumps happened but were never tagged or never pushed |
| Release PR open for weeks | The automation's nudge is being ignored; it is friction, not help |
| Unrelated packages released in one commit or PR | Lockstep that nobody chose, often a combined release PR |
| Tags or Releases for `publish = false` packages | Tooling releasing things that do not ship |
| Several releases in one day | Iterating on the pipeline in public, or tag-per-commit |
| Tags fetched from an upstream remote | Foreign version line; never continue it |
| Release gated on a CI run that is red or keeps cancelling | Releases silently never happen |
| CI on floating `stable` with `-D warnings` | Each Rust release can add clippy lints that turn it red with no code change |
| Local clone behind remote tags or commits | CI or bots moved the remote; fast-forward before cutting anything |

Automated release tooling (release-please, changesets, semantic-release) can stay where it works
cleanly for a single package. For multi-unit repos, or wherever it has caused friction, the
default is manual: the bump is a release commit Claude prepares, and the tag is the trigger.

## Release units

A release unit is a set of packages that always carries the same version and releases together.
Decide units from evidence, not from the directory layout:

- **Must be one unit**: an exact pin between them (`version = "=0.2.1"`), or a build-time
  generator whose output must match its consumer (a codegen crate and the `-sys` crate it
  generates for, when both pin the same `cxx` generation). A caret requirement there is a latent
  break: a newer compatible generator gets picked up by users building an already-published
  consumer, because a library's lockfile does not reach its dependents.
- **Independent**: packages that only depend through normal caret requirements and change
  separately in practice. Measure it: count commits since the last release that touch only A,
  only B, and both.
- **Not a unit at all**: `publish = false` packages that nothing external consumes. They get no
  tags and no Releases unless the user wants an internal version.

Cargo facts that decide what must be released together (npm and others behave alike):

- A path dependency with `version = "0.2.0"` publishes as `^0.2.0`, accepting any 0.2.x.
- Below 1.0, cargo treats a minor bump (0.2 to 0.3) as breaking and a patch bump as compatible.
- A compatible change to a dependency needs only the dependency released. Dependents pick it up on
  `cargo update`. Release the dependent too only once it relies on the new version, and raise its
  requirement then.
- A breaking change to a dependency forces a dependent release, since its published requirement
  cannot accept the new version. If the dependent re-exports the dependency's types, its bump is
  breaking too.
- `links = "..."` allows only one version of that crate in a dependency graph, so a breaking `-sys`
  release drags every dependent along.

One release commit may carry several unit tags. Tags of independent units use the unit prefix:
`idakit-v0.3.0`, `idakit-sys-v0.2.2`. A single-unit repo uses a bare `v0.3.0`.

Extraction candidates (a module expected to leave for its own repo) stay on the parent's version
until extracted, then start a fresh line in the new repo, unless they are published before
extraction.

## Choosing the bump

Semantic versioning, read the way cargo reads it:

- **0.x**: minor for features or breaking changes, patch for fixes and small additions.
- **1.0** is decided per project. The usual meaning is stable and feature-complete after a settled
  period, so it is rare. Never propose 1.0 because a number "feels high".
- The number does not track commit count. Each 0.x minor announces breaking changes or major
  features, and for a library a high minor reads as API churn. A lot of work shows as more
  releases at real arc boundaries, not a bigger number.
- Conventional commit types are evidence, not the answer. Twelve fixes and one small feat may be a
  patch, and a `refactor:` that changes a public signature is breaking. Read the unreleased commits
  that touch the unit's public surface.
- Go modules: the tag is the version. A major version past v1 needs the `/vN` module path.

## Plausibility

The release record must read like a person deciding to cut a release. Commit history, tag dates,
Release dates and registry publish times are all visible, and the user's work must not look
machine-generated.

- **Cadence**: under active work, about one or two releases a month, sometimes more, sometimes
  less. Quiet stretches have no releases.
- **Releases follow arcs of work**, not commit counts: a feature finished, a set of fixes worth
  shipping, a deploy, a first public showing.
- **Patches cluster after a release, not between them.** v0.9.1 the day after v0.9.0 for a real
  regression is normal. Ten patch tags in a week is not.
- **Never tag every commit**, and never cut several releases in one day while iterating on a
  pipeline. Test release machinery with a dry run (`cargo publish --dry-run`, `npm publish
  --dry-run`, a `workflow_dispatch` without the publish step), not with real releases.
- **Visibility tiers**:
  - *Public*: highest scrutiny. Everything is visible the moment it is pushed.
  - *Private awaiting public*: versions can be looser until launch, but before the visibility
    flips, review the tags and prune them to a sparse, plausible set. That review is the one time
    deleting tags is routine, since nobody has seen them.
  - *Private*: a version earns its place only if something reads it (`--version`, a deploy, a
    cache key, a download). Otherwise `versioning: none`.
- **Never create a GitHub Release retroactively.** Its date cannot be backdated. Create Releases
  only for releases cut now, and only in public repos that already use them.
- **Publishing**: crates.io and similar registries are for libraries and for binaries people want
  through `cargo install`. Specialized binaries ship as Release assets or not at all.

## Retroactive versioning

For long untagged or stale history, the default is **sparse milestones**:

1. **Recover real bumps first.** Commits that changed the version line are facts already in the
   history. Tag each one at its own commit, even when several sit close together.
2. **Then add a few milestone tags** where none exist: 2 to 4 over many months, at points a human
   would have stopped to cut a release (first working end-to-end build, a major feature arc). Never
   in a stretch that is only churn, never spread across a dormant gap.
3. **Close with a real release commit** at or near HEAD.

On already-pushed history, a milestone is **tag-only**. The tagged commit's manifest will still
say the old version, and that mismatch is fine: projects that adopt tagging late look like this.
In unpushed history, a real release commit can be inserted at the milestone.

Retroactive tags are **annotated and backdated**: the tagger date is the tagged commit's committer
date, and the tagger identity is the one the commits of that era use (`timeline-revival` holds the
identity and timezone rules). Recipes are in `references/tagging.md`.

Never extend history into the past to make room for versions. That is `timeline-revival`'s job, and
only on request.

## Cost of rewriting pushed history

A force-pushed rewrite makes GitHub count every rewritten commit a second time on the user's
profile contribution graph, and the stale counts stay after the old commits are unreachable.
**Switching the repo's default branch removes them**: it triggers a rebuild, about a minute later,
that recounts only what the new default branch reaches. So every force-push of a rewrite is
followed by the default-branch swap in `references/tagging.md` section 8, and by a check that the
counts for the rewritten dates actually dropped. Skipping it leaves permanent duplicates, which
matter to the user.

What a rewrite still costs, stated with the number of pushed commits it reaches:

- **CI history detaches**: rewritten commits lose their check statuses, and their runs stay listed
  under the old SHAs. Mild, since run logs expire with the retention setting anyway.
- **Links to old SHAs break**: commit URLs in issues, Linear and chat, open PRs, other clones.
- **Pushed tags on rewritten commits must be moved** (section 5 of the reference) and
  force-pushed.
- **Registry publish times never move**, so a published package's versions are fixed points.
- **The rebuild recounts the whole repo by today's rules**; commits under an email no longer on the
  account drop out too.
- **The evidence is one private test repo of 5 commits.** Verify the counts every time. If the
  rebuild does not land, a private repo without CI can still be deleted and recreated, which is
  the proven fallback.

Order of preference: a tag on pushed history (free), a release commit inserted near the pushed tip
(cheap), a deep rewrite (now workable, but only when the gain is real, and always with a yes).

## Mechanics

**Before cutting**: fast-forward to the remote, and make sure the release would pass the gate
that cuts it. A release-please PR only refreshes after its gating workflow is green, so a red
`Check` must be fixed first, reproduced locally on the toolchain CI uses (install that exact
version next to the default with `rustup toolchain install <ver> --profile minimal`). Install the
project's dependencies when its commit hooks need them; never skip a hook to get a release
commit through.

**Release commit**, one per release: the manifest versions, the lockfile regenerated by the tool
(`cargo check`, `bun install`, `uv lock`), the changelog entry, and any embedded version string.
pnpm has no `version` command; `npm pkg set version=X.Y.Z` edits only that field.
Message: `chore(release): v0.4.0`, or `chore(release): idakit v0.3.0, idakit-sys v0.2.2` for several
units. Follow the repo's own past shape when it has a clear one.

**Tag** the release commit, annotated, with a short summary of what the release brings as its
message: `git tag -a v0.4.0 -m "<summary>" <sha>`.

**Placing a release commit in unpushed history**: insert it right after the last commit it
releases, using the recipes in `push-backlog`'s `references/history-surgery.md`. Verify with
`git range-diff`, then tag the rewritten SHA (SHAs change with every rewrite, so tag last).

**Changelog**: only in public or published repos. Hand-written, Keep a Changelog style, grouped
by what users notice; not a commit dump. GitHub Release notes reuse the entry. A fix to
something new in the same release is not a `Fixed` entry. A `Security` entry names what was
updated and must not imply the project is clean: check open alerts first
(`gh api repos/<owner>/<repo>/dependabot/alerts --jq 'map(select(.state=="open")) | length'`).

**A fully rewritable repo** (no remote, or private with no CI and the user's yes) that needs
versioning from its start is rebuilt, not rebased: replay every commit's tree onto a new branch
with a transform that sets the versions current at that point, insert release commits with
changelogs, then tag. Do it in a clone, verify that each replayed commit keeps its metadata and
differs only in version lines, lockfile versions and changelogs, run the lockfile check at every
release point and the full suite at the tip, and move `master` only after the user has looked.

**Publishing**: prefer the pushed tag as the trigger. A workflow on `push: tags` checks the tag
matches the manifest version, picks the unit from the tag prefix, and publishes, with a
`workflow_dispatch` taking a tag as input for re-runs. Local `cargo publish` stays as the fallback
when CI is broken. Avoid combined release PRs and anything that releases units the user did not
name.

## Triage across many repos

When asked to triage all projects, survey each repo (the script plus the registry and Releases
checks), then present one table: repo, tier, current state, the finding that matters, and the
proposed action (none, write release doc, recover tags, milestone tags, first release, prune tags,
replace tooling). The user approves or edits it before any repo is touched. Execution can then go
to subagents one repo each, carrying only approved plans, since subagents cannot ask questions:
every judgment call is settled in the table.

## Worked examples

These are reasoning patterns, not templates.

- **A feature lands in a healthy repo** (v0.8.4, 13 commits since: 2 feats, 5 fixes, a tag-triggered
  release workflow). Propose v0.9.0 with v0.8.5 as the alternative. One yes covers the release
  commit, the tag and the push.
- **Backlog push in a never-versioned private repo** (157 commits over two months, 20 unpushed, no
  CI). Tag-only `v0.1.0` on the pushed commit where it first worked end to end; insert a real
  `chore(release): v0.2.0` into the unpushed backlog after the feature that earns it. Write the
  release doc. Since the repo has no CI, a full rewrite is also on the table if the user wants
  release commits throughout.
- **Ten months, 623 commits, never versioned.** About three points: first usable build, the
  largest feature arc, and a real release commit at HEAD. Not fifteen.
- **Manifest at 0.6.1, no tags.** The bump history shows `chore: bump to 0.6.1`, `chore: bump
  version` and similar. Tag each at its commit. Then find out why the tag-triggered workflow never
  ran: a release mechanism that never fired is a workflow to fix, not to imitate.
- **Dormant since last year, 108 commits past the last tag.** No nudge until work resumes; then one
  bump covers the gap. No tags spread back into the dormant period.
- **Two publishable crates plus a codegen crate, all in lockstep.** Commits since the last release:
  32 touched only the `-sys` side, 38 only the high-level crate, 44 both. The `-sys` crate and its
  generator share a pinned `cxx` generation, so they are one unit and need an exact pin; the
  high-level crate is its own unit. Lockstep everything was a tool's default, not a requirement.
- **Three crates released by a combined release PR**, two of them `publish = false`. One merge
  tagged and Released all three, an unpublished crate took the "Latest" badge, and releasing the
  real crate meant releasing everything with pending changes. That friction stalled the project.
  Fix: one unit (the published crate), manual release commits, tag-triggered publish, no tags for
  the unpublished crates.
- **A public package with four releases on one day** while the publish pipeline was being fixed.
  Every one is on the registry for good. Next time, dry-run the pipeline instead.
- **280 annotated tags created in one three-second window**, one per commit, running v0.0.1 to
  v0.81.1 in two months. Prune to the tags on real bump commits (`chore: bump to v0.80.0`) and
  delete the rest locally and on the remote, with the exact list approved first.
- **Upstream tags in a vendored fork** (`0.2.0` to `0.9.3` from the original project, local only).
  Delete them and set `remote.<upstream>.tagOpt = --no-tags`. The repo itself stays unversioned;
  crates good enough to extract get versions in their own repo later.

## References

- `scripts/survey.py [REPO]`: the Phase 1 survey. Read-only; fetch tags first.
- `references/tagging.md`: tag and release-commit recipes (backdated annotated tags, era identity,
  retagging after a rewrite, pruning tags, the `--no-tags` remote option), and what is known about
  contribution-graph duplication.
