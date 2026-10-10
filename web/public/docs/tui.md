# The TUI, screen by screen

Screens below were captured from running sessions at 96 columns — including the agent's own output, which is why the wording varies.

## The first screen

```
cd your-project && memdoor tui
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
something, then keeps the first thing you asked — which is also how the
conversation is listed by `memdoor resume`.

The footer's status line is the part worth learning: the git branch, the model
that answered, and its rung on the agent's ladder with the reason it is there
(`first rung · the default`, or `pinned by you` after `/model 3`).
Routing is never an opaque auto mode — the line always says what is answering
and why. Under it, the keys that apply right now.

![A question on this repository: judged reads, the answer with file:line](/demo.cast)

## In its own worktree

```
memdoor tui --worktree fix-login
```

The session works in `.memdoor/worktrees/fix-login` on branch
`memdoor/fix-login`, cut from your HEAD. Your checkout is not touched, and two
windows in two worktrees cannot step on each other's files. When the window
closes it says where the work is and how to merge it or throw it away; a
worktree where nothing changed is removed with its branch. The same name
reopens the same worktree. `--worktree` with no name picks one from the clock.

The tab shows `●` while a turn runs and `✓` when one has finished and you have
not looked yet. If the window is in the background, the terminal also gets a
bell and a notification.

## A turn, frame by frame

Type a task; every tool call renders as a frame:

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

The agent works in the directory you launched from: its file tools stay inside it and its commands start there — not a sandbox: a shell command can still reach other folders.

## What it touched

After a turn you want to know what changed and where, not to browse a tree.
`ctrl+f` (or `/files`) opens the project's files with the ones this
conversation read or changed first, newest first, each with what happened to
it (`✎ calc.go  2 edits · 1 read`); the rest of the tree is behind the same
filter. Type to filter, `↑↓` to move, enter to preview the file beside the
list, `ctrl+d` for its diff since the turn started (a file the turn created
shows whole, as added), `ctrl+e` to open it in `$EDITOR` and come back,
tab to put `@path` into the prompt, esc to close. No modes to learn: one key
in, one key out.

## This turn as a graph

`ctrl+g` (or `/graph`) draws the turn so far the way `/workflow` draws a
run: the reads and searches in the first layer, the edits that depended on
them, the runs and checks that depended on the edits, each with its state
and time, the failing check in red with its reason. The head line says how
many reads, edits and runs, and whether the last run passed. `↑↓` move,
`enter` on a file step opens it in the files panel, `esc` closes.

## What a frame shows

A frame says what happened in the shape you scan for, and `ctrl+o` shows
the raw result underneath:

- **a search** (`grep`, `glob`, `locate`, `web_search`) reads as its hits
  grouped by file with counts, most hits first; a web search as its
  sources; nothing found as "no hits".
- **a spawned run** (`Spawn(…)`) shows the child's own steps as it takes
  them, newest last, and `✓ reported back` when its result wakes the
  conversation.
- **a workflow call** shows the run's tasks with the panel's glyphs:
  `✓` done, `▶` running, `⏸` waiting for you, `✗` failed.
- **a change** shows its diff; **a command** streams its tail while it
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

In the coding agent the list also carries `/model` and `/go`. One more character narrows it.

### `/usage` — what this month cost

```
> /usage

  This month (2026-09)

  Turns        184      Input tokens    4.1M      Decisions   612
  Models       z-ai/glm-5.3-flash (171)  ·  z-ai/glm-5.3 (13)
```

Your provider's account is the authority on the money; this is Memdoor's own
per-turn ledger, and `memdoor meter` prints every line of it.

### `/model` — which model is answering, and pinning one

```
> /model

  coder   z-ai/glm-5.3-flash → deepseek/deepseek-v4.1-flash → z-ai/glm-5.3
          now: rung 1/3 · first rung · the default

  /model 2            pin a rung        /model auto          back to the ladder
  /model <vendor/name> [price|throughput|latency] [order a,b]
```

`/model-search glm` lists the catalogue with real prices per million tokens, and
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

The prices are left out of this capture: they change daily, and the picker
shows today's.

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

Skills are markdown instructions on disk. `/skill:review some/file.go` makes that skill the turn's task, deterministically. Drop a file into `.agents/skills/` and it appears in the dropdown immediately, no restart.

### `/go`, `/fresh`, `/clear`, `/new`, `/exit`

When a turn has produced a plan, `/go` (also `/run`, `/approve`) hands it to the coder. `/clear` wipes what the agent remembers of this conversation, so its next turn starts from nothing; the messages stay for `memdoor resume`. `/fresh` does the same and sends your last request again — the way out of a turn that has read too much, and what the note after `Esc` points to. `/new` starts a fresh session. `/exit` leaves, same as `Ctrl+C`.

The full reference is **[Slash commands](/docs/slash-commands)**.

## Reading long output

Scrolling up never yanks you back down when new output lands. A bar appears at the bottom telling you what you're missing, and `End` — or a click on the bar — jumps back to live:

```
 ↓ 2 new messages — End or click to jump to latest
```

Mouse wheel and `PgUp`/`PgDn` scroll; while the command list is open they move the selection instead.

## If the gateway restarts

The TUI reconnects on its own and sends anything you typed while it was away. A step that was mid-flight may have finished during the gap — its events aren't replayed, so check the folder rather than assuming it was lost.

## From your phone

`/remote` prints a link and a QR code. The page it opens on your phone is this
same TUI, running on your computer: watch a turn, answer its question, stop it,
start the next one. Remote control is Pro.

→ **[Remote control](/docs/remote)**

## Coming back to a conversation

Each launch starts a fresh conversation. To return to an earlier one, leave the TUI and:

```
$ memdoor conversations

      TITLE                                   STARTED        LAST USED
   1. cut the 30 seconds about coming home    Sep 6 10:24    7m ago

$ memdoor resume --last
Resuming "cut the 30 seconds about coming home" (7m ago, 4 turns)
```

It opens with that title in the header and the last exchanges in view, on the same session. `memdoor resume` with no argument lists and asks; `resume 2` takes the second row. Conversations are listed for the folder you're in — `--all` shows every one.

## Where to next

- **[Headless, then attached](/docs/headless)** — runs that need no window, and coming back into one.
- **[Boundaries](/docs/boundaries)** — what the agent may touch, as rules you can read.
- **[Features](/docs/features)** — the complete overview.
- **[Getting Started](/docs/getting-started)** — install, key, first turn.
- **[How It Works](/docs/how-it-works)** — the architecture underneath.
