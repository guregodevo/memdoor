#!/usr/bin/env bash
# End-to-end TUI smoke test: drive the REAL `memdoor tui` in a PTY (via tmux)
# against a running gateway, send a task, and print the rendered screen. This is
# the shell analogue of Claude Code's stdin.write + lastFrame tests, but full
# stack — it exercises the actual binary, WebSocket, and agents.
#
# Usage:
#   scripts/tui-smoke.sh "create hello.go and run it"
#
# Env:
#   TASK        the message to send (or pass as $1)
#   WAIT        seconds to wait for the agent before capturing (default 60)
#   COLS/ROWS   PTY size (default 120x40)
#   GATEWAY     gateway URL (default http://localhost:18789)
set -euo pipefail

TASK="${1:-${TASK:-create hello.go that prints Hello World and run it}}"
WAIT="${WAIT:-60}"
COLS="${COLS:-120}"; ROWS="${ROWS:-40}"
GATEWAY="${GATEWAY:-http://localhost:18789}"
SESSION="tui-smoke-$$"

command -v tmux >/dev/null || { echo "tmux required (brew install tmux)"; exit 1; }
curl -s -m 3 "$GATEWAY/" >/dev/null || { echo "gateway not reachable at $GATEWAY"; exit 1; }

cleanup() { tmux kill-session -t "$SESSION" 2>/dev/null || true; }
trap cleanup EXIT

# Spawn the TUI in a detached PTY. memdoor tui creates a fresh channel per launch,
# so a turn isn't poisoned by a busy channel's history.
env="MEMDOOR_GATEWAY=$GATEWAY"
tmux new-session -d -s "$SESSION" -x "$COLS" -y "$ROWS"
tmux send-keys -t "$SESSION" "$env ./memdoor tui" Enter
sleep 8

agent=coder; case "$TASK" in @*\ *) agent="${TASK%% *}"; agent="${agent#@}";; esac
echo "== task: $TASK  (agent: $agent) =="
tmux send-keys -t "$SESSION" "$TASK" Enter
echo "== waiting ${WAIT}s for the agent =="
sleep "$WAIT"

echo "== rendered screen =="
tmux capture-pane -t "$SESSION" -p | grep -vE '^\s*$'
