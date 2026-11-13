#!/usr/bin/env bash
# ci.sh <run dir> <wait minutes>: after the pull request is opened (url.txt), what its CI says and whether
# it merges. Polls gh pr checks every PR_CREATE_POLL seconds (default 20) until none is pending or the wait
# is over, then writes ci.txt: each check's bucket and name, and the PR's mergeable state. Exit 1 when a
# check failed or the pull request conflicts; pending at the end of the wait is reported and is not a failure.
# A repository with no CI reports "no checks".
set -u
r="$1"; wait="${2:-10}"; poll="${PR_CREATE_POLL:-20}"
url=$(cat "$r/url.txt" 2>/dev/null) || { echo "no pull request opened: nothing to check"; exit 0; }
end=$(( $(date +%s) + wait * 60 ))
while :; do
  rows=$(gh pr checks "$url" --json bucket,name --jq '.[] | "\(.bucket)\t\(.name)"' 2>/dev/null || true)
  pending=$(printf '%s\n' "$rows" | grep -c '^pending' || true)
  [ "$pending" = 0 ] && break
  [ "$(date +%s)" -ge "$end" ] && break
  sleep "$poll"
done
merge=$(gh pr view "$url" --json mergeable,mergeStateStatus --jq '"\(.mergeable) (\(.mergeStateStatus))"' 2>/dev/null || echo unknown)
{
  [ -n "$rows" ] && printf '%s\n' "$rows" || echo "no checks"
  echo "mergeable: $merge"
  [ "$pending" != 0 ] && echo "$pending check(s) still pending after $wait minutes"
} | tee "$r/ci.txt"
failed=$(printf '%s\n' "$rows" | grep -c '^fail' || true)
case "$merge" in CONFLICTING*) conflicting=1;; *) conflicting=0;; esac
[ "$failed" = 0 ] && [ "$conflicting" = 0 ] || { echo "ci: $failed check(s) failed, conflicting=$conflicting"; exit 1; }
