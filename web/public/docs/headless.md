# Headless, then attached

One gateway, every door. Every run is a conversation on your gateway,
whether a window was open or not: the window, the phone, the editor, a
schedule, a workflow from a shell or CI, a spawned child, and `memdoor
resume` into any of them. That is what lets you leave a run alone and come
back into it live, instead of killing it and starting over.

## The rule

Every command is headless except the window. `memdoor tui` is the window.
`memdoor resume` is the window at a terminal and text into a pipe, the Unix
rule, no flag. `memdoor run`, `workflow run`, `cron add`, `guards` and the
rest are commands and nothing else: a prompt or arguments in, text or events
out, an exit code. Stdin is the input when it is a pipe or a file; a
terminal is never read.

Headless means it runs without a desktop app or an interface: the gateway is
a background process on the machine the run belongs to (your laptop, a CI
runner), and nothing else is needed. You never start it by hand: `memdoor
run`, `workflow run`, `workflow resume`, `cron add`, `resume`, `tui` and
`acp` start one when none is running, and set the machine up the first
time (a home that has never seen Memdoor is one command away from its first
answer). A run with the laptop closed runs on [a box that stays
on](/docs/gateway): `memdoor gateway install` there, and your laptop, ssh and
your phone are doors onto it. A box that memdoor.ai runs for you is coming.

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
  and that one command with `--wait`: it starts the gateway, sets the runner
  up itself, and exits with the run's outcome. On GitHub that job is one
  step, `uses: guregodevo/memdoor@v1` with the workflow's name and your
  key; `pr-review`, `fix-tests` and `release-notes` are built in and need
  no file in the repo. See [Workflows](/docs/workflows).
- **An editor session.** The coder in VS Code, Zed or JetBrains is a
  conversation on the same gateway. See [In your editor](/docs/editor).
- **The phone, from a script.** `memdoor remote <id>` prints the
  link `/remote` would, on stdout, and returns: the gateway holds the link,
  so the run a script or a CI job started is on your phone for as long as
  that gateway runs. See [Remote control](/docs/remote).

Each of these answers into a conversation; a check's answer also wakes the
conversation that asked for it, so the agent acts on it without you.

## A turn from a script

`memdoor run` is one coder turn with no window: the prompt from the
arguments or stdin, the answer on stdout, everything else on stderr, and an
exit code that is the turn's receipt.

```bash
memdoor run "make the retry wait configurable and add a test"
git diff | memdoor run "review this diff; one line per finding"
memdoor run --json "fix the failing test" | jq -r 'select(.event=="done") | .receipt'
```

With both arguments and a pipe, what is piped in is the input the prompt is
about. `--json` streams NDJSON events (`text`, `tool_start`, `tool_done`,
`question`, `done`); the `done` event carries the conversation id, the
receipt and the exit code. The codes:

| Exit | Meaning |
|---|---|
| 0 | the turn ended and what it changed was checked, or nothing needed a check |
| 1 | the gateway, the provider or the turn failed |
| 2 | it ended unverified: a change with no check after it, or a failing check |
| 3 | in a pipe, the agent asked a question nobody could answer and took the default; `--yes` answers the first option instead. At a terminal you answer it there. |
| 130 | ctrl-c or a TERM signal ended it; the conversation keeps what ran, `memdoor resume <id>` continues it |

![A turn from a script: the answer on stdout, the exit code, then the same run opened in a window](/headless.cast)

Each run is a conversation: its id is on stderr at the end, and `memdoor
resume <id>` opens it in a window. `--conversation <id>` continues one.

`memdoor workflow run <name> --wait` is the same contract for a workflow:
progress on stderr, the files the run produced on stdout, exit 0 done, 2
failed, 1 stopped; a gate is reported and waited on. `--stop-at-gate` ends
it at a gate instead, exit 3 with the gate's review on stderr: the job that
has nobody to approve from (the run stays waiting on the gateway).

## A walk through the doors

Every command below was run in this order on 11 October 2026, against a
gateway that was not running when the first one started, in a scratch Go
project whose `Add` returned `a - b`. The exit codes are the receipts.

