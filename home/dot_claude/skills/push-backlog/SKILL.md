---
name: push-backlog
description: Push a backlog of unpushed commits so CI runs on each commit that deserves it, wait for the results, repair reds by rewriting the unpushed history behind them, and loop until everything is pushed. Use whenever the branch is more than a commit or two ahead of its remote and the user wants it pushed with real CI coverage, e.g. "push my commits", "push these one at a time", "drain the backlog", "get these through CI", "I'm 15 ahead, push them", "push and fix whatever goes red", or when a just-pushed commit went red and more unpushed commits wait behind it. Also use for planning how to push (sequential vs parallel waves, what to skip or reorder) even when the user only asks for advice.
---

# Push Backlog

The user wants a backlog of local commits pushed so that CI runs fully on each commit that
deserves its own run, with a high green rate. A few reds are acceptable; a red is repaired by
editing the still-unpushed history right behind it, so the next commit goes green, and the loop
continues until the backlog is empty.

No fixed procedure fits every project. CI that takes four minutes with free concurrency calls for
a different plan than CI that takes forty minutes on one runner, and the user's priorities change
session to session. This skill gives you the facts to gather, the levers you have, how they trade
off, and the mechanics. The judgment is yours. Investigate first, then decide.

## Authority

Invoking this skill is the user's explicit request to push, create fix commits, amend, reorder and
rebase the **unpushed** commits on the current branch without asking at each step. For this branch
and this loop, that overrides the usual rules against committing and rewriting history. It does
not cover:

- **Already-pushed history.** Rewriting it, or any force push, needs a yes first, every time.
- **The user's uncommitted or staged work.** The staging area is their review. Never fold it into
  a commit, stash it away or un-stage it (see the tree-state section in
  `references/history-surgery.md`).
- **Anything outside the backlog**: other branches, tags, releases, workflow dispatches that
  publish.

How much to check in beyond that depends on the situation. When CI is slow, the backlog is long
and the user will plainly be away, act on your best judgment and save questions for the moments
that really need them. When CI is quick and the user is watching, there is no cost to checking in
on a fork in the road.

## Phase 1: Survey before you ask

Gather everything you can measure, so the interview only covers what you can't. Read, don't guess.
Most of this is cheap.

**Backlog and starting point**

```bash
git fetch
git status -sb
git log --reverse --format='%h %s' --stat @{u}..HEAD
gh run list --commit <full upstream sha>    # is the starting point already red?
```

If the upstream tip is red, find out why before planning anything. Every commit you push inherits
that red until something fixes it, and the fix may already be in the backlog.

**CI shape.** Read the workflow files that a push to this branch triggers. You are looking for:

- **Triggers and path filters.** A commit whose files all match `paths-ignore` triggers nothing,
  so it rides along with the next push.
- **Concurrency groups.** These decide whether parallel pushes are possible at all (see Phase 3).
- **Jobs per push and matrix size.** Wall time per push is set by the slowest workflow, not by the
  sum of workflows.
- **Side effects of a push**: deploys, releases, publishing, notifications. Note the conditions
  they run under (`if: github.event_name == 'push'`, environments) and how resilient they are.
- **Runner types.** Self-hosted usually means one runner, which is serial whatever else you do.
  On private repos, macOS and Windows runners cost more minutes.

**CI history.** `scripts/ci-history.sh` prints each workflow's median and maximum duration and its
red rate, the wall time per pushed commit, and how many pushed commits got their own run. The last
figure tells you how the user has been treating this repo.

**Local check cost and parity.** Find out what the project offers (`just --list`, the CI steps
themselves), roughly how long it takes, and whether it reproduces what CI checks. These two answers
vary enormously. Sometimes the full CI suite runs locally in two minutes and matches exactly.
Sometimes a handful of commits takes an hour and still misses what CI catches (other targets,
other platforms, wasm, cross-lints). Note how much disk a build tree eats, and whether a worktree
here is cheap or painful: build tree size, git-excluded or vendored content a fresh worktree won't
have, relative-path assumptions.

**Working tree.** Is it dirty, and is anything staged? Note whether the user is likely to keep
working in it.

**Per-commit risk.** Read each commit's file list and judge how likely it is to break CI:

| Signal | Read |
| --- | --- |
| Only files matching the path filters | Triggers no CI; push it with its neighbour |
| Dependencies, lockfile, feature flags, toolchain, build scripts, CI files | High risk, often failing only on targets you don't build locally |
| Platform- or cfg-gated code, other targets (wasm, Windows, macOS) | High risk when local checks cover only the host |
| Removes, renames or moves items other modules or crates use | Medium-high; the breakage surfaces far from the diff |
| Self-contained logic with its own tests | Low |
| Comes after an unfixed red | Inherits that red, whatever its own content |
| Already diagnosed as red (e.g. the red upstream, fix later in the backlog) | Known red; plan around it instead of discovering it again |

## Phase 2: A short interview

