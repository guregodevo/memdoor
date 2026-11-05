#!/usr/bin/env bash
# verdict.sh <file>: PASS, FAIL or none, from the first `verdict:` line in the
# first ten (markdown emphasis aside). An llm reviewer or judge that narrates
# instead of answering ("Let me inspect the diff…", 2026-11-05) gave no
# verdict: none, which is not a FAIL.
f="$1"
[ -s "$f" ] || { echo none; exit 0; }
v=$(head -10 "$f" | tr -d '*`_' | grep -m1 -oiE '^[[:space:]]*verdict:[[:space:]]*(PASS|FAIL)' | grep -oiE '(PASS|FAIL)$' | tr a-z A-Z)
echo "${v:-none}"
