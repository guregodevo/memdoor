#!/usr/bin/env bash
# snapshot.sh save|restore <run dir> <name>: the tree's change since the run
# started, as one patch (<run dir>/<name>.patch, new files included), or the
# tree put back to the start and that patch applied. The net (keep.yaml): the
# team only replaces the direct attempt with work that passes a check the
# attempt did not; otherwise the attempt is put back.
set -u
mode="$1"; r="$2"; name="$3"; p="$r/$name.patch"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 && [ -s "$r/base.txt" ] || { echo "snapshot: not in git, nothing kept"; exit 0; }
base=$(cat "$r/base.txt")
new_files() {
  git ls-files --others --exclude-standard -- . ':(exclude).memdoor' ':(exclude,glob)**/__pycache__/**' | while IFS= read -r f; do
    grep -qxF -- "$f" "$r/untracked-before.txt" 2>/dev/null || printf '%s\n' "$f"
  done
}
case "$mode" in
save)
  { git diff --binary "$base" -- . ':(exclude).memdoor' ':(exclude,glob)**/__pycache__/**'
    new_files | while IFS= read -r f; do git diff --binary --no-index -- /dev/null "$f"; done
  } > "$p"
  echo "snapshot $name: $(grep -c '^diff --git' "$p") files in $p" ;;
restore)
  [ -f "$p" ] || { echo "snapshot $name: none saved"; exit 1; }
  git diff --name-only "$base" -- . ':(exclude).memdoor' | while IFS= read -r f; do
    git checkout -q "$base" -- "$f" 2>/dev/null || rm -f -- "$f"
  done
  new_files | while IFS= read -r f; do rm -f -- "$f"; done
  if [ -s "$p" ]; then git apply --binary "$p" || { echo "snapshot $name: the patch did not apply"; exit 1; }; fi
  echo "snapshot $name: put back ($(grep -c '^diff --git' "$p") files)" ;;
esac
