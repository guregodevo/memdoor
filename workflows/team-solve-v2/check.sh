#!/usr/bin/env bash
# check.sh <run dir> <log>: the task's check, as the Check section of task.md
# names it. Exit 0 it passed, 1 it failed, 3 nothing to run (the judge
# decides). Writes the kind it ran to <run dir>/check-kind.txt.
set -u
r="$1"; log="$2"; w="$(cd "$(dirname "$0")" && pwd)"
section=$(sed -n '/^## Check/,/^## /p' "$r/task.md" | grep -v '^## ' | grep -v '^<!--')
cmd=$(printf '%s\n' "$section" | grep -o '`[^`]*`' | head -1 | tr -d '`')
case "$cmd" in samples|judge) cmd="" ;; esac
if [ -n "$cmd" ]; then
  echo command > "$r/check-kind.txt"; printf 'check: %s\n' "$cmd" > "$log"
  bash -c "$cmd" >> "$log" 2>&1 && exit 0 || exit 1
elif ls tests/*.in > /dev/null 2>&1 && ! printf '%s\n' "$section" | grep -qw judge; then
  echo samples > "$r/check-kind.txt"; printf 'check: samples\n' > "$log"
  bash "$w/check-samples.sh" >> "$log" 2>&1 && exit 0 || exit 1
fi
echo judge > "$r/check-kind.txt"; printf 'check: nothing to run, the judge decides\n' > "$log"
exit 3
