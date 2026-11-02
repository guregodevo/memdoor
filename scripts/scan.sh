#!/bin/sh
# scan.sh: the tree is public. Fail on a key, a personal path, a proprietary
# notice or a private host before it is pushed (make scan; .githooks/pre-push;
# CI). The personal patterns (the owner's home path and address, the server's
# addresses) are not written here: they are read from MEMDOOR_SCAN_PATTERNS, a
# file of extended regexes one per line, when it exists (the ops repo keeps
# one), so CI scans the public shapes and the owner's machine scans those plus
# its own.
set -e
cd "$(dirname "$0")/.."
# .memdoor/ is Memdoor's own state in a project (runs, diffs, reviews): never
# committed, never scanned.
# What is scanned is what could be committed: the tracked files and the
# untracked ones that are not ignored. A log or a build output that .gitignore
# keeps out is never public, so it is not scanned (the owner's gateway.log
# holds the home path and failed every deploy). Outside a git checkout, the
# whole tree.
publishable() {
    if git rev-parse --git-dir >/dev/null 2>&1; then
        git ls-files -z --cached --others --exclude-standard | grep -zvE '(^|/)\.memdoor/|\.cast$'
    else
        find . \( -name node_modules -o -name .git -o -name dist -o -name .memdoor \) -prune -o -type f ! -name '*.cast' -print0
    fi
}
fail=0
hit() { echo "SCAN FAILED ($1):" >&2; echo "$2" | sed 's/^/  /' >&2; fail=1; }
out=$(publishable | xargs -0 grep -IEnH "sk-or-v1-[A-Za-z0-9]{20}|sk-ant-[A-Za-z0-9-]{20}|sk-proj-[A-Za-z0-9_-]{20}|sk_live_[A-Za-z0-9]{10}|whsec_[A-Za-z0-9]{10}|ghp_[A-Za-z0-9]{30}|AKIA[0-9A-Z]{16}" 2>/dev/null | grep -v '_test.go:' || true)
[ -z "$out" ] || hit "a key" "$out"
out=$(publishable | xargs -0 grep -IElH "[Pp]roprietary\.|This repository is proprietary|All rights reserved" 2>/dev/null | grep -v '^scripts/scan.sh$' || true)
[ -z "$out" ] || hit "a proprietary notice" "$out"
if [ -n "${MEMDOOR_SCAN_PATTERNS:-}" ] && [ -f "$MEMDOOR_SCAN_PATTERNS" ]; then
    out=$(publishable | xargs -0 grep -IEnH -f "$MEMDOOR_SCAN_PATTERNS" 2>/dev/null || true)
    [ -z "$out" ] || hit "a private pattern" "$out"
fi
[ "$fail" = 0 ] && echo "scan clean"
exit $fail
