#!/usr/bin/env bash
# pick-issue.sh <issues.tsv> <tried.txt> [label]: the next issue to work, its number on stdout; nothing
# when there is none. issues.tsv: one open issue a line, number, the count of assignees, the labels
# joined by commas, the title (as next.yaml writes it from gh). Left out: an issue with a human
# assignee, one already tried (tried.txt, one number a line) or whose branch memdoor/issue-<n>
# exists, and, when a label is given, one without it. The lowest number first: the oldest issue.
set -u
tsv="$1"; tried="$2"; label="${3:-}"
awk -F'\t' -v label="$label" -v tried="$tried" '
  BEGIN { while ((getline l < tried) > 0) if (l != "") done[l] = 1 }
  NF >= 3 && $1 ~ /^[0-9]+$/ && !($1 in done) && $2 == 0 {
    if (label != "") { n = split($3, ls, ","); ok = 0; for (i = 1; i <= n; i++) if (ls[i] == label) ok = 1; if (!ok) next }
    if (best == "" || $1 + 0 < best + 0) best = $1
  }
  END { if (best != "") print best }' "$tsv"
