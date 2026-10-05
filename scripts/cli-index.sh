#!/usr/bin/env bash
# Regenerate the command index in docs/reference/CLI.md from the binary's own
# --help: every top-level command (hidden ones included) and its subcommands,
# under the groups `memdoor --help` prints. Run by `make cli-index`; the index
# drifted by hand three times (2026-10-03).
set -euo pipefail
BIN=${1:-./memdoor}
DOC=docs/reference/CLI.md
help() { "$BIN" "$@" --help 2>/dev/null; }
short() { help "$@" | sed -n '1p'; }

# The commands root.go leaves out of every --help listing (notCodingCommands).
hidden_code=$(sed -n '/^var notCodingCommands = \[\]string{/,/^}/p' cmd/cli/cmd/root.go | grep -v '^\s*//' | grep -oE '"[a-z-]+"' | tr -d '"' | sort -u)
visible=$("$BIN" --help 2>/dev/null | sed -n '/^Usage:/,/^Flags:/p' | grep -E '^  [a-z]' | awk '{print $1}' | grep -vx 'help\|completion\|memdoor' | sort -u)

out=$(mktemp)
{
  all=$( { help | sed -n '/^Usage:/,/^Flags:/p' | grep -E '^  [a-z]' | awk '{print $1}'; echo "$hidden_code"; } | grep -vx 'help\|completion\|memdoor' | grep -v '^$' | sort -u)
  total=$(echo "$all" | wc -l | tr -d ' ')
  nvis=$(echo "$visible" | wc -l | tr -d ' ')
  hid=$(comm -23 <(echo "$all") <(echo "$visible") | sed 's/.*/`&`/' | paste -sd, - | sed 's/,/, /g')
  echo "Generated from the binary's \`--help\` by \`make cli-index\` ($total top-level commands)."
  echo "\`memdoor --help\` shows $nvis of them; the others are hidden from it and run when typed:"
  echo "$hid. A command that appears here"
  echo "and nowhere else in this file is documented by its own \`memdoor <command> --help\`."
  echo
  # Groups in the order --help prints them; then the ungrouped and hidden.
  help | sed -n '/^Usage:/,/^Flags:/p' | awk '
    /^[A-Z][A-Za-z &,]*:$/ && $0 !~ /^Usage:|^Flags:/ {g=$0; sub(/:$/,"",g); next}
    /^  [a-z]/ && $1 != "memdoor" {print g "\t" $1}' > "$out.groups"
  for c in $hidden_code; do printf 'Hidden\t%s\n' "$c" >> "$out.groups"; done
  cut -f1 "$out.groups" | awk '!seen[$0]++' | while IFS= read -r g; do
    [ "$g" = "Additional Commands" ] && title="Other commands" || title="$g"
    [ "$g" = "Hidden" ] && title="Hidden (notCodingCommands in root.go)"
    echo "**$title**"; echo
    awk -F'\t' -v g="$g" '$1==g {print $2}' "$out.groups" | { grep -vx 'help\|completion' || true; } | while read -r c; do
      echo "- \`$c\` — $(short "$c")"
      help "$c" | sed -n '/^Available Commands:/,/^$/p' | { grep -E '^  [a-z]' || true; } | { grep -v '^  help ' || true; } | while read -r s rest; do
        echo "  - \`$s\` — $rest"
      done
    done
    echo
  done
} > "$out"
python3 - "$DOC" "$out" <<'PY'
import sys,re
doc,new=sys.argv[1],open(sys.argv[2]).read().rstrip()+"\n"
s=open(doc).read()
a,b="<!-- cli-index:start -->\n","<!-- cli-index:end -->"
if a not in s:
    i=s.index("## Command index\n")+len("## Command index\n\n")
    j=s.index("\n---\n\n## Commands")
    s=s[:i]+a+s[i:j].rstrip()+"\n"+b+"\n"+s[j:]
i=s.index(a)+len(a); j=s.index(b)
open(doc,"w").write(s[:i]+new+s[j:])
PY
rm -f "$out" "$out.groups"
echo "cli-index: $DOC updated"
