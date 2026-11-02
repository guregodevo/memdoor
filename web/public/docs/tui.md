# The TUI, screen by screen

Screens captured from running sessions at 96 columns, agent output included.

## The first screen

```
cd your-project && memdoor
```

```
  live myapp/notes · new session · ⏵⏵ auto
────────────────────────────────────────────────────────────────────────────────
  Ask for a change and it reads what it needs, patches, builds and runs the
  tests before it answers. @ mentions a file or folder · /model is which model
  answers · /remote opens this on your phone ·
  /help for the rest.



────────────────────────────────────────────────────────────────────────────────
 >
────────────────────────────────────────────────────────────────────────────────
  ⎇ main  ·  z-ai/glm-5.3-flash  ·  rung 1/3 · first rung · the default
  / commands · ctrl+o expand · ctrl+c quit
```

Header: the conversation's name, "new session" until the first request,
then that request (the title `memdoor resume` lists). Footer: git branch,
the model that answered, its rung and the reason (`first rung · the
default`, or `pinned by you` after `/model 3`), then the keys that apply.

![A question on this repository: judged reads, the answer with file:line](/demo.cast)

## In its own worktree

```
memdoor tui --worktree fix-login
```

Works in `.memdoor/worktrees/fix-login` on branch `memdoor/fix-login`, cut
from HEAD; the checkout is untouched. On close the window says where the
work is and how to merge or discard it. A worktree with no change is
removed with its branch. A name reopens its worktree; no name derives one
from the time.

Tab title: `●` while a turn runs, `✓` when one finished unseen. Background
window: a terminal bell and a notification.

## A turn, frame by frame

One frame per tool call:

```
> write hello.py that prints the first 5 squares, then run it

⏺ apply_patch(input: "*** Add File: hello.py
+for i in range(1, 6):
+    print(i ** 2)")
  Patch applied. added=[hello.py] modified=[] deleted=[]

⏺ Bash(python3 hello.py)
  1
  4
  9
  16
  25

    The script hello.py has been successfully run and printed the first 5
    squares.
```

The agent works in the launch directory: file tools inside it, commands
starting there. Not a sandbox: a shell command can reach other folders.

## What it touched

`ctrl+f` (or `/files`): the project's files, the ones this conversation
touched first, newest first, each with what happened (`✎ calc.go  2 edits ·
1 read`); the rest of the tree behind the same filter. Type to filter.
`↑↓` moves. Enter previews. `ctrl+d`: the diff since the turn started (a
created file shows whole). `ctrl+e`: open in `$EDITOR` and return. Tab:
`@path` into the prompt. Esc closes.

## This turn as a graph

`ctrl+g` (or `/graph`): the turn so far as a graph. Layers: reads and
searches, the edits that depended on them, the runs and checks that
depended on the edits; each with state and time. A failing check is red
with its reason. Head line: counts of reads, edits, runs, and whether the
last run passed. `↑↓` move. `enter` on a file step opens it in the files
panel. `esc` closes.

## What a frame shows

A frame summarizes the result. `ctrl+o` shows the raw output underneath.

- A search (`grep`, `glob`, `locate`, `web_search`): hits grouped by file
  with counts, most hits first. A web search: its sources. Nothing found:
  "no hits".
- A spawned run (`Spawn(…)`): the child's steps as it takes them, newest
  last, and `✓ reported back` when its result wakes the conversation.
- A workflow call: the run's tasks with the panel's glyphs: `✓` done, `▶`
  running, `⏸` waiting for you, `✗` failed.
- A change: its diff. A command: its tail, streamed.

## Slash commands

### Typing `/` lists everything

The dropdown opens on `/` and filters as you type. Skills, including ones
the agent wrote, appear under `/skill:`:

```
╭──────────────────────────────────────────────────────────────────────────────╮
│  /usage                                                                      │
│  /dir                                                                        │
│  /copy                                                                       │
│  /clear                                                                      │
│  /new                                                                        │
│  /exit                                                                       │
│  /skill:review                                                               │
│  up/down navigate • Enter/Tab select • Esc close                             │
╰──────────────────────────────────────────────────────────────────────────────╯
────────────────────────────────────────────────────────────────────────────────
 > /
```

In the coding agent the list carries `/model` and `/go`.

### `/usage` — what this month cost

