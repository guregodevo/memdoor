#!/usr/bin/env bash
# scope.sh <run dir>: each file the run changed, with the lines it added and
# removed, against task.md's Scope, into <run dir>/scope.txt. The Scope names
# each file or directory the work may touch in backticks; `.` is the whole
# tree. Exit 1 when a changed file is outside it: a free model's run kept a
# "lesson" in AGENTS.md and a review edited the code it reviewed (2026-11-05).
set -u
r="$1"; out="$r/scope.txt"; : > "$out"
scope=$(sed -n '/^## Scope$/,/^## /p' "$r/task.md" | sed '1d;/^## /d;/^<!--.*-->$/d' \
  | grep -o '`[^`]*`' | tr -d '`' | sed 's#^\./##; s#/*$##; s#^$#.#')

# changed: "<added> <removed> <path>" per file the run changed.
changed() {
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && [ -s "$r/base.txt" ]; then
    git diff --numstat "$(cat "$r/base.txt")" -- . ':(exclude).memdoor' ':(exclude,glob)**/__pycache__/**' \
      | awk -F'\t' '{print $1, $2, $3}'
    git ls-files --others --exclude-standard -- . ':(exclude).memdoor' ':(exclude,glob)**/__pycache__/**' | while IFS= read -r f; do
      grep -qxF -- "$f" "$r/untracked-before.txt" 2>/dev/null && continue
      printf '%s 0 %s\n' "$(wc -l < "$f" | tr -d ' ')" "$f"
    done
  else
    find . -type f -newer "$r/start" -not -path './.memdoor/*' -not -path './.git/*' -not -path '*/__pycache__/*' \
      | sed 's#^\./##' | sort | while IFS= read -r f; do printf '? ? %s\n' "$f"; done
  fi
}

inside() {
  [ -z "$scope" ] && return 0
  while IFS= read -r s; do
    [ "$s" = . ] || [ "$1" = "$s" ] || case "$1" in "$s"/*) ;; *) false ;; esac && return 0
  done <<< "$scope"
  return 1
}

fail=0
# HEAD WHERE THE RUN STARTED (problem.yaml's head.txt). A direct attempt
# reviewing a commit ran `git checkout HEAD~1` and left the tree on the code
# before the commit (2026-11-05): put it back first, then measure.
if [ -s "$r/head.txt" ] && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  want=$(cat "$r/head.txt")
  now=$(git symbolic-ref -q --short HEAD || git rev-parse -q --verify HEAD)
  if [ "$now" != "$want" ] && [ "$(git rev-parse -q --verify "$now")" != "$(git rev-parse -q --verify "$want")" ]; then
    if git checkout -q "$want" 2>/dev/null; then
      echo "HEAD had moved to $now; put back on $want" >> "$out"
    else
      echo "HEAD MOVED to $now and could not be put back on $want" >> "$out"; fail=1
    fi
  fi
fi
if [ -z "$scope" ]; then
  echo 'scope: none named in backticks in task.md, so not checked' >> "$out"
else
  printf 'scope: %s\n' "$(printf '%s' "$scope" | tr '\n' ' ')" >> "$out"
fi
# A NEW file under the directory of a file the Scope names is inside it: a
# test needs its fixture beside it, and its name is not known in advance (a
# correct SWE-bench patch added tests/utils_tests/test_module/__main__.py next
# to the scoped tests/utils_tests/test_autoreload.py and was failed, 2026-11-05).
# An edit to an existing file outside the Scope still fails.
beside() {
  [ -s "$r/base.txt" ] && git cat-file -e "$(cat "$r/base.txt"):$1" 2>/dev/null && return 1
  while IFS= read -r s; do
    [ -f "$s" ] || [ -f "$(git rev-parse --show-toplevel 2>/dev/null)/$s" ] || continue
    d=$(dirname "$s"); [ "$d" = . ] && continue
    case "$1" in "$d"/*) return 0 ;; esac
  done <<< "$scope"
  return 1
}
while read -r add del f; do
  [ -n "${f:-}" ] || continue
  if inside "$f" || beside "$f"; then
    printf 'changed: %s +%s -%s\n' "$f" "$add" "$del" >> "$out"
  else
    printf 'OUTSIDE THE SCOPE: %s +%s -%s (undo it: git checkout the file, or delete it when the run made it)\n' "$f" "$add" "$del" >> "$out"; fail=1
  fi
done < <(changed)
cat "$out"
exit "$fail"
