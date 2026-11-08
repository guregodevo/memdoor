#!/usr/bin/env bash
# cross.sh <run dir>: every attempt's check run on every other attempt's
# change, in a scratch worktree at the run's base (the project's tree is never
# touched while other steps work in it). An attempt scores one point per other
# attempt's check it passes; a check that passes on the tree as found proves
# nothing and is not counted. Writes <run dir>/pick-points.txt ("n points
# size", one line per attempt) and prints the matrix. Exit 0: settled, the pick
# is not a tie (one attempt has the most points, no check tells any apart, or
# every attempt passes every other's check);
# exit 1: a tie at the top on checks that discriminate, where another attempt
# can break it. The fan-out's `between` (direct.yaml), and the pick's input.
set -u
r="$1"; R=$(cd "$r" && pwd); here=$(pwd)
atts=$(ls "$R"/attempts/*.patch 2>/dev/null | while read -r p; do [ -s "$p" ] && basename "$p" .patch; done | sort -n | tr '\n' ' ')
: > "$R/pick-points.txt"; : > "$R/pick-matrix.txt"
[ -n "$atts" ] || { echo "no attempt changed the tree"; exit 0; }
for m in $atts; do eval "score_$m=0"; done
checks=0
t="$R/cross-tree"; base=$(cat "$R/base.txt" 2>/dev/null)
if [ -n "$base" ] && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  git worktree remove --force "$t" >/dev/null 2>&1; rm -rf "$t"
  git worktree add -q --detach "$t" "$base" >/dev/null 2>&1 || { echo "cross: no scratch tree at $base"; exit 0; }
  ln -s "$here/.memdoor" "$t/.memdoor"
  # The tree as the run found it: the base, and the untracked files it had.
  clean() {
    git -C "$t" checkout -q --force "$base" -- . 2>/dev/null
    git -C "$t" clean -fdq -e .memdoor >/dev/null 2>&1
    while IFS= read -r f; do
      [ -n "$f" ] && [ -e "$here/$f" ] && mkdir -p "$t/$(dirname "$f")" && cp -p "$here/$f" "$t/$f"
    done < "$R/untracked-before.txt" 2>/dev/null
  }
  for k in $atts; do
    [ -x "$R/attempt-$k" ] || continue
    clean
    if ( cd "$t" && "$R/attempt-$k" ) > "$R/attempt-$k-base.log" 2>&1; then
      printf 'check %s: passes on the tree as found: proves nothing\n' "$k" >> "$R/pick-matrix.txt"; continue
    fi
    checks=$((checks + 1)); eval "own_$k=1"
    for m in $atts; do
      [ "$m" = "$k" ] && continue
      clean
      git -C "$t" apply --binary "$R/attempts/$m.patch" 2>/dev/null || { printf 'check %s on %s: the patch did not apply\n' "$k" "$m" >> "$R/pick-matrix.txt"; continue; }
      if ( cd "$t" && "$R/attempt-$k" ) > "$R/attempt-$k-on-$m.log" 2>&1; then
        eval "n=\$score_$m"; eval "score_$m=$((n + 1))"; printf 'check %s on %s: passes\n' "$k" "$m" >> "$R/pick-matrix.txt"
      else
        printf 'check %s on %s: FAILS\n' "$k" "$m" >> "$R/pick-matrix.txt"
      fi
    done
  done
  git worktree remove --force "$t" >/dev/null 2>&1; rm -rf "$t"
fi
top=-1; at_top=0; unanimous=yes
for m in $atts; do eval "own_$m=\${own_$m:-0}"; done
for m in $atts; do
  eval "sc=\$score_$m"; size=$(wc -c < "$R/attempts/$m.patch" | tr -d ' ')
  printf '%s %s %s\n' "$m" "$sc" "$size" >> "$R/pick-points.txt"
  if [ "$sc" -gt "$top" ]; then top=$sc; at_top=1; elif [ "$sc" -eq "$top" ]; then at_top=$((at_top + 1)); fi
  eval "o=\$own_$m"; [ "$sc" -eq $((checks - o)) ] || unanimous=no
done
cat "$R/pick-matrix.txt"
summary=$(awk '{printf "%s%s %s", (NR>1?", ":""), $1, $2}' "$R/pick-points.txt")
printf 'attempts: %s; checks that fail on the tree as found: %s; points: %s\n' "$(echo $atts | wc -w | tr -d ' ')" "$checks" "$summary"
# Settled: no check tells any apart, one attempt leads, or every attempt
# passes every other's check (they agree; another attempt cannot break a tie
# that is unanimous: the ledger smoke ran five where three agreed).
if [ "$checks" -eq 0 ] || [ "$at_top" -eq 1 ]; then echo "settled"; exit 0; fi
if [ "$unanimous" = yes ]; then echo "settled: every attempt passes every other attempt's check"; exit 0; fi
echo "not settled: $at_top attempts tie at $top points"; exit 1
