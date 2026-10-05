#!/bin/bash
# The selftest's runner: start a fixture workflow in this scratch project,
# wait until it ends or waits at a gate, print its status.
M=${MEMDOOR_BIN:-memdoor}; D=$(cd "$(dirname "$0")" && pwd)
waitrun(){ for i in $(seq 1 ${2:-200}); do s=$($M workflow status $1 2>&1); echo "$s" | head -1 | grep -qE "· (done|failed|stopped|waiting) ·" && break; sleep 3; done; echo "$s"; }
case $1 in
  run) out=$($M workflow run $2 ${3:+--partition $3} --dir $D 2>&1); echo "$out" | head -1
       id=$(echo "$out" | grep -oE "[a-z0-9-]+-[0-9]{8}-[0-9]{6}(-[0-9]+)?" | head -1); [ -n "$id" ] && waitrun $id ;;
  wait) waitrun $2 ;;
  id) echo "$3" | grep -oE "$2-[0-9]{8}-[0-9]{6}(-[0-9]+)?" | head -1 ;;
esac
