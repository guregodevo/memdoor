#!/usr/bin/env bash
# failures.sh <log>: the failing tests a test runner's log lists, one per line,
# sorted. It reads pytest's short summary (FAILED/ERROR lines), unittest's and
# Django's runner (FAIL:/ERROR: headers) and go test (--- FAIL lines, and a
# package that failed). Exit 2 when the log is not a finished run of one of
# them, or its count of failures does not match the list it printed: then
# nothing is known about which tests failed (check.sh, #109). Exit 2 too
# when it names none: a failed run then says nothing about the tests.
set -u
export LC_ALL=C
log="$1"
[ -f "$log" ] || exit 2
out=$(awk '
  function num(s, word,   a) { return match(s, "[0-9]+ " word) ? substr(s, RSTART, RLENGTH) + 0 : 0 }
  # pytest: "FAILED path::test - why", and the closing "N failed, M passed in 1.2s"
  /^(FAILED|ERROR) [^ (]/ { id = substr($0, index($0, " ") + 1); sub(/ - .*$/, "", id); py[id] = 1; pyn++ }
  /[0-9]+ (passed|failed|errors?|skipped|deselected|xfailed|xpassed)[ ,].* in [0-9.]+s|[0-9]+ (passed|failed|errors?|skipped) in [0-9.]+s|no tests ran in [0-9.]+s/ {
    pydone = 1; pyexp = num($0, "failed") + num($0, "error")
  }
  # unittest and Django: "FAIL: test_x (mod.Class)", "Ran 12 tests in 0.3s", "FAILED (failures=1, errors=2)" or "OK"
  /^(FAIL|ERROR): / { id = substr($0, index($0, ": ") + 2); ut[id] = 1; utn++ }
  /^Ran [0-9]+ tests? in / { utran = 1 }
  utran && /^OK( |$)/ { utdone = 1; utexp = 0 }
  utran && /^FAILED \(/ {
    utdone = 1; s = $0; utexp = 0
    if (match(s, /failures=[0-9]+/)) utexp += substr(s, RSTART + 9, RLENGTH - 9) + 0
    if (match(s, /errors=[0-9]+/)) utexp += substr(s, RSTART + 7, RLENGTH - 7) + 0
  }
  # go test: "--- FAIL: TestX (0.0s)", a package line "ok  pkg 0.1s" or "FAIL\tpkg\t0.1s", "FAIL\tpkg [build failed]"
  /^[ \t]*--- FAIL: / { id = $0; sub(/^[ \t]*--- FAIL: /, "", id); sub(/ \([0-9.]+s\)$/, "", id); go[id] = 1 }
  /^(ok|FAIL|\?)[ \t]+[^ \t:]+([ \t]|$)/ { godone = 1 }
  /^FAIL[ \t]+[^ \t:]+([ \t]|$)/ { id = $0; sub(/^FAIL[ \t]+/, "", id); sub(/[ \t]+[0-9.]+s$/, "", id); go["package " id] = 1 }
  END {
    known = 0; c = 0
    if (pydone && pyn == pyexp) { known = 1; for (k in py) { print k; c++ } }
    else if (pydone) exit 2
    if (utdone && utn == utexp) { known = 1; for (k in ut) { print k; c++ } }
    else if (utran) exit 2
    if (godone) { known = 1; for (k in go) { print k; c++ } }
    # A failed run that names no failing test says nothing about the tests.
    if (!known || !c) exit 2
  }
' "$log") || exit 2
[ -z "$out" ] || printf '%s\n' "$out" | sort -u
