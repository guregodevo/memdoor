#!/usr/bin/env bash
# A newcomer's first session, end to end, on the memdoor binary given: the
# three ways a person connects a model, a first turn on each, and the key
# still there after the gateway restarts (and, in CI, after an upgrade over
# the running gateway). The model is the stand-in provider (internal/standin):
# it answers one fixed reply and only to its own key, so the reply on screen
# proves the key reached it, and nothing is spent.
#
#   1. A key in the shell: DEEPSEEK_API_KEY (and DEEPSEEK_BASE_URL, the
#      stand-in) set, `memdoor` opens straight onto the model.
#   2. memdoor connect deepseek: the key pasted at the hidden prompt, then
#      `memdoor run` turns, a gateway restart, and the upgrade.
#   3. /connect deepseek in the window: the base URL replaced, the key pasted
#      over the prefilled DEEPSEEK_API_KEY, a turn in the same window.
#
# Each runs in a HOME of its own, with no other variable from this shell.
# The windows are driven in tmux.
#
#   go build -o standin ./internal/standin
#   STANDIN_BIN=./standin scripts/first-session.sh
#
# Env: MEMDOOR_BIN (default: memdoor on PATH), STANDIN_BIN (required),
# PORT (the gateway, default 18789), STANDIN_PORT (default 18961),
# UPGRADE=1 runs the installer from memdoor.ai over the running gateway: CI
# only, as it restarts the gateway service of the host's own user if there
# is one.
#
# Run by .github/workflows/installers.yml after each deploy.
set -euo pipefail

MEMDOOR_BIN=${MEMDOOR_BIN:-$(command -v memdoor || true)}
STANDIN_BIN=${STANDIN_BIN:?"the stand-in provider: go build -o standin ./internal/standin"}
PORT=${PORT:-18789}
STANDIN_PORT=${STANDIN_PORT:-18961}
UPGRADE=${UPGRADE:-0}
[ -x "$MEMDOOR_BIN" ] || { echo "no memdoor binary (MEMDOOR_BIN)"; exit 1; }
command -v tmux >/dev/null || { echo "tmux is needed to drive the window"; exit 1; }

WORK=$(mktemp -d)
GATEWAY=http://127.0.0.1:$PORT
STANDIN_URL=http://127.0.0.1:$STANDIN_PORT/v1
REPLY="Hello from the stand-in provider."
SESSIONS=""
KEY="sk-$(od -An -tx1 -N16 /dev/urandom | tr -d ' \n')" # a DeepSeek key's shape
printf %s "$KEY" > "$WORK/key"

