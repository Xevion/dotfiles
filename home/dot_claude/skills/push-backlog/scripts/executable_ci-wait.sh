#!/usr/bin/env bash
# Wait for every GitHub Actions run that pushes of the given commits started, then report.
#
# Usage: ci-wait.sh [-R owner/repo] [--event push|''] [--grace SEC] [--interval SEC] [--timeout MIN] SHA...
#
# Output, one line per event as it happens:
#   run  <sha7> <conclusion> <workflow> <url>
#   fail <sha7> <workflow> / <job>: <failed steps>
#   sha  <sha7> green|red|no-runs|timeout
#   done green=<n> red=<n> no-runs=<n> timeout=<n>
# Exit: 0 when nothing is red or timed out, 1 when anything is red, 3 on timeout only, 2 on bad usage.
#
# --grace (default 90s) is how long to wait for runs to appear before calling a SHA no-runs.
# --event defaults to push; pass '' to include every event (workflow_run chains, dispatches).
set -uo pipefail

repo_args=()
event=push
grace=90
interval=30
timeout_min=120
shas=()

while (($#)); do
  case $1 in
    -R | --repo) repo_args=(-R "$2"); shift 2 ;;
    --event) event=$2; shift 2 ;;
    --grace) grace=$2; shift 2 ;;
    --interval) interval=$2; shift 2 ;;
    --timeout) timeout_min=$2; shift 2 ;;
    -h | --help) sed -n '2,15p' "$0"; exit 0 ;;
    -*) echo "unknown flag: $1" >&2; exit 2 ;;
    *)
      full=$(git rev-parse --verify --quiet "$1^{commit}") || { echo "not a commit: $1" >&2; exit 2; }
      shas+=("$full"); shift ;;
  esac
done
((${#shas[@]})) || { echo "no SHAs given" >&2; exit 2; }

event_args=()
[[ -n $event ]] && event_args=(--event "$event")

is_green='.conclusion == "success" or .conclusion == "skipped" or .conclusion == "neutral"'
declare -A reported verdict
start=$(date +%s)

report_failed_jobs() {
  local sha7=$1 wf=$2 id=$3
  gh run view "${repo_args[@]}" "$id" --json jobs 2>/dev/null |
    jq -r --arg s "$sha7" --arg w "$wf" '.jobs[]
      | select(.conclusion == "failure" or .conclusion == "timed_out" or .conclusion == "cancelled")
      | "fail \($s) \($w) / \(.name): \([.steps[] | select(.conclusion == "failure") | .name] | join(", "))"'
}

while :; do
  elapsed=$(($(date +%s) - start))
  waiting=0
  for sha in "${shas[@]}"; do
    [[ -n ${verdict[$sha]:-} ]] && continue
    sha7=${sha:0:7}
    if ! json=$(gh run list "${repo_args[@]}" --commit "$sha" "${event_args[@]}" --limit 100 \
      --json databaseId,status,conclusion,workflowName,url 2>&1); then
      echo "warn gh run list failed for $sha7: ${json%%$'\n'*}" >&2
      waiting=1
      continue
    fi

    while IFS=$'\t' read -r id concl wf url; do
      [[ -z $id || -n ${reported[$id]:-} ]] && continue
      reported[$id]=1
      echo "run  $sha7 $concl $wf $url"
      case $concl in success | skipped | neutral) ;; *) report_failed_jobs "$sha7" "$wf" "$id" ;; esac
    done < <(jq -r '.[] | select(.status == "completed") | [.databaseId, .conclusion, .workflowName, .url] | @tsv' <<<"$json")

    total=$(jq length <<<"$json")
    open=$(jq '[.[] | select(.status != "completed")] | length' <<<"$json")
    if ((elapsed < grace)) || ((open > 0)); then
      waiting=1
      continue
    fi
    if ((total == 0)); then
      verdict[$sha]=no-runs
    elif (($(jq "[.[] | select(($is_green) | not)] | length" <<<"$json") > 0)); then
      verdict[$sha]=red
    else
      verdict[$sha]=green
    fi
    echo "sha  $sha7 ${verdict[$sha]}"
  done

  ((waiting == 0)) && break
  if ((elapsed > timeout_min * 60)); then
    for sha in "${shas[@]}"; do
      [[ -z ${verdict[$sha]:-} ]] && verdict[$sha]=timeout && echo "sha  ${sha:0:7} timeout"
    done
    break
  fi
  sleep "$interval"
done

declare -A count=([green]=0 [red]=0 [no-runs]=0 [timeout]=0)
for sha in "${shas[@]}"; do count[${verdict[$sha]}]=$((count[${verdict[$sha]}] + 1)); done
echo "done green=${count[green]} red=${count[red]} no-runs=${count[no-runs]} timeout=${count[timeout]}"
((count[red] > 0)) && exit 1
((count[timeout] > 0)) && exit 3
exit 0
