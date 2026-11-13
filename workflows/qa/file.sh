#!/usr/bin/env bash
# file.sh <run dir> <labels> <qa label> <max open>: files each slug of final.txt as an issue with gh, its
# issue.md the body. Labels are the comma-separated ones asked for plus the qa label, and only those the
# repository has (gh label list). With <max open> issues of the qa label already open, nothing is filed.
# The issues' URLs go to filed.txt.
set -u
r="$1"; labels="$2"; qa="$3"; max="$4"
: > "$r/filed.txt"
[ -s "$r/final.txt" ] || { echo "file: nothing to file"; exit 0; }
command -v gh >/dev/null || { echo "gh is not installed: the issues are in $r/bugs/*/issue.md"; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "gh is not signed in (gh auth login, or GH_TOKEN): the issues are in $r/bugs/*/issue.md"; exit 1; }
open=$(gh issue list --state open --label "$qa" --limit 100 --json number --jq length 2>/dev/null || echo 0)
if [ "$open" -ge "$max" ]; then echo "cap: $open issues labelled $qa are open, the most is $max; nothing filed"; exit 0; fi
have=$(gh label list --limit 200 --json name --jq '.[].name' 2>/dev/null)
args=()
for l in $(printf '%s' "$labels" | tr ',' ' ') "$qa"; do
  [ -n "$l" ] && printf '%s\n' "$have" | grep -qxF -- "$l" && args+=(--label "$l")
done
while IFS= read -r slug; do
  [ -n "$slug" ] || continue
  [ "$open" -lt "$max" ] || { echo "cap reached: $slug left unfiled"; continue; }
  url=$(gh issue create --title "$(head -1 "$r/bugs/$slug/title.txt")" --body-file "$r/bugs/$slug/issue.md" ${args[@]+"${args[@]}"}) \
    || { echo "gh issue create failed for $slug"; continue; }
  echo "$url" | tee -a "$r/filed.txt"; open=$((open + 1))
done < "$r/final.txt"
