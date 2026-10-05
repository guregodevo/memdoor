#!/bin/bash
# $1 session $2 cast $3 "keys..." (a script: lines of `type <text>` / `key <tmux key>` / `sleep <s>`) $4 end marker $5 limit
S=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$(dirname "$0")/../.." && pwd)
sess=$1; cast=$2; script=$3; until=$4; limit=$5; dir=/tmp/memdoor-$sess
tmux kill-session -t $sess 2>/dev/null
# DEMO_DIR=<project>: the session runs THERE instead of in a clean checkout — a
# workflow of this repo's own (ship builds, pushes and deploys; a scratch
# checkout has no node_modules and no remote), recorded where it really runs.
if [ -n "$DEMO_DIR" ]; then dir=$DEMO_DIR; else
rm -rf $dir && mkdir -p $dir && git -C $REPO archive HEAD | tar -x -C $dir && (cd $dir && git init -q && git add -A >/dev/null 2>&1 && git -c user.email=demo@memdoor.ai -c user.name=demo commit -q -m checkout)
[ -d $REPO/.memdoor/workflows/digest ] && mkdir -p $dir/.memdoor/workflows && cp -R $REPO/.memdoor/workflows/digest $dir/.memdoor/workflows/
fi
cd $dir; env -u CMUX_WORKSPACE_ID -u CMUX_SOCKET_PATH tmux new-session -d -s $sess -x 96 -y 30 "MEMDOOR_NO_BROWSER=1 $REPO/memdoor tui"; sleep 7
python3 $S/sample-cast.py $sess $cast 96 30 "$until" $limit > $cast.log 2>&1 &
sleep 1
while IFS= read -r line; do
  case "$line" in
    type\ *) t="${line#type }"; for ((i=0;i<${#t};i++)); do tmux send-keys -t $sess -l "${t:$i:1}"; sleep 0.02; done; sleep 0.6; tmux send-keys -t $sess Enter ;;
    key\ *) tmux send-keys -t $sess "${line#key }" ;;
    sleep\ *) sleep "${line#sleep }" ;;
  esac
done <<< "$script"
for i in $(seq 1 $((limit/5+2))); do sleep 5; grep -q recorded $cast.log && break; done
cat $cast.log; tmux kill-session -t $sess 2>/dev/null
