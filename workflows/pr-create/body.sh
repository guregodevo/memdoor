#!/usr/bin/env bash
# body.sh <run dir> <issue>: the pull request's title (title.txt) and body (body.md). The title is the first
# line of description.md (what pr-description wrote); the rest is the Description. With the repository's
# .github/PULL_REQUEST_TEMPLATE.md the template is filled: the Description section replaced, `Fixes #` given
# the issue number, a checkbox whose backticked command passed in checks.tsv ticked (a failed one stays
# unticked with its exit status after it), and the `<test output>` line replaced by checks.txt. Without a
# template the body is the description and the test results. Nothing is added that a run did not record.
set -u
r="$1"; issue="${2:-}"
[ -s "$r/description.md" ] || { echo "no description.md: pr-description wrote nothing"; exit 1; }
head -1 "$r/description.md" > "$r/title.txt"; tail -n +2 "$r/description.md" | sed '1{/^$/d;}' > "$r/desc-body.md"
t=.github/PULL_REQUEST_TEMPLATE.md
if [ -f "$t" ]; then
  awk -v desc="$r/desc-body.md" -v logs="$r/checks.txt" -v tsv="$r/checks.tsv" -v issue="$issue" '
    BEGIN { while ((getline l < tsv) > 0) { split(l, a, "\t"); st[a[1]] = a[2] } }
    /^## / { skip = 0 }
    skip { next }
    /^## Description/ { print; print ""; while ((getline l < desc) > 0) print l; print ""; skip = 1; next }
    /^Fixes #/ { print (issue != "" ? "Fixes #" issue : $0); next }
    /^- \[ \] `/ {
      c = $0; sub(/^- \[ \] `/, "", c); sub(/`.*/, "", c)
      if (c in st) { if (st[c] == "0") { sub(/\[ \]/, "[x]") } else { $0 = $0 " (exit " st[c] ", see the output)" } }
      print; next }
    /^<test output>$/ { while ((getline l < logs) > 0) print l; next }
    { print }' "$t" > "$r/body.md"
else
  { cat "$r/desc-body.md"; printf '\n## Test results\n\n```\n'; cat "$r/checks.txt"; printf '```\n'; } > "$r/body.md"
fi
echo "body: $(head -1 "$r/title.txt")"
