# Tagging and Release-Commit Recipes

Every recipe here was run against a scratch repo on git 2.54. Write literal SHAs, versions and
identities into each command; placeholders are in angle brackets.

## Contents

1. Tag types and what each one shows
2. A tag for a release cut now
3. A backdated tag for a past commit
4. Rewrites move SHAs and restamp committers
5. Retagging after a rewrite
6. Finding and pruning tags
7. Keeping foreign tags out
8. Contribution-graph duplication: what is known

## 1. Tag types and what each one shows

| | Lightweight | Annotated |
| --- | --- | --- |
| Stores | a ref to a commit | an object with tagger, date, message |
| Date shown by tools and GitHub | the commit's | the tagger date |
| `git describe` default | ignored | used |
| Use | never, for versions | every version tag |

An annotated tag created today on a 2024 commit says "tagged 2024-11-14" forever unless the
tagger date is set by hand. A batch of annotated tags all stamped within seconds (`survey.py`
reports this as BURST) is the clearest sign of generated tagging.

## 2. A tag for a release cut now

```bash
git tag -a v0.4.0 -m "Map editor, faster pathfinding, save-file fixes" <release-commit-sha>
git push origin v0.4.0                     # one tag; never `git push --tags`
```

`git push --tags` pushes every local tag, including backup and checkpoint tags the user keeps
locally on purpose. Push tags by name.

## 3. A backdated tag for a past commit

Take the tagger date and identity from the tagged commit's committer, so the tag reads as made
when the commit was:

```bash
git log -1 --format='%cI | %cn | %ce' <sha>
# 2024-03-20T15:40:00-06:00 | Era Person | era@example.com
GIT_COMMITTER_DATE='2024-03-20T15:40:00-06:00' \
GIT_COMMITTER_NAME='Era Person' GIT_COMMITTER_EMAIL='era@example.com' \
  git tag -a v0.1.0 -m "First usable build" <sha>
git for-each-ref --format='%(refname:short) %(taggername) %(taggerdate:iso-strict)' refs/tags/v0.1.0
```

A small delay after the commit (minutes to a day) is realistic too; never a date before the
commit. The identity must be one the user actually used in that era; `timeline-revival` has the
identities and the timezone rules. Tags are unsigned unless the repo's era-correct commits are
signed.

## 4. Rewrites move SHAs and restamp committers

Inserting a release commit with a rebase changes the SHA of every commit after it, so **tag last**,
after the history is final and verified with `git range-diff`.

A plain rebase also sets every rewritten commit's **committer date to now and the committer to
the current git identity**, while keeping author fields. In recent history that is harmless. On
backdated or era-specific history it exposes the rewrite, and a backdated tag then disagrees with
its own commit. Keep both:

```bash
GIT_COMMITTER_NAME='Era Person' GIT_COMMITTER_EMAIL='era@example.com' \
  git rebase --committer-date-is-author-date <base>
```

The env identity applies to every commit the rebase writes, so it only fits a span made under one
identity. `--committer-date-is-author-date` loses any gap the original commits had between author
and committer dates. For mixed identities or exact original committer dates, rewrite with
`git filter-repo` callbacks instead, per `timeline-revival`.

The non-interactive insert, amend and reorder recipes live in `push-backlog`'s
`references/history-surgery.md`; add the two options above to them when the span is backdated.

## 5. Retagging after a rewrite

A tag keeps pointing at the old commit after a rewrite. Move it to the new SHA, keeping its
message, date and tagger:

```bash
git for-each-ref --format='%(taggerdate:iso-strict)|%(taggername)|%(taggeremail:trim)' refs/tags/v0.1.0
git tag -l --format='%(contents)' v0.1.0
GIT_COMMITTER_DATE='<date>' GIT_COMMITTER_NAME='<name>' GIT_COMMITTER_EMAIL='<email>' \
  git tag -f -a v0.1.0 -m '<message>' <new-sha>
```

A moved tag that was already pushed needs `git push --force origin v0.1.0`, which is a rewrite of
pushed state: ask first. Anyone who fetched the old tag keeps it until they delete it locally.

## 6. Finding and pruning tags

```bash
for t in $(git tag); do git merge-base --is-ancestor "$t" HEAD || echo "not in HEAD: $t"; done
git ls-remote --tags --refs origin          # what the remote really has
```

To prune generated tags down to the real ones, keep the tags whose commit changed a version line
(the survey's bump-history section), delete the rest:

```bash
git tag -d <tag>...                          # local
gh release list                              # does any of them carry a Release?
gh release delete <tag> --cleanup-tag --yes  # tags with a Release: deletes both
git push origin --delete <tag>...            # tags without one
```

Every remote deletion needs the user's yes on the exact list. Handle a tag's Release explicitly
rather than deleting only the tag under it.

## 7. Keeping foreign tags out

A vendored or forked upstream remote brings its tags on every fetch. They belong to someone else's
version line:

```bash
git ls-remote --tags --refs <upstream>       # confirm which local tags came from it
git tag -d <their tags>                      # only exact name-and-object matches
git config remote.<upstream>.tagOpt --no-tags
```

An explicit `git fetch --tags <upstream>` still overrides `tagOpt`.

## 8. Contribution-graph duplication: what is known

From GitHub's contribution troubleshooting docs:

- Commits count toward the profile graph when they are on the default branch (or `gh-pages`) of a
  non-fork repo and the author email is linked to the account. New commits can take up to 24 hours.
- After squashing or force-pushing rewritten history, the profile "may continue to count commits
  that are no longer in the repository's visible history", private repos included.
- GitHub Support may be able to rebuild contribution data. Community reports say Free-plan tickets
  are often auto-closed; paid plans get human support.
- Removing an email from the account drops its historical contributions; adding it back triggers
  a rebuild, with up to 24 hours of lag each way.

Observed by the user: deleting the repository removes the duplicates.

Measured on a throwaway private repo, 5 commits backdated onto otherwise empty days (2026-11):

| Step | Count on the test days |
| --- | --- |
| First push | 5, within seconds |
| Rewrite force-pushed (same dates, new SHAs) | 10, within about 20 s |
| Archive and unarchive, public and back to private, rename | 10, no effect |
| Default switched to a branch holding none of the commits | 0, about a minute later |
| Default switched to a branch at the rewritten tip | 5, about a minute later |
| Default switched back to `master`, temp branch deleted | 5, stable |

So the counts are stored per pushed SHA, and changing the default branch rebuilds them from what
the new default branch reaches. Untested: public repos, and repos of real size.

**Recipe** after force-pushing a rewritten `master`:

```bash
git push origin master:refs/heads/rebuild-tmp
gh repo edit <owner>/<repo> --default-branch rebuild-tmp
# wait until the affected dates drop back (about a minute), measured as below
gh repo edit <owner>/<repo> --default-branch master
git push origin --delete rebuild-tmp
```

Both switches trigger a rebuild; the second recounts the same commits, so it holds. Check the
calendar for the rewritten commits' dates before and after. If the counts do not drop within a few
minutes, report it rather than retrying: the behavior may have changed.

Measure with the GraphQL API rather than by eye:

```bash
gh api graphql -f query='query { viewer { contributionsCollection(from: "2019-01-01T00:00:00Z",
  to: "2019-12-31T23:59:59Z") { contributionCalendar { totalContributions weeks {
  contributionDays { date contributionCount } } } } } }'
```

`from` and `to` may span at most one year.
