# Headless, then attached

Every run is a conversation on your gateway, whether a window was open or
not. That is what lets you leave a run alone and come back into it live,
instead of killing it and starting over.

Headless means it runs without a desktop app or an interface: the gateway is
a background process on the machine the run belongs to (your laptop, a CI
runner), and nothing else is needed. You never start it by hand: `memdoor tui`, `memdoor acp`, `memdoor workflow
run`, `memdoor workflow resume` and `memdoor cron add` start one when none
is running, and set the machine up the first time. A run with the laptop
closed is not here yet; it is what the hosted workflow state on memdoor.ai
is for, and it is coming.

## Runs that need no window

- **A scheduled check.** Inside a turn the agent schedules its own re-check
  (`cron(every: "20s", task: "is CI green?", times: 6)`) and the turn ends;
  from the shell, `memdoor cron add --id nightly --schedule "0 6 * * *"
  --agent coder --message "…"` runs a turn on a schedule, and `--workflow
  <name>` runs a workflow on one. See [Scheduled checks](/docs/cron).
- **A workflow from the shell or CI.** `memdoor workflow run <name>` starts
  the project's workflow and prints the run; `memdoor workflow status
  <run-id>` shows every task's state, `history <name>` what earlier runs did.
  On a CI runner the whole job is the installer, the key in the environment
  and that one command: it starts the gateway and sets the runner up itself.
  See [Workflows](/docs/workflows).
- **An editor session.** The coder in VS Code, Zed or JetBrains is a
  conversation on the same gateway. See [In your editor](/docs/editor).

Each of these answers into a conversation; a check's answer also wakes the
conversation that asked for it, so the agent acts on it without you.

## Coming back in

```
$ memdoor resume --all
Recent conversations:

      ID        TITLE                                        STARTED        LAST USED
   1. 41af4ac5  nightly: build the rule, grade it, file it   Oct 10 06:00   9m ago   ~/Dev/qant
   2. c0921b32  release                                      Oct 10 21:42   2h ago   ~/Dev/memdoor
   3. f9697a1e  make the wait ceiling configurable           Oct 10 19:01   5h ago   ~/Dev/throttle
$ memdoor resume 2
```

The window opens on that conversation, its saved messages in view and the
same session underneath: your next line is the next turn of that run. If a
turn is running when you attach, you see it from that moment on (its tool
frames, its answer) and `Esc` interrupts it; what it did before you arrived
is in the saved messages, not replayed as frames. `memdoor resume <id>`
takes the id a run printed, from any directory; with no `--all` the list is
this folder's conversations.

## A run waiting on you

A workflow gate shows ⏸ with the diff. Approve from the window (`a`) or from
anywhere:

```bash
memdoor workflow approve <run-id> <task>
memdoor workflow changes <run-id> <task> "the notes miss the migration"
```

`changes` reruns the task with your comment and waits again. A failed run
is resumed at the failed step with `memdoor workflow resume <run-id>`; what
it finished is skipped.

## From your phone

`/remote` in a window gives a link that is the terminal itself, end to end
encrypted, so a run can be watched and answered from a phone. See
[Remote control](/docs/remote).

## Letting a turn run

A turn judged unfinished by its receipts continues on its own until it is
shown done or a token budget is spent; the budget is what makes an
unattended turn safe to leave. See [No babysitting](/docs/no-babysitting).

## If the gateway restarts

Conversations and runs are on disk; a window reconnects and shows what it
missed, and a workflow resumes from its last finished task.

## Next

- **[Scheduled checks](/docs/cron)** · **[Workflows](/docs/workflows)** · **[In your editor](/docs/editor)**
- **[Boundaries](/docs/boundaries)** — what a run may touch while you are away.
