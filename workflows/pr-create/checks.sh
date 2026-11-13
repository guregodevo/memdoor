#!/usr/bin/env bash
# checks.sh <run dir> <commands>: runs each command (comma-separated) in the project and records the result:
# checks.tsv, one line `<command><TAB><exit status>`, and checks.txt, each command's last 15 lines of output
# under a `$ <command>` line, for the PR's test results. On a failure the lines that name it (fail, error or
# panic, 10 at most) come before the tail, so the log says which test, not only that one failed. A command that fails is recorded, not hidden, and
# does not stop the next. With no commands both files say nothing was run.
set -u
r="$1"; cmds="${2:-}"
: > "$r/checks.tsv"; : > "$r/checks.txt"
[ -n "$cmds" ] || { echo "no checks run (set the checks arg)" > "$r/checks.txt"; echo "checks: none asked for"; exit 0; }
IFS=',' read -r -a list <<< "$cmds"
for c in "${list[@]}"; do
  c=$(printf '%s' "$c" | sed -e 's/^ *//' -e 's/ *$//'); [ -n "$c" ] || continue
  out=$(bash -c "$c" 2>&1); s=$?
  printf '%s\t%s\n' "$c" "$s" >> "$r/checks.tsv"
  { printf '$ %s   (exit %s)\n' "$c" "$s"
    [ "$s" = 0 ] || printf '%s\n' "$out" | grep -iE 'fail|error|panic' | head -10
    printf '%s\n' "$out" | tail -15; printf '\n'; } >> "$r/checks.txt"
  echo "checks: $c -> exit $s"
done
