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
EXCL='--exclude-dir=node_modules --exclude-dir=.git --exclude-dir=dist --exclude=*.cast'
fail=0
hit() { echo "SCAN FAILED ($1):" >&2; echo "$2" | sed 's/^/  /' >&2; fail=1; }
out=$(grep -rIEn $EXCL "sk-or-v1-[A-Za-z0-9]{20}|sk-ant-[A-Za-z0-9-]{20}|sk-proj-[A-Za-z0-9_-]{20}|sk_live_[A-Za-z0-9]{10}|whsec_[A-Za-z0-9]{10}|ghp_[A-Za-z0-9]{30}|AKIA[0-9A-Z]{16}" . 2>/dev/null | grep -v '_test.go:' || true)
[ -z "$out" ] || hit "a key" "$out"
out=$(grep -rIEln $EXCL --exclude=scan.sh "[Pp]roprietary\.|This repository is proprietary|All rights reserved" . 2>/dev/null || true)
[ -z "$out" ] || hit "a proprietary notice" "$out"
if [ -n "${MEMDOOR_SCAN_PATTERNS:-}" ] && [ -f "$MEMDOOR_SCAN_PATTERNS" ]; then
    out=$(grep -rIEn $EXCL -f "$MEMDOOR_SCAN_PATTERNS" . 2>/dev/null || true)
    [ -z "$out" ] || hit "a private pattern" "$out"
fi
[ "$fail" = 0 ] && echo "scan clean"
exit $fail
