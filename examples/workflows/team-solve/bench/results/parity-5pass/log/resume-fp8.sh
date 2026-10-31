#!/bin/bash
# resume-fp8.sh <tag/id> <partition>: resume a stopped old-order run on gvs5h-parity-fp8, keeping its finished steps; graded like one-call.sh
S=<scratch>
WB=$HOME/memdoor-coder/lcb-bench/workflows; B=$HOME/Dev/aktapus/examples/workflows/team-solve/bench
q=$1; p=$2; id=${q#*/}; T=$S/onecall/$q
mkdir -p $T/.memdoor/runs/gvs5h-parity-fp8 && cp -Rn $T/.memdoor/runs/gvs5h-parity/. $T/.memdoor/runs/gvs5h-parity-fp8/
(cd $T && HOME=$S/home2 MEMDOOR_NO_BROWSER=1 $S/gw18/memdoor --gateway http://127.0.0.1:18799 workflow run --wait --timeout 12h --partition $p $WB/gvs5h-parity-fp8 > $T/resume-fp8.log 2>&1)
k=$(echo $p | tr -d 'T-' | sed -E 's/^([0-9]{8})/\1-/'); r=$(grep -oE 'started as [^ ]+' $T/resume-fp8.log | head -1 | awk '{print $3}')
c=$(python3 -I -c "
import json
rows=[json.loads(l) for l in open('$S/home2/.memdoor/meter.jsonl')]
rows=[x for x in rows if x.get('session') in ('workflow:gvs5h-parity-$k','workflow:$r')]
print('%.3f %d'%(sum(x.get('cost_usd') or 0 for x in rows), sum(x.get('output_tokens') or 0 for x in rows)))")
export LCB_BENCH=$HOME/memdoor-coder/lcb-bench LCB_RUNNER=$HOME/Dev/GVS5H/codebase/livecodebench
if [ -s $T/solution.py ]; then
  sm=$($S/lcb/venv/bin/python $B/grade.py --public $id $T/solution.py 2>/dev/null | head -1 | grep -oE "[0-9]+/[0-9]+")
  al=$($S/lcb/venv/bin/python $B/grade.py --private $id $T/solution.py 2>/dev/null | head -1 | grep -oE "[0-9]+/[0-9]+")
else sm=-; al="no code"; fi
line=$(printf '%s\t%s\t%s\t$%s\t%s tokens\tmoved to fp8 hosts mid-run' "$id" "$sm" "$al" ${c% *} ${c#* })
printf '%s\t%s\n' "${q%%/*}" "$line" >> $S/batch.tsv
echo "$(date +%H:%M:%S) finished $line"
