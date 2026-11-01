# The TUI, screen by screen

Screens below were captured from running sessions at 96 columns. The agent's output is included, which is why the wording varies.

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

The header names the conversation. It reads "new session" until you ask
something. Then it keeps the first thing you asked, which is how the
conversation is listed by `memdoor resume`.

The footer's status line shows the git branch, the model that answered, and
its rung on the agent's ladder with the reason (`first rung · the default`,
or `pinned by you` after `/model 3`). Under it, the keys that apply.

![A question on this repository: judged reads, the answer with file:line](/demo.cast)

## In its own worktree

```
memdoor tui --worktree fix-login
```

The session works in `.memdoor/worktrees/fix-login` on branch
`memdoor/fix-login`, cut from your HEAD. Your checkout is not touched. Two
windows in two worktrees cannot step on each other's files. When the window
closes it says where the work is and how to merge it or throw it away. A
worktree where nothing changed is removed with its branch. A name reopens its
worktree. `--worktree` with no name derives one from the time.

The tab shows `●` while a turn runs and `✓` when one has finished and you have
not looked yet. If the window is in the background, the terminal gets a
bell and a notification.

## A turn, frame by frame

Each tool call renders as a frame:

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

The agent works in the directory you launched from. Its file tools stay inside it and its commands start there. It is not a sandbox: a shell command can still reach other folders.

## What it touched

`ctrl+f` (or `/files`) opens the project's files with the ones this
conversation read or changed first, newest first. Each carries what happened to
it (`✎ calc.go  2 edits · 1 read`). The rest of the tree is behind the same
filter. Type to filter. `↑↓` moves. Enter previews the file beside the
list. `ctrl+d` shows its diff since the turn started (a file the turn created
shows whole, as added). `ctrl+e` opens it in `$EDITOR` and comes back.
Tab puts `@path` into the prompt. Esc closes.

## This turn as a graph

`ctrl+g` (or `/graph`) draws the turn so far the way `/workflow` draws a
run: the reads and searches in the first layer, the edits that depended on
them, the runs and checks that depended on the edits. Each carries its state
and time. A failing check is red with its reason. The head line says how
many reads, edits and runs, and whether the last run passed. `↑↓` move.
`enter` on a file step opens it in the files panel. `esc` closes.

## What a frame shows

A frame summarizes the result. `ctrl+o` shows the raw output underneath:

- **a search** (`grep`, `glob`, `locate`, `web_search`) reads as its hits
  grouped by file with counts, most hits first. A web search reads as its
  sources. Nothing found reads as "no hits".
- **a spawned run** (`Spawn(…)`) shows the child's steps as it takes
  them, newest last, and `✓ reported back` when its result wakes the
  conversation.
- **a workflow call** shows the run's tasks with the panel's glyphs:
  `✓` done, `▶` running, `⏸` waiting for you, `✗` failed.
- **a change** shows its diff. **A command** streams its tail while it
  runs.

## Slash commands

### Typing `/` lists everything

The dropdown opens on `/` and filters as you type. Skills appear under `/skill:` alongside the built-ins, including skills the agent wrote for itself:

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

In the coding agent the list carries `/model` and `/go`. One more character narrows it.

### `/usage` — what this month cost

```
> /usage

  This month (2026-09)

  Turns        184      Input tokens    4.1M      Decisions   612
  Models       z-ai/glm-5.3-flash (171)  ·  z-ai/glm-5.3 (13)
```

This is Memdoor's per-turn ledger. Your provider's account is the authority
on cost. `memdoor meter` prints every line of it.

### `/model` — which model is answering, and pinning one

```
> /model

  coder   z-ai/glm-5.3-flash → deepseek/deepseek-v4.1-flash → z-ai/glm-5.3
          now: rung 1/3 · first rung · the default

  /model 2            pin a rung        /model auto          back to the ladder
  /model <vendor/name> [price|throughput|latency] [order a,b]
```

`/model-search glm` lists the catalogue with real prices per million tokens and
pins from the result. The picker moves with up/down, reorders hosts with
shift+up/down, and `s` cycles how they are sorted:

```
╭──────────────────────────────────────────────────────────────────────────────╮
│  z-ai/glm-5.3 · hosts, cheapest first                                        │
│  🟢 Baidu            fp8   1310k ctx   99.9%   $ in / $ out                   │
│    Morph            fp8   1048k ctx   99.7%   $ in / $ out                   │
│  up/down navigate • shift+up/down reorder • s sort • Enter pin • Esc cancel   │
╰──────────────────────────────────────────────────────────────────────────────╯
```

Prices are omitted from this capture. They change daily.

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

Skills are markdown instructions on disk. `/skill:review some/file.go` makes that skill the turn's task. Drop a file into `.agents/skills/` and it appears in the dropdown at once, with no restart.

### `/go`, `/fresh`, `/clear`, `/new`, `/exit`

When a turn has produced a plan, `/go` (also `/run`, `/approve`) hands it to the coder. `/clear` wipes what the agent remembers of this conversation, so its next turn starts from nothing. The messages stay for `memdoor resume`. `/fresh` does the same and sends your last request again. Use it when a turn has read too much; the note after `Esc` points to it. `/new` starts a fresh session. `/exit` leaves, same as `Ctrl+C`.

The full reference is [Slash commands](/docs/slash-commands).

## Reading long output

Scrolling up is not interrupted by new output. A bar at the bottom shows what arrived. `End` or a click on the bar jumps back to live:

```
 ↓ 2 new messages — End or click to jump to latest
```

Mouse wheel and `PgUp`/`PgDn` scroll. While the command list is open they move the selection instead.

## If the gateway restarts

The TUI reconnects and sends anything you typed while it was away. A step that was mid-flight may have finished during the gap. Its events are not replayed, so check the folder.

## If the gateway refuses the window

A gateway opens only your workspace and your conversations. If the window asks for someone else's, the gateway refuses and the TUI does not retry. It shows the reason once, with what to do:

```
⚠ the gateway refused this window: not your workspace (yours is acme). Start memdoor again without -w or $MEMDOOR_WORKSPACE: it opens your own workspace. …
```

`memdoor auth whoami` shows which account this machine is signed in as. `memdoor auth login-direct --email <email> --password <pass>` switches to another.

## From your phone

`/remote` prints a link and a QR code. The page it opens on your phone is this
TUI, run by your gateway. Watch a turn, answer its question, stop it,
start the next one, with this window closed if you like. On your gateway
it is free. Only the memdoor.ai relay, for a gateway kept to localhost, is
part of the hosted gateway.

See [Remote control](/docs/remote).

## Coming back to a conversation

Each launch starts a fresh conversation. To return to an earlier one, leave the TUI and:

```
$ memdoor resume

      TITLE                                   STARTED        LAST USED
   1. make the throttle limit configurable    Sep 6 10:24    7m ago

$ memdoor resume --last
Resuming "make the throttle limit configurable" (7m ago, 4 turns)
```

It opens with that title in the header and the last exchanges in view, on the same session. `memdoor resume` with no argument lists and asks. `resume 2` takes the second row. Conversations are listed for the current folder. `--all` shows every one.

## Where to next

- **[Headless, then attached](/docs/headless)** — runs that need no window, and coming back into one.
- **[Boundaries](/docs/boundaries)** — what the agent may touch, as rules you can read.
- **[Features](/docs/features)** — the complete overview.
- **[Getting Started](/docs/getting-started)** — install, key, first turn.
- **[How It Works](/docs/how-it-works)** — the architecture underneath.
