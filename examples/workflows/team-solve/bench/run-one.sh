#!/bin/sh
# run-one.sh <workflow dir> <id>: one problem through one workflow, then graded on the samples and on all tests.
set -u
B=$(cd "$(dirname "$0")" && pwd); WFDIR=$1; id=$2; WF=$(basename "$WFDIR"); P=${PROBLEMS:-$B/problems}; d="$P/$id"
GW=${MEMDOOR_GATEWAY:-http://localhost:18799}; MEMDOOR=${MEMDOOR_BIN:-memdoor}; PY=${LCB_PYTHON:-python3}
export MEMDOOR_NO_BROWSER=1
mkdir -p "$B/results" "$B/logs/$WF"; start=$(date +%s)
rm -f "$d/solution.py" "$d/plan.md" "$d/notes.md" "$d/review.md" "$d/answer.md" "$d"/*.log "$d/test-status.txt" "$d/tasks.json" "$d"/status-*.txt "$d"/next-*.txt "$d"/nexts-*.txt "$d"/wstatus-*.txt "$d"/samples-*.txt "$d"/digest-*.md "$d"/*-reply*.md "$d"/ideation-reply.md
[ -d "$d/.git" ] || git -C "$d" init -q
(cd "$d" && "$MEMDOOR" --gateway "$GW" workflow run --timeout "${RUN_TIMEOUT:-12h}" "$WFDIR") > "$B/logs/$WF/$id.log" 2>&1
run=$(grep -oE "started as [^ ]+" "$B/logs/$WF/$id.log" | head -1 | awk '{print $3}')
state=unknown
if [ -n "$run" ]; then
  i=0
  while [ $i -lt ${WAIT_STEPS:-8640} ]; do   # 5 s a step: 12 h by default (a ten-round harness whose calls may think for an hour)
    st=$(cd "$d" && "$MEMDOOR" --gateway "$GW" workflow status "$run" 2>&1)
    state=$(printf '%s' "$st" | head -1 | grep -oE "· (running|done|failed|stopped) ·" | tr -d '· ')
    [ "$state" = running ] || break
    sleep 5; i=$((i+1))
  done
  printf '%s\n' "$st" >> "$B/logs/$WF/$id.log"
fi
samples=-; all=-
if [ -f "$d/solution.py" ]; then
  samples=$("$PY" "$B/grade.py" --public "$id" "$d/solution.py" 2>/dev/null | head -1 | grep -oE "[0-9]+/[0-9]+")
  all=$("$PY" "$B/grade.py" --private "$id" "$d/solution.py" 2>/dev/null | head -1 | grep -oE "[0-9]+/[0-9]+")
fi
printf '%s\t%s\t%s\t%s\t%s\n' "$id" "${state:-unknown}" "${samples:--}" "${all:--}" "$(( $(date +%s) - start ))" >> "$B/results/$WF.tsv"
echo "$id ${state:-unknown} samples ${samples:--} all ${all:--} $(( $(date +%s) - start ))s"
