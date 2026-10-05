---
name: test-tui
description: Drive the real TUI from tmux to test a change end to end — the gateway to point it at, the keys to send, what to assert, and the traps that make a green test a lie
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: "\U0001F5A5"
    skillKey: test-tui
    os:
      - darwin
      - linux
---

# Testing the TUI

A unit test says the function returns the right value. It does not say the
panel opened, the keys reached it, or the file on disk changed. For anything a
person sees or types, drive the real program.

## Which half did you change?

The TUI is a client; the work is split, and so is the test.

| Changed | Needs | Does not need |
|---|---|---|
| `cmd/tui/...` (panels, keys, rendering, slash menu) | a fresh `make build` | a gateway restart |
| `gateway/...`, `pkg/...` (handlers, tools, stores) | `make build` **and** a gateway restart | — |

**Confirm which binary is serving before you believe a result.** A gateway
that is already up refuses to be replaced and says so in one line that is
easy to miss ("Gateway already running … leaving it alone"), so a "verified"
run can be testing the old code. This cost an hour once: the port was still
being served by a binary downloaded from production.

```bash
lsof -nP -iTCP:18799 -sTCP:LISTEN | tail -1   # the PID and the program
# More than one process can hold the port (a gateway and its child): kill each
# PID, wait for the port to free, then prove the new listener is a NEW PID —
# three runs in one evening tested a stale process before this became a rule.
# -sTCP:LISTEN: without it lsof also lists every CLIENT connected to the port,
# and the kill takes down your own TUI (2026-10-04).
OLDS=$(lsof -ti:18799 -sTCP:LISTEN | tr '\n' ' '); for p in ${=OLDS}; do kill $p; done   # ${=…}: zsh does not split $OLDS
for i in $(seq 1 20); do [ -z "$(lsof -ti:18799 -sTCP:LISTEN)" ] && break; sleep 1; done
# start yours, then:
NEW=$(lsof -ti:18799 -sTCP:LISTEN | head -1); case " $OLDS " in *" $NEW "*) echo ABORT; exit 1;; esac
```

## A gateway of your own

Never restart the user's gateway (`:18789`) to test your change — their TUI is
attached to it. Run your own on another port, with its own `HOME` so your
config, workspace and `~/.memdoor/mcp.json` are yours:

```bash
S=<scratch>                       # never the repo
set -a; . ./.envrc; set +a        # the OpenRouter key, sourced, never printed
HOME=$S/home nohup ./memdoor gateway --port 18799 > $S/gw.log 2>&1 & disown
sleep 15; curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18799/api/status
```

`nohup … & disown` from a foreground call: a backgrounded tool call reaps its
children. The first run of a fresh `HOME` needs `memdoor setup` once — a TUI
whose config does not match the gateway exits at once with "this machine is
not set up yet", which looks exactly like a crash in tmux.

## Driving it

```bash
cd $S/work && git init -q         # a scratch project, never the repo: the
                                  # coder writes files where the TUI started
env -u CMUX_WORKSPACE_ID -u CMUX_SOCKET_PATH \
  tmux new-session -d -s t -x 150 -y 46 \
  "MEMDOOR_NO_BROWSER=1 HOME=$S/home ./memdoor --gateway http://localhost:18799 tui"
sleep 12                          # start the TUI only after /api/status answers
tmux send-keys -t t "/mcp" Enter
tmux capture-pane -t t -p         # the rendered screen; -e keeps the colours
tmux capture-pane -t t -p -S -200 # with scrollback
tmux kill-session -t t            # yours only
```

- `send-keys -l "<text>"` sends it literally: without `-l`, words like `Enter`,
  `Up` or `Space` inside the text are read as key names.
- A panel owns the keyboard while it is open: letters are its shortcuts, `esc`
  closes it. Send `Escape` before typing a message, or your prompt becomes a
  sequence of shortcuts.
- `MEMDOOR_NO_BROWSER=1` on anything that signs in. Without it a real browser
  tab opens on the user's screen.
- Never `pkill -f "memdoor tui"` — that kills the user's session too.

Waiting: poll for what you expect rather than sleeping blind. A turn is
running while the screen says `esc to interrupt`; it has ended when that has
been gone for a few polls.

```bash
for i in $(seq 1 40); do tmux capture-pane -t t -p | grep -q "esc to interrupt" || break; sleep 3; done
```

## What to assert

**The filesystem and the logs, not the screen.** The screen says what the
model claims; the file says what happened.

```bash
cat $S/work/.mcp.json                      # the change the action should have made
./memdoor logs query --regex "PATTERN" --since 2m --limit 20
```

Assert the screen only for what is *only* on the screen: a cursor, a row's
state, a key hint, a dropdown's contents, an error's wording.

## Negative paths are the point

A feature that works when everything is right is half tested. For every input
box, drive: nothing on the machine (a command that does not exist), something
that runs but answers wrongly (`cat` for a protocol), nothing listening
(`http://localhost:1/…`), a truncated paste, and the same thing twice. Then
read the message as a person: a Go error reaching the box is a bug —
`Post "https://x/y": read tcp 192.168.1.135:56777->…: connection reset by peer`
printed the machine's own address at someone. Say what is wrong and what to
do, in words.

After a refused action, check that nothing was written. Saving what failed is
how a repository ends up carrying a server that fails for everyone who clones
it.

## A helper beats repetition

Six cases through one box is one small script, not six transcripts: send the
keys, wait, print the box and the file it should or should not have written.
Keep it in the scratch directory and throw it away with the run.

## When it hangs

Check, in this order: the gateway answers `/api/status`; the listener is your
binary; the TUI is still alive (`tmux has-session`); a picker is waiting for a
key; the gateway log. A turn can sit 30–120s before its first tool call —
read `AGENT BUILD` in the logs before calling it a hang.
