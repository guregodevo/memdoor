#!/usr/bin/env bash
# check.sh <run dir> <log>: the task's check, as the Check section of task.md
# names it. Exit 0 it passed (or, when it fails on the tree as found, the
# change adds no failing test), 1 it failed, 3 nothing to run or nothing it
# says counts (the judge decides). Writes the kind it ran to
# <run dir>/check-kind.txt (command, samples, judge; with "+repro", or
# "repro" alone, when the reproduction ran too) and each part's status to
# <run dir>/check-parts.txt ("command 0", "repro 1").
set -u
r="$1"; log="$2"; w="$(cd "$(dirname "$0")" && pwd)"
R="$(cd "$r" && pwd)"
section=$(sed -n '/^## Check/,/^## /p' "$r/task.md" | grep -v '^## ' | grep -v '^<!--')
# A Check that starts with the word samples or judge is that, whatever it
# quotes after; otherwise its first command in backticks.
first=$(printf '%s\n' "$section" | grep -m1 '[^[:space:]]' | tr -cs 'A-Za-z' ' ' | awk '{print tolower($1)}')
cmd=$(printf '%s\n' "$section" | grep -o '`[^`]*`' | head -1 | tr -d '`')
case "$cmd" in samples|judge|reproduce) cmd="" ;; esac
case "$first" in samples|judge) cmd="" ;; esac
# The reproduction (reproduce.yaml): when the Check names it, one was
# written, and it fails on the tree as the run found it (done.sh reproduce),
# it runs beside the command. A Check the tree passed before any change
# proves no fix: 87 of 107 wrong attempts on SWE-bench Verified had passed
# theirs (2026-11-06, #150).
repro=""
if printf '%s\n' "$section" | sed 's/`[^`]*`//g' | grep -qw reproduce; then
  if [ -x "$R/repro" ] && [ -s "$R/repro-baseline-status.txt" ] && [ "$(cat "$R/repro-baseline-status.txt")" != 0 ]; then
    repro="$R/repro"
  fi
fi
: > "$R/check-parts.txt"
part() { printf '%s %s\n' "$1" "$2" >> "$R/check-parts.txt"; }
kind=judge; status=3
if [ -n "$cmd" ]; then
  kind=command; printf 'check: %s\n' "$cmd" > "$log"
  if bash -c "$cmd" >> "$log" 2>&1; then
    status=0
  # A Check that fails on the tree as the run found it (baseline.yaml) counts
  # what the work adds: it passes when no test fails that did not fail there,
  # as the runner lists them (failures.sh). One test already failing turned
  # the whole Check off (astropy-13033, 2026-11-05, #109).
  elif [ "$(cat "$r/baseline-status.txt" 2>/dev/null)" = 1 ] &&
     before=$(bash "$w/failures.sh" "$r/baseline-check.log") && after=$(bash "$w/failures.sh" "$log"); then
    added=$(comm -13 <(printf '%s\n' "$before" | LC_ALL=C sort -u) <(printf '%s\n' "$after" | LC_ALL=C sort -u) | grep -v '^$')
    if [ -z "$added" ]; then
      printf '\nthe Check fails on the tree as the run found it too (baseline-check.log), and this change adds no failing test to it: it passes\n' >> "$log"
      status=0
    else
      printf '\nthe tests this change makes fail (they pass on the tree as the run found it, baseline-check.log):\n%s\n' "$added" >> "$log"
      status=1
    fi
  # A broken Check whose log lists no tests: nothing it says counts, the judge decides.
  elif [ "$(cat "$r/baseline-status.txt" 2>/dev/null)" = 1 ]; then
    printf '\nthe Check also fails on the tree as the run found it (baseline-check.log): nothing it says counts, the judge decides\n' >> "$log"
    kind=judge; status=3
  else
    status=1
  fi
  part command "$status"
elif [ "$first" != judge ] && ls tests/*.in > /dev/null 2>&1 && ! printf '%s\n' "$section" | grep -qw judge; then
  kind=samples; printf 'check: samples\n' > "$log"
  if bash "$w/check-samples.sh" >> "$log" 2>&1; then status=0; else status=1; fi
  part samples "$status"
else
  printf 'check: nothing to run, the judge decides\n' > "$log"
fi
if [ -n "$repro" ]; then
  printf '\nreproduction: %s (it fails on the tree as the run found it)\n' "$repro" >> "$log"
  "$repro" >> "$log" 2>&1; rs=$?
  if [ "$rs" = 0 ]; then
    printf 'reproduction: passes (exit 0): the behaviour the Prompt describes is gone\n' >> "$log"
  else
    printf 'reproduction: FAILS (exit %s): the behaviour the Prompt describes is still there\n' "$rs" >> "$log"
    rs=1
  fi
  part repro "$rs"
  if [ "$kind" = judge ]; then kind=repro; else kind="$kind+repro"; fi
  if [ "$rs" != 0 ]; then status=1; elif [ "$status" = 3 ]; then status=0; fi
elif printf '%s\n' "$section" | sed 's/`[^`]*`//g' | grep -qw reproduce; then
  printf '\nno reproduction: none was written that fails on the tree as the run found it; the Check is %s\n' "$kind" >> "$log"
fi
echo "$kind" > "$R/check-kind.txt"
exit "$status"
