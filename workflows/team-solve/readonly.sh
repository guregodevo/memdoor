#!/usr/bin/env bash
# readonly.sh <run dir>: the Check again, with the change's edits to tests that
# were there before put back as they were. A change can pass the Check because
# it rewrote the test that pinned the old behaviour: 75 of 82 wrong attempts on
# SWE-bench Verified edited a test file that existed, and the replay of them
# against the original files failed a third (2026-11-08). The tests that fail
# here, and pass both on the tree as the run found it and in the Check on the
# change itself, go to <run dir>/readonly.txt, one per line; the evidence for
# readonly-justify to <run dir>/readonly.md. Neither is written to when the
# change edits no test that was there before. Always exits 0: a step of the
# DAG decides what a broken test means.
set -u
r="$1"; w="$(cd "$(dirname "$0")" && pwd)"
R="$(cd "$r" && pwd)"
: > "$R/readonly.txt"; rm -f "$R/readonly.md"
git rev-parse --is-inside-work-tree >/dev/null 2>&1 && [ -s "$R/base.txt" ] || { echo "readonly: not in git, nothing to replay"; exit 0; }
base=$(cat "$R/base.txt")
# A test file by its path, the same rule in every language: a directory named
# test, tests, testing, __tests__, spec or specs, or a file named test_*,
# tests_*, *_test, *_tests or *.test.*.
is_test() {
  printf '%s\n' "$1" | awk -F/ '{
    for (i = 1; i < NF; i++) if ($i ~ /^(test|tests|testing|__tests__|spec|specs)$/) { found = 1 }
    n = $NF; stem = n; sub(/\.[^.]*$/, "", stem)
    if (stem ~ /^tests?_/ || stem ~ /_tests?$/ || n ~ /\.test\./) found = 1
    exit !found
  }'
}
edited=()
while IFS= read -r f; do
  [ -n "$f" ] && is_test "$f" && git cat-file -e "$base:$f" 2>/dev/null && edited+=("$f")
done < <(git diff --name-only "$base" -- . ':(exclude).memdoor')
if [ ${#edited[@]} = 0 ]; then echo "readonly: the change edits no test that was there before"; exit 0; fi
t="$R/readonly-tree"
git worktree remove --force "$t" 2>/dev/null; rm -rf "$t"
git worktree add -q --detach "$t" "$base" 2>/dev/null || { echo "readonly: no clean copy, nothing replayed"; exit 0; }
trap 'git worktree remove --force "$t" 2>/dev/null; rm -rf "$t"' EXIT
excl=(':(exclude).memdoor' ':(exclude,glob)**/__pycache__/**')
for f in "${edited[@]}"; do excl+=(":(exclude)$f"); done
rest=$(git diff --binary "$base" -- . "${excl[@]}"; printf x); rest=${rest%x}
if [ -n "$rest" ]; then
  printf '%s' "$rest" | ( cd "$t" && git apply --binary --whitespace=nowarn - ) || { echo "readonly: the change does not apply without its test edits, nothing replayed"; exit 0; }
fi
git ls-files --others --exclude-standard -- . ':(exclude).memdoor' ':(exclude,glob)**/__pycache__/**' | while IFS= read -r f; do
  grep -qxF -- "$f" "$R/untracked-before.txt" 2>/dev/null && continue
  mkdir -p "$t/$(dirname "$f")" && cp -p "$f" "$t/$f"
done
[ -d .memdoor ] && ln -s "$(pwd)/.memdoor" "$t/.memdoor" 2>/dev/null
# check.sh records its kind and parts for the steps after test: those stay the test step's.
for k in check-kind.txt check-parts.txt; do [ -f "$R/$k" ] && cp "$R/$k" "$R/$k.keep"; done
( cd "$t" && bash "$w/check.sh" "$R" "$R/readonly-check.log" > /dev/null 2>&1 ); status=$?
for k in check-kind.txt check-parts.txt; do [ -f "$R/$k.keep" ] && mv "$R/$k.keep" "$R/$k"; done
if [ "$status" = 0 ] || [ "$status" = 3 ]; then
  echo "readonly: the Check passes with the ${#edited[@]} edited test file(s) as they were: ${edited[*]}"; exit 0
fi
listed() { bash "$w/failures.sh" "$1" 2>/dev/null; }
if after=$(bash "$w/failures.sh" "$R/readonly-check.log"); then
  # C collation for comm only: the Check itself runs in the run's own locale.
  LC_ALL=C comm -23 <(printf '%s\n' "$after" | LC_ALL=C sort -u) <( { listed "$R/test-output.log"; listed "$R/baseline-check.log"; } | LC_ALL=C sort -u) | grep -v '^$' > "$R/readonly.txt"
  [ -s "$R/readonly.txt" ] || { echo "readonly: no test fails here that passes in the Check on the change"; exit 0; }
else
  # A log that lists no tests: what failed is in its tail, for readonly-justify to read.
  echo "(the runner's log lists no tests: see its tail below)" > "$R/readonly.txt"
fi
{
  printf '# Tests the change edited, run as they were\n\n'
  printf 'The change edited these test files, which were there before: %s.\n' "${edited[*]}"
  printf 'With those files as they were and the rest of the change in place, the Check fails. These tests fail there, and pass both on the tree as the run found it and in the Check on the change:\n\n'
  sed 's/^/- /' "$R/readonly.txt"
  printf '\n## What the Check printed (its tail)\n\n```\n'
  tail -c 6000 "$R/readonly-check.log"
  printf '\n```\n\n## How the change edited those test files\n\n```diff\n'
  git diff "$base" -- "${edited[@]}" | head -c 8000
  printf '\n```\n'
} > "$R/readonly.md"
echo "readonly: $(wc -l < "$R/readonly.txt" | tr -d ' ') test(s) fail with the edited tests as they were: readonly.md"
exit 0
