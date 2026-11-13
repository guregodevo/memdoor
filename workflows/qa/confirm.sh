#!/usr/bin/env bash
# confirm.sh <run dir>: runs each bug the explorer listed (<run dir>/candidates.txt, one slug a line)
# twice, from the project's root. A bug is confirmed when repro.sh exits 1 and prints a line starting
# `BUG: ` both times, leaves the project's files as they were, and its expectation has a source: source.txt is the
# path of a file of the project on line 1 and, on line 2, a line quoted from it verbatim (a doc's sentence, a test's
# assertion). A quote the file does not hold drops the bug: an expectation nobody wrote down is a guess. Written: confirmed.txt (the slugs),
# confirmed.md (each one's title, expectation and first BUG: line, what dedupe reads), confirm.log (every bug, kept or dropped,
# with the reason). The scripts' exit codes are compared, nothing about the project's language.
# A repro has no clock of its own here: the step's timeout bounds the whole run.
set -u
r="$1"
: > "$r/confirmed.txt"; : > "$r/confirmed.md"; : > "$r/confirm.log"
tree() { git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git status --porcelain -- . ':(exclude).memdoor' 2>/dev/null; }
drop() { echo "$1: dropped, $2" | tee -a "$r/confirm.log"; }
while IFS= read -r slug || [ -n "$slug" ]; do
  slug=$(printf '%s' "$slug" | tr -d '[:space:]')
  [ -n "$slug" ] || continue
  case "$slug" in *[!a-z0-9-]*|-*) drop "$slug" "not a slug (lowercase letters, digits, dashes)"; continue;; esac
  d="$r/bugs/$slug"
  [ -f "$d/repro.sh" ] && [ -s "$d/title.txt" ] && [ -s "$d/expected.md" ] && [ -s "$d/source.txt" ] || { drop "$slug" "repro.sh, title.txt, expected.md or source.txt is missing"; continue; }
  src=$(sed -n 1p "$d/source.txt" | tr -d '\r'); quote=$(sed -n 2p "$d/source.txt" | tr -d '\r')
  case "$src" in /*|*..*) drop "$slug" "source $src is not a path inside the project"; continue;; esac
  [ -f "$src" ] || { drop "$slug" "source $src is not a file of the project"; continue; }
  [ -n "$quote" ] && grep -qF -- "$quote" "$src" || { drop "$slug" "the quote is not in $src"; continue; }
  before=$(tree); ok=1
  for run in 1 2; do
    bash "$d/repro.sh" > "$d/run-$run.log" 2>&1; code=$?
    if [ "$code" != 1 ]; then drop "$slug" "run $run exited $code, not 1"; ok=0; break; fi
    grep -q '^BUG: ' "$d/run-$run.log" || { drop "$slug" "run $run printed no BUG: line"; ok=0; break; }
  done
  [ "$ok" = 1 ] || continue
  [ "$(tree)" = "$before" ] || { drop "$slug" "repro.sh changed the project's files"; continue; }
  echo "$slug" >> "$r/confirmed.txt"
  { printf '## %s\n\n%s\n\nExpected: %s\nGot: %s\n\n' "$slug" "$(head -1 "$d/title.txt")" "$(head -5 "$d/expected.md" | tr '\n' ' ')" "$(grep -m1 '^BUG: ' "$d/run-1.log")"; } >> "$r/confirmed.md"
  echo "$slug: confirmed twice ($(grep -m1 '^BUG: ' "$d/run-1.log"))" | tee -a "$r/confirm.log"
done < "$r/candidates.txt"
echo "confirm: $(wc -l < "$r/confirmed.txt" | tr -d ' ') of $(grep -c . "$r/candidates.txt" | tr -d ' ') confirmed"
