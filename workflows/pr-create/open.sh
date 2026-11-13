#!/usr/bin/env bash
# open.sh <run dir> <draft|now>: opens the pull request from title.txt and body.md. draft prints what it would
# do and touches nothing. now pushes the current branch to origin and runs gh pr create against the default
# branch. It refuses on the default branch and never merges: the maintainer does.
set -u
r="$1"; mode="${2:-draft}"
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null) || { echo "not a git repository"; exit 1; }
base=$(git symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##'); base=${base:-main}
[ "$branch" != "$base" ] && [ "$branch" != HEAD ] || { echo "on $branch: a pull request needs its own branch"; exit 1; }
[ "$(cat "$r/sync.status" 2>/dev/null)" != conflict ] || { echo "the branch conflicts with origin/$base (see $r/sync.txt): not opening it"; exit 1; }
if [ "$mode" != now ]; then
  echo "draft: would push $branch and open a pull request against $base titled: $(cat "$r/title.txt")"
  echo "the body is $r/body.md; set open: now to open it"; exit 0
fi
command -v gh >/dev/null && gh auth status >/dev/null 2>&1 || { echo "gh is not installed or not signed in (gh auth login, or GH_TOKEN)"; exit 1; }
git push -q -u origin "$branch" || { echo "push failed"; exit 1; }
url=$(gh pr create --base "$base" --head "$branch" --title "$(cat "$r/title.txt")" --body-file "$r/body.md") || { echo "gh pr create failed"; exit 1; }
printf '%s\n' "$url" | tee "$r/url.txt"
