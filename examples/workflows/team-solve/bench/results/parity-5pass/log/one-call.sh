#!/bin/bash
# one-call.sh <id>: one round-1 worker call on problem <id>, authors' exact form, no plan; graded locally
S=<scratch>
WB=$HOME/memdoor-coder/lcb-bench/workflows; B=$HOME/Dev/aktapus/examples/workflows/team-solve/bench
P=$HOME/memdoor-coder/lcb-bench/problems-27b
mkdir -p $S/claims/${TAG:-p1}; mkdir $S/claims/${TAG:-p1}/$1 2>/dev/null || { printf "%s\tskip (claimed by the other batch)\n" "$1"; exit 0; }
[ "$WF" = gvs5h-parity ] && WF=gvs5h-parity-fp8  # 2026-10-31 15:50: FP8 hosts only (Greg)
T=$S/onecall/${TAG:+$TAG/}$1; rm -rf "${T:?}"; mkdir -p $T; cp $P/$1/task.md $P/$1/problem.md $T/; cp -R $P/$1/tests $T/; [ -n "$PLAN_FILE" ] && cp "$PLAN_FILE" $T/plan.md; git -C $T init -q
out=$(cd $T && HOME=$S/home2 MEMDOOR_NO_BROWSER=1 $S/gw18/memdoor --gateway http://127.0.0.1:18799 workflow run --wait --timeout 12h $WB/${WF:-worker-mimic} 2>&1)
r=$(printf '%s' "$out" | grep -oE "started as [^ ]+" | head -1 | awk '{print $3}')
c=$(python3 -I -c "
import json
rows=[json.loads(l) for l in open('$S/home2/.memdoor/meter.jsonl') if json.loads(l).get('session')=='workflow:$r']
print('%.3f %d'%(sum(x.get('cost_usd') or 0 for x in rows), sum(x.get('output_tokens') or 0 for x in rows)))")
export LCB_BENCH=$HOME/memdoor-coder/lcb-bench LCB_RUNNER=$HOME/Dev/GVS5H/codebase/livecodebench
if [ -s $T/solution.py ]; then
  sm=$($S/lcb/venv/bin/python $B/grade.py --public $1 $T/solution.py 2>/dev/null | head -1 | grep -oE "[0-9]+/[0-9]+")
  al=$($S/lcb/venv/bin/python $B/grade.py --private $1 $T/solution.py 2>/dev/null | head -1 | grep -oE "[0-9]+/[0-9]+")
else sm=-; al="no code"; fi
printf '%s\t%s\t%s\t$%s\t%s tokens\n' "$1" "$sm" "$al" ${c% *} ${c#* }
