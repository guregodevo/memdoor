#!/usr/bin/env bash
# settle.sh <run dir> <max bugs>: from confirmed.txt, drop the bugs dedupe named as an open issue's (dups.txt
# lines `<slug> #<number>`; only a confirmed slug with a number from open.txt counts, so an invented one is
# ignored), keep the first <max bugs>, and write for each <run dir>/bugs/<slug>/issue.md. final.txt lists the
# slugs; summary.txt is the run's account.
#
# An issue body is data, under four fixed headings:
#   Context          the commit, the OS and the date, read here, and the steps: repro.sh verbatim
#   Expected results the explorer's sentence and the quote from the project's file that confirm.sh verified
#   Actual results   the first run's output and the two runs' exit codes, verbatim
#   Proposal         the explorer's suggestion (bugs/<slug>/proposal.md), only when it wrote one
# Only the title, the expected sentence and the proposal are the explorer's words.
set -u
r="$1"; max="$2"
: > "$r/final.txt"; : > "$r/dropped.txt"
commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
os=$(uname -sm 2>/dev/null || echo unknown)
day=$(date -u +%Y-%m-%d)
issue() {
  local d="$1" src quote
  src=$(sed -n 1p "$d/source.txt"); quote=$(sed -n 2p "$d/source.txt")
  {
    printf '**Context**\n\nCommit %s, %s, %s. Found by the `qa` workflow. The steps ran twice from the project root with the same result before this was written:\n\n```bash\n%s\n```\n\n' "$commit" "$os" "$day" "$(cat "$d/repro.sh")"
    printf '**Expected results**\n\n%s\n\n`%s` says:\n\n> %s\n\n' "$(cat "$d/expected.md")" "$src" "$quote"
    printf '**Actual results**\n\nExit 1 both times. Output of the first run:\n\n```\n%s\n```\n' "$(head -40 "$d/run-1.log")"
    [ -s "$d/proposal.md" ] && printf '\n**Proposal**\n\n%s\n' "$(cat "$d/proposal.md")"
  } > "$d/issue.md"
}
n=0; dup=0; all=0
while IFS= read -r slug || [ -n "$slug" ]; do
  [ -n "$slug" ] || continue
  all=$((all + 1))
  num=$(awk -v s="$slug" '$1 == s && $2 ~ /^#[0-9]+$/ { print substr($2, 2); exit }' "$r/dups.txt" 2>/dev/null)
  if [ -n "$num" ] && grep -q "^#$num " "$r/open.txt" 2>/dev/null; then
    echo "$slug: the open issue #$num" >> "$r/dropped.txt"; dup=$((dup + 1)); continue
  fi
  if [ "$n" -ge "$max" ]; then echo "$slug: over the cap of $max" >> "$r/dropped.txt"; continue; fi
  issue "$r/bugs/$slug"
  echo "$slug" >> "$r/final.txt"; n=$((n + 1))
done < "$r/confirmed.txt"
{
  echo "qa: $all confirmed, $dup already an open issue, $n to file"
  while IFS= read -r slug; do [ -n "$slug" ] && echo "  $slug: $(head -1 "$r/bugs/$slug/title.txt")  [$r/bugs/$slug/issue.md]"; done < "$r/final.txt"
  [ -s "$r/dropped.txt" ] && sed 's/^/  dropped /' "$r/dropped.txt"
} | tee "$r/summary.txt"
