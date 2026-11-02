#!/usr/bin/env bash
# A nonzero exit is preserved by the final workflow command.
set -u
shopt -s nullglob
inputs=(tests/*.in)
if [ "${#inputs[@]}" -eq 0 ]; then
    printf 'FAIL: no sample inputs found\n'
    exit 1
fi
actual=$(mktemp) || exit 1
trap 'rm -f "$actual"' EXIT
failures=0
for input in "${inputs[@]}"; do
    expected="${input%.in}.out"
    if [ ! -f "$expected" ]; then
        printf 'FAIL %s: missing %s\n' "$input" "$expected"
        failures=$((failures + 1))
        continue
    fi
    if ! python3 solution.py < "$input" > "$actual"; then
        printf 'FAIL %s: solution exited unsuccessfully\n' "$input"
        failures=$((failures + 1))
    elif diff -u "$expected" "$actual"; then
        printf 'PASS %s\n' "$input"
    else
        printf 'FAIL %s: output differs\n' "$input"
        failures=$((failures + 1))
    fi
done
printf 'Samples: %s; failures: %s\n' "${#inputs[@]}" "$failures"
[ "$failures" -eq 0 ]