| Command | Exit | What it did |
|---|---|---|
| `echo calc.go \| memdoor run "which function does the file on stdin export? change nothing"` | 0 | started the gateway, read stdin as the input, answered on stdout |
| `memdoor run --json "run go test ./... and report; change nothing"` | 2 | 15 events; `done` carried `⚠ nothing changed · ran: go test FAIL` |
| `memdoor workflow run check --wait` (test still broken) | 2 | stderr `failed · 1/2 done`, the failing task named |
| `memdoor run "fix Add in calc.go so TestAdd passes, then run go test ./..."` | 0 | `✓ checked: 1 file changed · go build PASS · go test PASS` |
| `memdoor workflow run check --wait` (green) | 0 | stderr `done · 2/2 done` |
| `memdoor resume --all \| head -4` | 0 | the list, the pipe closed cleanly |
| `memdoor resume --last \| head -6` | 0 | your line with `>`, the answer, the receipt |
| `memdoor --gateway 127.0.0.1:18799 providers` | 0 | a bare host:port reaches the gateway |
| `memdoor guards add --tool bash --pattern '\bgit push\b'` then `memdoor run "commit and push"` | 0 | committed, the push "blocked by a workspace tool guard", said so |
| `echo "" \| memdoor run "ask me whether the new file is a.go or b.go"` | 3 | `? Should the new empty file be named a.go or b.go? 1. a.go 2. b.go` on stderr |
| `memdoor cron add --id t --schedule "*/30 * * * *" --agent coder --message "say ok"`, `list`, `remove` | 0 | added, listed once, removed |
| `memdoor remote --last > link.txt` (12 October), the link opened from another terminal | 0 | the link alone on stdout; the other terminal showed the run's conversation and a turn typed there answered on the gateway; `/remote` in a window gave the same link; `remote off` revoked it |

The second row is the one to read twice: nothing changed and the last run
failed, so the pipeline sees 2. A failing test you did not touch is still a
failing test.

## In CI

The job is the installer, the key, and the command, in one step so the
gateway the first command starts is still there for the next. On GitHub:

```yaml
jobs:
  memdoor:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5          # whatever the project's checks need
        with: { go-version: stable }
      - name: Review the change and run the project's check workflow
        env:
          OPEN_ROUTER_API_KEY: ${{ secrets.OPEN_ROUTER_API_KEY }}
        run: |
          curl -fsSL "https://memdoor.ai/install.sh?ci=1" | MEMDOOR_CI=1 bash
          export PATH="$HOME/.local/bin:$PATH"
          git diff origin/${{ github.base_ref }}...HEAD \
            | memdoor run "review this diff; one finding per line; change nothing"
          memdoor workflow run check --wait
```

To finish a CI run from your phone, the job starts its gateway itself
(`memdoor gateway &`, waits for it with `memdoor gateway status`, then `memdoor run`), prints the link with `memdoor remote
--last` and `wait`s on the gateway: the link and its QR code are
in the job's log and the job stays there, the gateway holding the link, until
you are done or the job's timeout ends it; the full step is on the
[Remote control](/docs/remote) page. A runner's own address
is no door for a phone, so the job keeps its gateway to localhost
(`MEMDOOR_HOST: localhost` in the step's env) and the link goes through the
memdoor.ai relay: an account with Pro signed in on the runner, the token in
`MEMDOOR_BILLING_TOKEN` (or in `~/.memdoor/billing-token` when `memdoor
login` wrote it there).

The step fails when the review's last check fails (exit 2) or the workflow
run does, which is the point: the run's outcome is the job's. `?ci=1` keeps
the install out of the download count. The receipt behind this job is the
walk above and the same three commands on a home that had never seen
Memdoor (set up, answered, exit codes as shown); it has not been run on
GitHub's own runners yet. `memdoor run --json` piped into `jq` turns the
review into an annotation if you want one.

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

Into a pipe, the same command prints the conversation as text instead of
opening a window; no flag, the Unix rule:

```bash
memdoor resume 41af4ac5 | head -40
memdoor resume --last --follow     # text, and keep printing what the running turn does
```

Your lines start with `>`, the agent's are plain, a tool call is one `⏺`
line. `--gateway` takes a bare host or IP as well as a URL, so the same
commands reach a gateway on another machine: `memdoor --gateway
192.168.1.10 resume --all | head`.

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

Conversations and runs are on disk. A window reconnects on its own; a turn
that was mid-run may have finished while it was away, and its result is in
the saved messages, not replayed. A workflow run that was cut off continues
from its last finished task with `memdoor workflow resume <run-id>`.

## Next

- **[The gateway](/docs/gateway)** — the process every door opens onto.
- **[Scheduled checks](/docs/cron)** · **[Workflows](/docs/workflows)** · **[In your editor](/docs/editor)**
- **[Boundaries](/docs/boundaries)** — what a run may touch while you are away.
