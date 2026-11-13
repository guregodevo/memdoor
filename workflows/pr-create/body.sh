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
# A check's exit status, by its command, from checks.tsv.
status_of() {
  while IFS="$(printf '\t')" read -r cmd st; do [ "$cmd" = "$1" ] && { echo "$st"; return; }; done < "$r/checks.tsv"
}
if [ -f "$t" ]; then
  skip=0
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in "## "*) skip=0 ;; esac
    [ "$skip" = 1 ] && continue
    case "$line" in
      "## Description"*) printf '%s\n\n' "$line"; cat "$r/desc-body.md"; echo; skip=1; continue ;;
      "Fixes #"*) [ -n "$issue" ] && line="Fixes #$issue" ;;
      "<test output>") cat "$r/checks.txt"; continue ;;
      '- [ ] `'*)
        c=${line#- \[ \] \`}; c=${c%%\`*}; st=$(status_of "$c")
        if [ "$st" = 0 ]; then line="- [x] ${line#- \[ \] }"; elif [ -n "$st" ]; then line="$line (exit $st, see the output)"; fi ;;
    esac
    printf '%s\n' "$line"
  done < "$t" > "$r/body.md"
else
  { cat "$r/desc-body.md"; printf '\n## Test results\n\n```\n'; cat "$r/checks.txt"; printf '```\n'; } > "$r/body.md"
fi
echo "body: $(head -1 "$r/title.txt")"
