#!/usr/bin/env bash
# done.sh <step> <run dir>: whether an agent step did its part, the step's
# target. Exit 0 when it did; otherwise exit 1 and say what is missing. The
# problem step copies it into the run's folder; a task cannot edit its own
# target (gateway/target_guard.go).
set -u
step="$1"; r="$2"; miss=0
w=.memdoor/workflows/team-solve; [ -d "$w" ] || w="$HOME/.memdoor/workflows/team-solve"
no() { printf 'not done: %s\n' "$1"; miss=1; }
# section <file> <heading>: the lines under "## <heading>", comments and blanks dropped.
section() { sed -n "/^## $2\$/,/^## /p" "$1" | sed '1d;/^## /d;/^<!--.*-->$/d;/^[[:space:]]*$/d'; }
verdict() { [ -s "$1" ] && head -1 "$1" | grep -qE '^verdict: (PASS|FAIL)$'; }

case "$step" in
plan)
  t="$r/task.md"
  for s in Deliverable Check Scope "Done when"; do
    [ -n "$(section "$t" "$s")" ] || no "task.md: the $s section is not filled in"
  done
  { section "$t" Check | grep -q '`[^`]*`' || section "$t" Check | grep -qwE 'samples|judge|reproduce'; } \
    || no 'task.md: Check names no command in backticks, nor samples, nor judge, nor reproduce'
  section "$t" Scope | grep -q '`[^`]*`' || no 'task.md: Scope names no file or directory in backticks'
  first=$(grep -m1 '[^[:space:]]' "$r/prompt.md")
  grep -qF -- "$first" "$t" || no 'task.md: the Prompt section was changed; put it back as prompt.md has it'
  grep -q '[^[:space:]]' "$r/plan.md" 2>/dev/null || no 'plan.md is missing or empty'
  [ -f "$r/notes.md" ] || no 'notes.md is missing' ;;
direct)
  # An attempt runs in a worktree of its own whose HEAD is the tree as the
  # run found it (direct.yaml, isolate): what is uncommitted is its change.
  # Not changes.sh: the attempts run at once and would share its file.
  [ -n "$(git status --porcelain -- . ':(exclude).memdoor' 2>/dev/null)" ] || no 'nothing in the project changed yet: do the task as the Deliverable says' ;;
solve)
  bash "$w/changes.sh" "$r" > /dev/null
  [ -s "$r/changes.diff" ] || no 'nothing in the project changed yet: do the task as the Deliverable says' ;;
paths)
  # Each path check must fail on the tree as the run found it (the attempts
  # have not touched it: they run in worktrees of their own); one that passes
  # proves nothing about its path and is set aside, not counted.
  R=$(cd "$r" && pwd)
  [ -s "$R/paths.md" ] || no "$r/paths.md is missing: one line per check (its name, the code path, what it asserts), or why there is none"
  if [ -d "$R/paths" ]; then
    for c in "$R"/paths/*; do
      [ -f "$c" ] || continue
      if [ ! -x "$c" ]; then no "$c is not executable (chmod +x)"; continue; fi
      mkdir -p "$R/paths-logs"
      if "$c" > "$R/paths-logs/$(basename "$c").baseline.log" 2>&1; then
        mkdir -p "$R/paths-rejected"; mv "$c" "$R/paths-rejected/" 2>/dev/null
        printf '\n%s passed on the tree as the run found it: set aside, it proves nothing (paths-rejected/)\n' "$(basename "$c")" >> "$R/paths.md"
      fi
    done
  fi ;;
reproduce)
  # The reproduction is proven by the tree as the run found it: run there
  # (the clean copy baseline.yaml left), it must fail. One that passes
  # reproduces nothing. Only its exit code is read: the language and the
  # tooling are the project's, never named here.
  R=$(cd "$r" && pwd); t="$R/baseline-tree"
  [ -x "$R/repro" ] || no "$r/repro is missing or not executable: one executable (chmod +x) that exits non-zero while the behaviour the Prompt describes is present, and 0 once the project behaves as the Prompt expects"
  for p in "$(pwd)" "$(pwd -P)"; do
    [ -x "$R/repro" ] && grep -qF -- "$p" "$R/repro" && { no "repro names this tree by its absolute path ($p); use paths relative to the project root: it is run from the root of a clean copy too"; break; }
  done
  if [ "$miss" = 0 ]; then
    [ -d "$t" ] || t=$(pwd)   # no clean copy (not in git): the tree as it is
    ( cd "$t" && "$R/repro" ) > "$R/repro-baseline.log" 2>&1; s=$?
    printf '%s\n' "$s" > "$R/repro-baseline-status.txt"
    [ "$s" != 0 ] || no "repro exits 0 on the tree as the run found it ($t): it reproduces nothing. It must fail while the behaviour the Prompt describes is present ($r/repro-baseline.log)"
  fi ;;
review)
  verdict "$r/review.md" || no 'review.md: its first line must be `verdict: PASS` or `verdict: FAIL`' ;;
repair)
  head -1 "$r/review.md" 2>/dev/null | grep -qx 'verdict: PASS' && exit 0
  grep -q '[^[:space:]]' "$r/repair.md" 2>/dev/null || no 'repair.md is missing: what you changed and what the Check printed' ;;
judge)
  verdict "$r/verdict.md" || no 'verdict.md: its first line must be `verdict: PASS` or `verdict: FAIL`' ;;
*) echo "done.sh: unknown step $step"; exit 2 ;;
esac
[ "$miss" = 0 ] && printf '%s: done\n' "$step"
exit "$miss"
