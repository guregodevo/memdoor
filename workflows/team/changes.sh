#!/usr/bin/env bash
# changes.sh <run dir>: what the run changed in this tree, into <run dir>/changes.diff.
# In git: the diff against the snapshot taken when the run started (base.txt),
# plus the files that appeared since. Elsewhere: every file newer than the
# start marker, its content capped. .memdoor/ is never part of it.
set -u
r="$1"; out="$r/changes.diff"; : > "$out"
if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && [ -s "$r/base.txt" ]; then
  git diff "$(cat "$r/base.txt")" -- . ':(exclude).memdoor' >> "$out"
  git ls-files --others --exclude-standard -- . ':(exclude).memdoor' | while IFS= read -r f; do
    grep -qxF -- "$f" "$r/untracked-before.txt" 2>/dev/null && continue
    git diff --no-index -- /dev/null "$f" >> "$out"
  done
else
  find . -type f -newer "$r/start" -not -path './.memdoor/*' -not -path './.git/*' | sort | while IFS= read -r f; do
    printf '=== %s\n' "$f" >> "$out"; head -c 20000 "$f" >> "$out"; printf '\n' >> "$out"
  done
fi
printf 'changes: %s lines in %s\n' "$(wc -l < "$out" | tr -d ' ')" "$out"
