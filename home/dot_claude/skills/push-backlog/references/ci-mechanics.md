# CI Mechanics

GitHub Actions specifics, then notes for other hosts. The `gh` usage here was checked against
gh 2.103. Run `--help` on a subcommand at first use, because flags and JSON fields drift between
versions.

## Pushing one commit at a time

```bash
git merge-base --is-ancestor origin/<branch> <sha>   # 0: fast-forward, 1: not, other: error
git push origin <sha>:refs/heads/<branch>
```

A push refspec's source can be any revision, but its destination must be a real ref name. A bare
SHA source gives git nothing to infer the destination from, so spell out `refs/heads/<branch>`.

Each push starts workflows for its tip SHA only. Commits in the middle of a multi-commit push get
no runs of their own. Runs show up a few seconds after the push; `ci-wait.sh` allows a grace
window before it concludes that a push triggered nothing.

## Path filters and skip markers

- `paths` / `paths-ignore` on a push are evaluated over a two-dot diff of the whole push (before
  to after). So a docs-only commit pushed alone triggers nothing, while pushed together with a
  code commit it is covered by that run. A push of more than 1,000 commits always runs
  path-filtered workflows, and a push that changes no files does not trigger them.
- A head commit message containing `[skip ci]`, `[ci skip]`, `[no ci]`, `[skip actions]` or
  `[actions skip]`, or a `skip-checks: true` trailer, stops `push` and `pull_request` workflows.
  Batch-pushing does the same without leaving a marker in the log, so prefer that.
- Runs skipped by a path filter or a marker leave their checks pending. That matters for a PR
  with required checks, but not for a direct push.

## Concurrency

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true   # or false / omitted
  queue: max                 # optional, since 2026-05
```

- **No `concurrency` key**: every push runs to completion in parallel, up to runner capacity.
  Parallel waves work.
- **A group without `queue`**: one running and one pending run per group. A newly queued run cancels
  the pending one, so in a burst of pushes only the first and the last run, and the middle ones show
  as `cancelled`.
- **`cancel-in-progress: true`**: the running one is cancelled too, so only the last push of a burst
  finishes. Push sequentially, or batch on purpose.
- **`queue: max`**: up to 100 runs wait in order, and none are dropped. You can push the backlog in
  one burst of single-commit pushes and let CI serialize them. It is incompatible with
  `cancel-in-progress: true`. If the repo groups runs without it and the backlog is long, suggesting
  it to the user is worth a line.

Groups can be per workflow or per job, and a group name shared across workflows makes them cancel
each other's pending runs. Read the actual group expression.

## Run discovery and waiting

```bash
gh run list --commit <full sha> --event push --json databaseId,workflowName,status,conclusion,url
gh run view <id> --json jobs --jq '.jobs[] | {name, conclusion}'
```

`gh run list` JSON fields: attempt, conclusion, createdAt, databaseId, displayTitle, event,
headBranch, headSha, name, number, startedAt, status, updatedAt, url, workflowDatabaseId,
workflowName. `gh run view` adds `jobs`, each with its steps. The default `--limit` is 20, so pass
a larger one when you count. Give `--commit` a full SHA (`ci-wait.sh` resolves short ones).

Workflows chained with `workflow_run` report `event: workflow_run` and appear only after their
parent finishes. `ci-wait.sh --event ''` includes every event when a repo relies on them.

`ci-wait.sh` is the default way to wait, because it covers every run a SHA started. For a single
known run, `gh run watch <id> --exit-status --compact` blocks until it finishes and exits non-zero
on failure, and `gh run view <id> --exit-status` checks once without blocking. Either one runs
under Bash `run_in_background`, never in a polling loop of your own.

## Reading failures cheaply

```bash
gh run view <id> --json jobs --jq '.jobs[] | select(.conclusion=="failure") | {name, databaseId, steps: [.steps[] | select(.conclusion=="failure") | .name]}'
gh run view <id> --log-failed | grep -E 'error(\[|:)|panicked|FAILED|failed with exit|-->'
gh run view --job <job databaseId> --log-failed     # one job, when a matrix fails several ways
```

Start from the failed job and step names, which is often enough to classify the red, and pull log
lines only after that. Cache restore warnings and tool-install `::error` lines are often noise
ahead of the real error, so look for the compiler or test output. Lines `gh` cannot attribute to a
step are labelled `UNKNOWN STEP`, which is normal. A failure in an upload, download, checkout or
cache step, with build and test steps green, is the infrastructure signature.

## Reruns

```bash
gh run rerun <id> --failed     # failed jobs and their dependents only
gh run rerun <id> --job <job databaseId>
```

The `--job` value is the job's `databaseId` from `gh run view --json jobs`, not the number in the
job's browser URL. Rerun once for a suspected flake or a transient infrastructure error. A failure
that reproduces on rerun is real until shown otherwise. A rerun bumps the run's `attempt` and sets
it back in progress, so `ci-wait.sh` on the same SHA waits for it. `gh run view --attempt <n>`
reads an earlier attempt.

## Capacity and cost

- Accounts and organizations have a concurrent-job cap that depends on the plan, and macOS jobs
  have a smaller one. Past the cap, jobs sit in `queued` and a larger wave no longer saves wall
  time. Queued jobs in the run view are the sign.
- On private repositories, minutes are billed, and macOS and Windows minutes cost more than Linux.
  Estimate it as jobs per push x median job minutes x pushes.
- Self-hosted runners are often a single machine, which means serial whatever the YAML says.

## Deploys and other side effects

Find jobs that deploy, release or publish, and the conditions they run under. With parallel waves
their completion order is arbitrary, so an older commit can deploy last. Push the newest commit
alone at the end so its deploy is the final one. Space pushes out when a deploy needs bring-up time
before the next one starts. An environment with required reviewers pauses the run in `waiting`,
which `ci-wait.sh` treats as still running. Say so, rather than waiting it out silently.

## Other CI hosts

The same ideas carry over and the commands differ. GitLab runs a pipeline per push tip, and
"auto-cancel redundant pipelines" (or `interruptible: true`) plays the role of
`cancel-in-progress`; `glab ci status` and `glab ci view` cover what `gh run` does. For anything
else, find out three things before planning: whether every push gets its own pipeline, whether
newer pushes cancel older ones, and how to list and wait on the pipelines for a SHA.
`ci-wait.sh` is GitHub-only.