Ask only what the survey couldn't settle, in one `AskUserQuestion` batch, and include the plan you
would choose so the user can just approve it. Skip any question the user's request already
answered ("just push them, I want to be done" settles priority). The usual unknowns:

1. **Session priority.** These pull in different directions, and the user's mood genuinely varies:
   - *Wrap it up* (antsy): bigger waves, batch-skip low-value runs, accept a few reds, fewer local
     checks.
   - *Everything green* (patient): a run per commit, local prediction on risky commits, sequential
     where a red would cascade.
   - *Cheap* (cost-driven): fewer CI minutes and tokens. Batch-skip trivially safe commits, smaller
     fan-out, no speculative reruns, grep logs instead of reading them whole.
2. **Presence and the tree.** Will they be around, and will they keep editing or committing in this
   checkout? This sets how often you check in and how history surgery is done.
3. **Reds that should stay visible.** Rarely, a known-failing commit should keep its red in the log
   (a test demonstrated failing before its fix). Ask only when a commit looks like that.
4. **The plan itself.** Present the derived facts in a few lines, then the proposed waves, skips and
   reorders, with the alternative you passed over.
5. **A version bump**, when the repo versions and the backlog holds bump-worthy work. The backlog
   is still free to rewrite, so this is the cheapest moment to insert a release commit; the
   `versioning` skill decides the version and where it goes. Ask in the same batch, and settle it
   before planning waves, since a release commit changes the SHAs and may add a tag push.

Don't ask about anything with an obvious default: commit message style follows the project, and
the push order is oldest first.

## Phase 3: Plan

### Facts that constrain every plan

- **CI runs on the tip of each push, not on every commit in it.** To give a commit its own run,
  push it alone: `git push origin <sha>:refs/heads/<branch>`. To skip a commit's run, push it in
  the same push as a later commit.
- **Concurrency groups** (GitHub): a group holds one running and one pending run, and a newly queued
  run cancels the pending one. So pushing several commits quickly into a grouped workflow silently
  skips the middle ones. `cancel-in-progress: true` cancels the running one as well. `queue: max`
  lets up to 100 runs wait in order, which turns the painful sequential case into "push the whole
  backlog at once and CI works through it". Reds still cascade, though, so suggest it to the user
  as a workflow change rather than assuming it.
- **A red in a parallel wave contaminates the rest of that wave.** By the time commit *i* comes back
  red, commits *i+1..k* are already pushed and contain the same breakage, so they are red too and
  stay red in the log. Fix commits can only go after the wave. This is the real price of
  parallelism, so wave size should shrink as risk rises.
- **Runner caps.** Jobs per push times wave size, compared against the account's concurrent-job
  cap, tells you when a bigger wave stops buying wall time and just queues.

### The levers

- **Sequential**: push, wait, repeat. This gives the most greens and the most precise fixes, and
  costs N x T of wall time. It is the right default when a red would cascade, when concurrency is
  effectively one, or when every fix rewrites what follows anyway.
- **Parallel waves**: push *k* commits one after another (each its own push, so each gets its own
  run), then wait for all of them. Wall time is roughly ceil(N/k) x T. Use larger waves for
  low-risk runs, and end a wave just before a high-risk commit so that commit leads the next wave
  and can't contaminate its neighbours.
- **Batch-skip**: push a span as one push so only its tip runs. Use it for spans that match the
  path filters, spans already known to be red, and trivially safe spans when cost or speed matters.
  It also keeps guaranteed reds out of the log.
- **Move a known fix earlier.** When a later commit fixes an earlier red, reordering it to sit just
  after the red usually beats batch-skipping the span, because the commits in between then get
  real green runs. First check that it moves cleanly: compare the files it touches with those the
  intervening commits touch, and remember it can depend on their code without any textual
  conflict. When the fix is bundled inside a larger commit, splitting it out is a judgment call.
  It is worth it when the fix is whole files or clearly separable hunks. When it is tangled across
  several commits, batch-skip instead.
- **Predict locally.** Run the relevant checks on a commit before pushing it. A red caught this way
  is the best outcome, because the culprit is still unpushed and can simply be amended. Spend this
  where risk is high and the local check is cheap and matches CI. Pipeline it: check the next
  wave's risky commits while the current wave runs on CI.
- **Side effects.** When pushes deploy, the newest commit must be the last deploy. Pushing the
  final commit alone, after the rest have settled, is usually enough. Where a deploy needs time to
  come up, space the pushes so deploys land one after another. Most projects are low-stakes, so
  work out how resilient the deploy is rather than serializing by reflex.

### Worked reasoning (illustrative)

A 15-commit backlog sits on a red upstream tip: a missing dependency breaks the wasm build. The
fix is the 6th backlog commit, a two-line manifest change touching nothing the first five touch.
CI takes about 15 minutes per push, there is no concurrency group, there are about 5 jobs per push,
and green pushes deploy. One commit is docs-only and path-ignored.