```
> /usage

  This month (2026-09)

  Turns        184      Input tokens    4.1M      Decisions   612
  Models       z-ai/glm-5.3-flash (171)  ·  z-ai/glm-5.3 (13)
```

Memdoor's per-turn ledger; the provider's account is the authority on
cost. `memdoor meter` prints every line.

### `/model` — which model is answering, and pinning one

```
> /model

  coder   z-ai/glm-5.3-flash → deepseek/deepseek-v4.1-flash → z-ai/glm-5.3
          now: rung 1/3 · first rung · the default

  /model 2            pin a rung        /model auto          back to the ladder
  /model <vendor/name> [price|throughput|latency] [order a,b]
```

`/model-search glm`: the catalogue with prices per million tokens; pins
from the result. Picker: up/down moves, shift+up/down reorders hosts, `s`
cycles the sort:

```
╭──────────────────────────────────────────────────────────────────────────────╮
│  z-ai/glm-5.3 · hosts, cheapest first                                        │
│  🟢 Baidu            fp8   1310k ctx   99.9%   $ in / $ out                   │
│    Morph            fp8   1048k ctx   99.7%   $ in / $ out                   │
│  up/down navigate • shift+up/down reorder • s sort • Enter pin • Esc cancel   │
╰──────────────────────────────────────────────────────────────────────────────╯
```

Prices omitted from this capture; they change daily.

### `/context` — how full the window is

```
> /context

    Context Usage

    Tokens: 3.2k tokens / 28.7k tokens (11.2%)

     ████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░

    Status: Healthy
    • No compaction needed yet
    • Local compaction will trigger at 60%
```

`/compact` reports the same budget from the compaction side.

### `/agents` — who's available

```
> /agents

    agent:coder
    Name: coder
    Tools: 17 configured

    agent:planner
    Name: planner
    Tools: 4 configured
```

### `/skill:<name>` — run a skill as the task

A skill is a markdown file on disk. `/skill:review some/file.go` makes it
the turn's task. A file dropped into `.agents/skills/` appears in the
dropdown at once.

### `/go`, `/fresh`, `/clear`, `/new`, `/exit`

`/go` (also `/run`, `/approve`): hands a produced plan to the coder.
`/clear`: the agent forgets this conversation; the messages stay for
`memdoor resume`. `/fresh`: `/clear`, then the last request again; for a
turn that read too much. `/new`: a fresh session. `/exit`: same as
`Ctrl+C`.

Full reference: [Slash commands](/docs/slash-commands).

## Reading long output

New output does not interrupt scrolling up. A bar shows what arrived;
`End` or a click on it returns to live:

```
 ↓ 2 new messages — End or click to jump to latest
```

Mouse wheel and `PgUp`/`PgDn` scroll; with the command list open, they
move the selection.

## If the gateway restarts

The TUI reconnects and sends what was typed meanwhile. A step mid-flight
may have finished during the gap; its events are not replayed. Check the
folder.

## If the gateway refuses the window

A gateway opens only your workspace. A window asking for another is
refused, once, with the reason and what to do; the TUI does not retry:

```
⚠ the gateway refused this window: not your workspace (yours is acme). Start memdoor again without -w or $MEMDOOR_WORKSPACE: it opens your own workspace. …
```

`memdoor auth whoami`: which account this machine is signed in as. `memdoor
auth login-direct --email <email> --password <pass>`: switch to another.

## From your phone

`/remote`: a link and a QR code. The page is this TUI, run by your
gateway, usable with this window closed. Free on your gateway.

See [Remote control](/docs/remote).

## Coming back to a conversation

Each launch is a fresh conversation. An earlier one, from the shell:

```
$ memdoor resume

      TITLE                                   STARTED        LAST USED
   1. make the throttle limit configurable    Sep 6 10:24    7m ago

$ memdoor resume --last
Resuming "make the throttle limit configurable" (7m ago, 4 turns)
```

Opens with that title and the last exchanges in view, on the same session.
No argument: list and ask. `resume 2`: the second row. Listed per folder;
`--all`: every one.

## Where to next

- **[Headless, then attached](/docs/headless)** — runs that need no window, and coming back into one.
- **[Boundaries](/docs/boundaries)** — what the agent may touch, as rules you can read.
- **[Features](/docs/features)** — the complete overview.
- **[Getting Started](/docs/getting-started)** — install, key, first turn.
- **[How It Works](/docs/how-it-works)** — the architecture underneath.
