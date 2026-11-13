#!/usr/bin/env bash
# pick-area.sh <run dir> <focus> [minutes]: the one area this run explores, written to <run dir>/focus.txt. A focus given
# by the person wins. Else the first line of .memdoor/qa/areas.txt that .memdoor/qa/tried.txt does not hold,
# which is then added to tried.txt; with every area tried, tried.txt starts over. The schedule is the budget:
# one area a run. <minutes> (default 8) is the soft deadline written to <run dir>/deadline.txt as epoch seconds:
# the explorer stops itself by then, before the step's clock stops it with nothing written.
set -u
r="$1"; focus="${2:-}"; minutes="${3:-8}"
printf '%s\n' "$(( $(date +%s) + minutes * 60 ))" > "$r/deadline.txt"
areas=.memdoor/qa/areas.txt; tried=.memdoor/qa/tried.txt
mkdir -p .memdoor/qa; touch "$tried"
if [ -n "$focus" ]; then printf '%s\n' "$focus" > "$r/focus.txt"; echo "area: $focus (asked for)"; exit 0; fi
pick() { awk 'FILENAME == ARGV[1] { done[$0] = 1; next } NF && !($0 in done) { print; exit }' "$tried" "$areas" 2>/dev/null; }
a=$(pick)
if [ -z "$a" ]; then : > "$tried"; a=$(pick); fi
if [ -z "$a" ]; then : > "$r/focus.txt"; echo "area: none listed, the whole project"; exit 0; fi
printf '%s\n' "$a" > "$r/focus.txt"; printf '%s\n' "$a" >> "$tried"
echo "area: $a ($(grep -c . "$areas") listed, $(grep -c . "$tried") tried)"