Plan: move the fix to the front, so the first five commits are no longer doomed. Let the docs-only
commit ride with its neighbour. The risky commits are two dependency-heavy telemetry commits and a
CI-config change, so each leads its own wave. Fill the waves with the low-risk fixes. Push the tip
alone at the end so its deploy is the one that lands last. Run a local wasm clippy on the risky
commits while the earlier waves are on CI, because that target is where this repo's reds have
come from.

## Phase 4: The loop

Each round:

1. **Re-derive the backlog.** Run `git fetch` and `git log --reverse @{u}..HEAD`. SHAs change with
   every rewrite, and the user may have committed more, so never work from remembered SHAs. If the
   upstream moved under you (another machine, a bot), stop and ask whether to rebase onto it.
2. **Predict locally** where the plan calls for it.
3. **Push** each commit in the wave, oldest first, as a fast-forward:
   `git push origin <sha>:refs/heads/<branch>`. Never pass `--force` in this step.
4. **Wait.** Run `scripts/ci-wait.sh <sha>...` with Bash `run_in_background` and stop polling. Its
   exit is your wake-up. It reports each run as it finishes, the failed jobs and steps, and a
   green/red/no-runs verdict per SHA. Use the wait for the next wave's local checks or for
   preparing a fix you already suspect.
5. **Triage every red before fixing any.** Read the failure: `gh run view <id> --log-failed`,
   grepped for the real error. The first `::error` is often cache or tool-install noise. Classify
   it as:
   - a real failure (reproduce it locally when that is cheap, so the fix is confirmed before
     another CI round),
   - inherited from an earlier red in the same wave,
   - a flake (rerun once with `gh run rerun <id> --failed`),
   - or infrastructure (runner, secret, registry, outage).

   When a wave brings back several distinct problems, fix them all locally before the next push
   instead of letting CI reveal them one round at a time.
6. **Place the fix.** The red commit R is pushed; what follows it is not.
   - **Default: insert a fix commit right after R.** The log then reads red, fix, green, which is
     honest and cheap.
   - **Amend the next commit instead** when the fix naturally belongs to its content (it touches
     the same area, or it is that commit's own follow-up).
   - **Propose rewriting R and force-pushing** when a standalone fix commit would look wrong: a
     trivial oversight in R that normal local checks would have caught. This is pushed history, so
     ask first.
   - **If the red was predicted locally before pushing, amend the culprit itself.**
   - Write commit messages in the project's style, describing what the fix changes, not that CI
     was red.

   Recipes are in `references/history-surgery.md`, all tested. Two traps in them catch everyone:
   - Git opens the configured editor on `rebase --continue` after a conflict, and on any commit
     without a message. Without a terminal that fails or hangs, so always use
     `GIT_EDITOR=true git rebase --continue`, and pass `-m`/`-C`/`--no-edit` to commit.
   - `--autostash` and `reset --keep` lose the user's staged state, and `reset --merge` can
     delete a staged new file outright. When
     anything is staged, do the surgery in a worktree and hand it back with a branch switch, as
     the reference shows.

   Verify every rewrite with `git range-diff` before the next push.
7. **Report** one line per round (format below).

## Reporting

Write one terse line per round in chat, for example:
`round 3: pushed a1b2c3d..f4e5d6c (3) | 2 green, 1 red: wasm clippy | next: fix after f4e5d6c`

Use `PushNotification` when a red needs the user's decision, or when the loop ends (early, or
complete after a long unattended stretch). Don't use it for routine greens. It suppresses itself
when the user is at the terminal, so sending one when it is warranted costs nothing.

Waiting in the background spends no tokens; reading whole CI logs spends many. Grep the logs.

## Stopping early

Stop, report, and hand back when:

- **The fix needs a design decision**: the behaviour is in question, a test's expectation is in
  doubt, or the right approach is unclear. A mechanical fix is yours; a decision is theirs.
- **CI infrastructure is broken**: reruns don't clear it, a secret is missing, or a registry,
  action or runner is down.
- **The same problem survives two different fixes.** Don't guess a third time with the same
  approach.

A series of *different* problems is not that. Each one fixed is progress. Weigh whether the next
fix is likely the last, the session's priority, and whether you are spending CI rounds to discover
problems one at a time that you could reproduce locally in one go. Keep going while that balance
holds.

Pause and ask, without abandoning the loop, when: a fix needs pushed history rewritten; your
surgery would touch files the user has uncommitted changes in; or the upstream moved.

When you stop, say what is pushed, what is left, the current red and your diagnosis, and what you
would do next.

## References

- `references/history-surgery.md`: non-interactive rebase recipes (insert, amend, reorder, split a
  fix out), handling a dirty or busy tree, worktree surgery, verifying with range-diff, and
  force-pushing pushed history safely. Read it before the first rewrite.
- `references/ci-mechanics.md`: GitHub Actions details (push and run discovery, path filters, skip
  markers, concurrency, reruns, log triage, runner caps, deploy ordering) and pointers for other CI
  hosts.
- `scripts/ci-wait.sh SHA...`: wait for every push-triggered run on the given commits and report.
  `--help` for options.
- `scripts/ci-history.sh`: CI duration and red-rate history for planning.
