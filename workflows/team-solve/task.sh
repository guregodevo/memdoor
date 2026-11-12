#!/usr/bin/env bash
# task.sh <template> <prompt.md> <task.md>: the task from the template, the
# prompt in place of @@PROMPT@@, its blank first and last lines dropped. A
# prompt's own "## " headings would read as task.md's sections: they go one
# level down. awk on bytes (LC_ALL=C): no Python, and any text and locale
# pass through as they are (#110).
set -u
export LC_ALL=C
P="$2" awk '
  BEGIN {
    n = 0
    P = ENVIRON["P"]
    while ((getline l < P) > 0) { if (l ~ /^## /) l = "#" l; L[++n] = l }
    s = 1; while (s <= n && L[s] ~ /^[ \t\r]*$/) s++
    e = n; while (e >= s && L[e] ~ /^[ \t\r]*$/) e--
  }
  index($0, "@@PROMPT@@") {
    i = index($0, "@@PROMPT@@")
    printf "%s", substr($0, 1, i - 1)
    for (k = s; k <= e; k++) printf "%s%s", L[k], (k < e ? "\n" : "")
    print substr($0, i + 10)
    next
  }
  { print }
' "$1" > "$3"