fail() {
    echo "FAIL: $*"
    for s in $SESSIONS; do
        echo "--- the screen of $s"
        tmux capture-pane -t "$s" -p 2>/dev/null | grep -v '^[[:space:]]*$' | sed "s/$KEY/<key>/g" || true
    done
    echo "--- the stand-in's log"; tail -20 "$WORK/standin.log" || true
    for g in "$WORK"/*/home/.memdoor/gateway.log; do
        [ -f "$g" ] && { echo "--- $g"; grep -v 'Migration' "$g" | tail -8; }
    done
    exit 1
}

gateway_pid() { { curl -sf --max-time 2 "$GATEWAY/api/status" 2>/dev/null || true; } | sed -n 's/.*"pid":\([0-9]*\).*/\1/p'; }

stop_gateway() {
    local pid
    pid=$(gateway_pid)
    [ -n "$pid" ] || return 0
    kill "$pid" 2>/dev/null || true
    for _ in $(seq 1 30); do [ -z "$(gateway_pid)" ] && return 0; sleep 1; done
    kill -9 "$pid" 2>/dev/null || true
    sleep 1
}

cleanup() {
    for s in $SESSIONS; do tmux kill-session -t "$s" 2>/dev/null || true; done
    stop_gateway
    [ -n "${STANDIN_PID:-}" ] && kill "$STANDIN_PID" 2>/dev/null || true
    rm -rf "$WORK"
}
trap cleanup EXIT

# A clean environment: the host's PATH and terminal, a HOME of the scenario's
# own, the variables it names, and nothing else (no stray key).
in_env() { # home, then NAME=value... and the command
    local home=$1; shift
    env -i "HOME=$home" "PATH=$PATH" "TERM=xterm-256color" "LANG=${LANG:-en_US.UTF-8}" "USER=${USER:-ci}" \
        "TMPDIR=${TMPDIR:-/tmp}" MEMDOOR_OWNER=1 MEMDOOR_NO_BROWSER=1 "$@"
}

# The same as one shell line, for tmux.
env_line() { # home, then NAME=value... and the command
    local home=$1 a; shift
    printf 'env -i HOME=%q PATH=%q TERM=xterm-256color LANG=%q USER=%q TMPDIR=%q MEMDOOR_OWNER=1 MEMDOOR_NO_BROWSER=1' \
        "$home" "$PATH" "${LANG:-en_US.UTF-8}" "${USER:-ci}" "${TMPDIR:-/tmp}"
    for a in "$@"; do printf ' %q' "$a"; done
}

new_home() { mkdir -p "$WORK/$1/home" "$WORK/$1/proj"; (cd "$WORK/$1/proj" && git init -q . 2>/dev/null || true); }

# A memdoor command in the scenario's home and project; its output is printed.
md() { # scenario, then memdoor's arguments
    local s=$1; shift
    (cd "$WORK/$s/proj" && in_env "$WORK/$s/home" "$MEMDOOR_BIN" --gateway "$GATEWAY" "$@") 2>&1 | sed "s/$KEY/<key>/g"
}

# A program in a tmux pane of its own, 120x40.
open_pane() { # name, scenario, command line run in the project
    local name=$1 s=$2 cmd=$3
    tmux new-session -d -s "$name" -x 120 -y 40 "cd '$WORK/$s/proj' && $cmd; echo EXIT=\$?; sleep 600"
    SESSIONS="$SESSIONS $name"
}

screen() { tmux capture-pane -t "$1" -p; }

wait_for() { # pane, extended regex, seconds, what it means
    local pane=$1 re=$2 secs=$3
    for _ in $(seq 1 "$secs"); do
        screen "$pane" | grep -Eq "$re" && return 0
        sleep 1
    done
    fail "$4: no /$re/ on the screen after ${secs}s"
}

wait_gone() { # pane, extended regex, seconds, what it means
    local pane=$1 re=$2 secs=$3
    for _ in $(seq 1 "$secs"); do
        screen "$pane" | grep -Eq "$re" || return 0
        sleep 1
    done
    fail "$4: /$re/ still on the screen after ${secs}s"
}

paste_key() { # pane: the key as a terminal pastes it (bracketed paste)
    tmux load-buffer -b key "$WORK/key"
    tmux paste-buffer -p -d -b key -t "$1"
}

expect_reply() { # scenario: one `memdoor run` turn answered by the stand-in
    local out
    out=$(md "$1" run "say hello") || fail "memdoor run exited non-zero: $out"
    echo "$out" | tail -3
    echo "$out" | grep -qF "$REPLY" || fail "memdoor run: no reply from the stand-in"
}

# The stand-in provider.
STANDIN_KEY="$KEY" "$STANDIN_BIN" -port "$STANDIN_PORT" > "$WORK/standin.log" 2>&1 &
STANDIN_PID=$!
for _ in $(seq 1 20); do curl -sf -o /dev/null -H "Authorization: Bearer $KEY" "$STANDIN_URL/models" && break; sleep 0.5; done
curl -sf -o /dev/null -H "Authorization: Bearer $KEY" "$STANDIN_URL/models" || fail "the stand-in did not start"
[ -z "$(gateway_pid)" ] || fail "a gateway already answers on :$PORT; pick another PORT"
"$MEMDOOR_BIN" --version | head -1

echo "== 1. a key in the shell: the window opens on the model"
new_home shell
open_pane fs-shell shell "$(env_line "$WORK/shell/home" "DEEPSEEK_API_KEY=$KEY" "DEEPSEEK_BASE_URL=$STANDIN_URL" "$MEMDOOR_BIN" --gateway "$GATEWAY")"
wait_for fs-shell 'standin-1' 60 "the window did not open on the stand-in's model"
tmux send-keys -t fs-shell -l "say hello"; tmux send-keys -t fs-shell Enter
wait_for fs-shell "^ +$REPLY *\$" 90 "the first turn in the window got no reply"
echo "ok: the window opened on standin-1 and the first turn answered"
tmux kill-session -t fs-shell
stop_gateway

echo "== 2. memdoor connect deepseek: the key pasted at the prompt"
new_home cli
FIRST_BIN=$MEMDOOR_BIN
if [ "$UPGRADE" = 1 ]; then
    # Where the installer puts it in this HOME, so the upgrade replaces the
    # binary this scenario's gateway runs.
    mkdir -p "$WORK/cli/home/.local/bin"
    cp "$MEMDOOR_BIN" "$WORK/cli/home/.local/bin/memdoor"
    MEMDOOR_BIN=$WORK/cli/home/.local/bin/memdoor
fi
open_pane fs-cli cli "$(env_line "$WORK/cli/home" "$MEMDOOR_BIN" --gateway "$GATEWAY" connect deepseek --base "$STANDIN_URL")"
wait_for fs-cli 'Token or key' 60 "memdoor connect did not ask for the key"
paste_key fs-cli; tmux send-keys -t fs-cli Enter
wait_for fs-cli '✓ deepseek connected' 60 "memdoor connect did not connect"
wait_for fs-cli '^EXIT=0' 10 "memdoor connect did not exit 0"
echo "ok: connected"
expect_reply cli
echo "-- the gateway restarted, started again by memdoor run with no key in its environment"
stop_gateway
expect_reply cli
md cli providers | grep -E '^ +deepseek .* connected \(' || fail "deepseek is not connected in memdoor providers after the restart"
if [ "$UPGRADE" = 1 ]; then
    echo "-- the installer over the running gateway (what memdoor upgrade runs)"
    [ -n "$(gateway_pid)" ] || fail "no gateway running before the upgrade"
    out=$( (cd "$WORK/cli/proj" && curl -fsSL "https://memdoor.ai/install.sh?ci=1" |
        in_env "$WORK/cli/home" "PATH=$WORK/cli/home/.local/bin:$PATH" MEMDOOR_CI=1 "MEMDOOR_PORT=$PORT" bash) 2>&1) ||
        fail "the installer failed: $out"
    echo "$out" | grep -E '^(==>|✓|⚠)'
    echo "$out" | grep -qF "✓ Installed memdoor → $MEMDOOR_BIN" || fail "the installer did not replace $MEMDOOR_BIN"
    echo "$out" | grep -qE '✓ Gateway (restarted|started again)' || fail "the installer did not restart the gateway"
    "$MEMDOOR_BIN" --version | head -1
    expect_reply cli
    md cli providers | grep -E '^ +deepseek .* connected \(' || fail "deepseek is not connected in memdoor providers after the upgrade"
else
    echo "-- the upgrade: skipped (UPGRADE=1, in CI)"
fi
tmux kill-session -t fs-cli
stop_gateway
MEMDOOR_BIN=$FIRST_BIN

echo "== 3. /connect deepseek in the window: the key pasted over the prefilled name"
new_home window
open_pane fs-window window "$(env_line "$WORK/window/home" "$MEMDOOR_BIN" --gateway "$GATEWAY")"
wait_for fs-window 'No model connected yet: pick one below' 60 "the first run did not open the window on /connect"
wait_for fs-window 'Sign in with OpenRouter' 10 "the /connect list does not offer the OpenRouter sign-in"
tmux send-keys -t fs-window Escape
wait_gone fs-window 'Sign in with OpenRouter' 10 "Esc did not close the /connect list"
tmux send-keys -t fs-window -l "/connect deepseek"; tmux send-keys -t fs-window Enter
wait_for fs-window 'DeepSeek — base URL' 10 "/connect deepseek did not ask for the base URL"
tmux send-keys -t fs-window C-u; tmux send-keys -t fs-window -l "$STANDIN_URL"; tmux send-keys -t fs-window Enter
wait_for fs-window '> DEEPSEEK_API_KEY' 10 "the key step did not prefill DEEPSEEK_API_KEY"
paste_key fs-window
wait_gone fs-window '> DEEPSEEK_API_KEY' 5 "the pasted key did not replace the prefilled name"
tmux send-keys -t fs-window Enter
wait_for fs-window '✓ deepseek connected' 60 "/connect deepseek did not connect"
tmux send-keys -t fs-window -l "say hello"; tmux send-keys -t fs-window Enter
wait_for fs-window "^ +$REPLY *\$" 90 "the first turn after /connect got no reply"
echo "ok: /connect took the pasted key and the first turn answered"

echo "== the stand-in's requests"
sed -E 's/^[0-9/]+ [0-9:]+ //' "$WORK/standin.log" | sort | uniq -c
grep -q 'key=refused' "$WORK/standin.log" && fail "a request reached the stand-in with a wrong key"
echo "✓ first session: all three ways connect, answer, and keep the key"
