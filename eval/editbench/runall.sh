#!/bin/bash
# edit-bench full sweep (HASHLINE.md §7): 2 formats × 2 models × 20 tasks = 80 runs.
# The runner skips tasks whose result file exists, so this resumes after a cut.
# Needs OPEN_ROUTER_API_KEY in the environment.
set -u
root="$(cd "$(dirname "$0")/../.." && pwd)"
bin="$(mktemp -d)/editbench"
cd "$root" && go build -o "$bin" ./eval/editbench || exit 1
MODELS=(
  "${GLM_MODEL:-z-ai/glm-5.3-flash}"
  "${DEEPSEEK_MODEL:-deepseek/deepseek-v4.1-flash}"
)
for m in "${MODELS[@]}"; do
  for f in patch hashline; do
    "$bin" -model "$m" -format "$f"
  done
done
