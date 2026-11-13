#!/usr/bin/env bash
# sync.sh <run dir>: whether the branch merges into the default branch without conflicts, checked before
# anything else runs. Fetches origin's default branch, then asks git to merge the two in memory
# (git merge-tree; a scratch worktree merge on a git that lacks it) and touches nothing of the project.
# Writes sync.txt (the account) and sync.status (clean or conflict). A conflict lists the files and exits 1:
# no PR is worth opening, nor its checks worth running, until the branch is rebased. Behind the base is
# reported and is not a failure.
set -u
r="$1"
base=$(git symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##'); base=${base:-main}
if git fetch -q origin "$base" 2>/dev/null; then fetched="fetched origin/$base"; else fetched="could not fetch: compared with the last origin/$base seen"; fi
git rev-parse --verify -q "origin/$base" >/dev/null || { echo "no origin/$base to compare with"; exit 1; }
behind=$(git rev-list --count "HEAD..origin/$base"); ahead=$(git rev-list --count "origin/$base..HEAD")
files=""; conflict=0
if out=$(git merge-tree --write-tree --name-only "origin/$base" HEAD 2>&1); then :; else
  s=$?
  if [ "$s" = 1 ]; then conflict=1; files=$(printf '%s\n' "$out" | sed -n '2,/^$/p' | sed '/^$/d')
  else
    t=$(mktemp -d); git worktree add -q --detach "$t" HEAD 2>/dev/null
    if ( cd "$t" && git -c user.name=x -c user.email=x@x merge --no-commit --no-ff "origin/$base" >/dev/null 2>&1 ); then :; else
      conflict=1; files=$(cd "$t" && git diff --name-only --diff-filter=U)
    fi
    git worktree remove --force "$t" 2>/dev/null
  fi
fi
{ echo "$fetched; the branch is $ahead ahead, $behind behind origin/$base"; } > "$r/sync.txt"
if [ "$conflict" = 1 ]; then
  echo conflict > "$r/sync.status"
  printf 'conflicts with origin/%s in:\n%s\n' "$base" "$files" | tee -a "$r/sync.txt"
  echo "rebase or merge origin/$base, then run again"; exit 1
fi
echo clean > "$r/sync.status"; echo "merges cleanly into origin/$base; $ahead ahead, $behind behind"
