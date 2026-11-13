#!/usr/bin/env bash
# Summarize recent push-triggered GitHub Actions runs on a branch, for planning a push loop.
#
# Usage: ci-history.sh [-R owner/repo] [-b branch] [-n runs]
#
# Prints per-workflow run count, non-green count, and median/max duration in minutes; wall time per
# pushed commit (first run created to last run finished); and how many first-parent commits in the
# covered span got a run of their own.
set -uo pipefail

repo_args=()
branch=""
limit=150

while (($#)); do
  case $1 in
    -R | --repo) repo_args=(-R "$2"); shift 2 ;;
    -b | --branch) branch=$2; shift 2 ;;
    -n) limit=$2; shift 2 ;;
    -h | --help) sed -n '2,8p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)
[[ -z $branch ]] && branch=${upstream#*/}
[[ -z $branch ]] && { echo "no upstream; pass -b <branch>" >&2; exit 2; }

json=$(gh run list "${repo_args[@]}" --event push --branch "$branch" --limit "$limit" \
  --json headSha,workflowName,createdAt,startedAt,updatedAt,conclusion,status) || exit 1

jq -r '
  def secs: fromdateiso8601;
  def mins: . / 60 * 10 | round / 10;
  def median: sort | if length == 0 then 0 else .[length / 2 | floor] end;
  def green: .conclusion == "success" or .conclusion == "skipped" or .conclusion == "neutral";
  map(select(.status == "completed")) as $runs
  | if ($runs | length) == 0 then "no completed push runs on this branch" else
    ( ["workflow", "runs", "not_green", "median_min", "max_min"] | @tsv ),
    ( $runs | group_by(.workflowName)[]
      | (map((.updatedAt | secs) - (.startedAt | secs))) as $d
      | [.[0].workflowName, length, (map(select(green | not)) | length), ($d | median | mins), ($d | max | mins)]
      | @tsv ),
    "",
    ( $runs | group_by(.headSha)
      | map((map(.updatedAt | secs) | max) - (map(.createdAt | secs) | min)) as $w
      | "pushed_commits_with_runs\t\(length)\nwall_min_median\t\($w | median | mins)\nwall_min_max\t\($w | max | mins)" ),
    ( $runs | map(select(green | not) | .headSha) | unique | "commits_not_green\t\(length)" )
  end
' <<<"$json"

oldest=$(jq -r 'map(select(.status == "completed")) | min_by(.createdAt) | .headSha // empty' <<<"$json")
if [[ -n $oldest && -n $upstream ]] && git cat-file -e "$oldest^{commit}" 2>/dev/null; then
  span=$(($(git rev-list --count --first-parent "$oldest..$upstream") + 1))
  ran=$(jq '[.[].headSha] | unique | length' <<<"$json")
  echo "first_parent_commits_in_span	$span"
  echo "share_with_own_run	$ran/$span"
fi
