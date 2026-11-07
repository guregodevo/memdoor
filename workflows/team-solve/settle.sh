#!/usr/bin/env bash
# settle.sh <run dir>: the plan's sections into task.md, the Prompt untouched.
# A section the planner left out or got wrong is filled safely: the Check by
# the one the prompt names ("Check: go test ./x/ passes."), else the judge; the
# Scope by the paths the prompt names that exist, else the whole tree; so a run
# never stops on its plan. awk on bytes (LC_ALL=C): no Python, and any text
# and locale pass through as they are (#110).
set -u
export LC_ALL=C
r="$1"; plan="$r/plan.md"; task="$r/task.md"; prompt="$r/prompt.md"
v="$r/.settle"; mkdir -p "$v"
filled=()

# section <file> <name>: the lines under "## <name>" up to the next "## "
# heading, without the blank lines around them.
section() {
  awk -v name="$2" '
    f && /^##[ \t]/ { exit }
    f { L[++n] = $0 }
    !f && $0 ~ ("^##[ \t]*" name "[ \t]*$") { f = 1 }
    END {
      s = 1; while (s <= n && L[s] ~ /^[ \t\r]*$/) s++
      e = n; while (e >= s && L[e] ~ /^[ \t\r]*$/) e--
      if (s > e) exit
      sub(/^[ \t\r]+/, "", L[s]); sub(/[ \t\r]+$/, "", L[e])
      for (k = s; k <= e; k++) print L[k]
    }' "$1"
}
ticked() { grep -q '`[^`][^`]*`' "$1"; }

for name in Deliverable Check Scope "Done when"; do section "$plan" "$name" > "$v/$name"; done

if [ ! -s "$v/Deliverable" ]; then echo "changes to this tree" > "$v/Deliverable"; filled+=(Deliverable); fi

first=$(awk '{ for (i = 1; i <= NF; i++) { print $i; exit } }' "$v/Check" | tr -d '`*' | tr 'A-Z' 'a-z')
if ! ticked "$v/Check" && [ "$first" != samples ] && [ "$first" != judge ] && [ "$first" != reproduce ]; then
  cmd=$(awk '{
    i = index($0, "Check:"); if (!i) next
    s = substr($0, i + 6); sub(/^[ \t]+/, "", s); sub(/^`/, "", s)
    if (match(s, /`?[ \t]+(passes|exits 0)([^A-Za-z0-9_]|$)/)) {
      c = substr(s, 1, RSTART - 1); sub(/^[ \t]+/, "", c)
      if (c != "" && c !~ /`/) { print c; exit }
    }
  }' "$prompt")
  if [ -n "$cmd" ]; then printf '`%s`\n' "$cmd"; else echo judge; fi > "$v/Check"
  filled+=(Check)
fi

if ! ticked "$v/Scope"; then
  grep -oE '[A-Za-z0-9_./-]+/[A-Za-z0-9_./-]+|[A-Za-z0-9_-]+\.[A-Za-z0-9_]{1,5}' "$prompt" | sed 's/[.,:;)]*$//' | sort -u |
    while IFS= read -r p; do [ -n "$p" ] && [ -e "$p" ] && printf '`%s`\n' "$p"; done > "$v/Scope"
  [ -s "$v/Scope" ] || echo '`.`' > "$v/Scope"
  filled+=(Scope)
fi

if [ ! -s "$v/Done when" ]; then echo "The Prompt is done as written." > "$v/Done when"; filled+=("Done when"); fi

# task.md's own sections, split at their "## " headings; the Prompt's own
# headings were put one level down by task.sh, so they do not split.
awk -v V="$v" '
  function emit(name,   l, f) { f = V "/" name; while ((getline l < f) > 0) print l; close(f) }
  /^## / {
    name = substr($0, 4); sub(/^[ \t]+/, "", name); sub(/[ \t]+$/, "", name)
    print
    skip = (name == "Deliverable" || name == "Check" || name == "Scope" || name == "Done when")
    if (skip) { print ""; emit(name); print "" }
    next
  }
  !skip { print }
' "$task" > "$task.new" && mv "$task.new" "$task"
rm -rf "$v"

out=""
for name in Deliverable Check Scope "Done when"; do
  mark=""; for f in "${filled[@]+"${filled[@]}"}"; do [ "$f" = "$name" ] && mark=" (filled safely)"; done
  out="$out${out:+, }$name$mark"
done
echo "settled: $out"
