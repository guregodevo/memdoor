# Headless, then attached

Every run is a conversation on your gateway, whether a window was open or not:
a turn from the window, the phone or the editor, a schedule, a workflow from a
shell or CI, a spawned child. `memdoor resume` opens any of them. A run can be
left alone and rejoined live instead of killed and started over.

## The rule

Every command is headless except `memdoor tui`, the window. `memdoor resume`
opens the window at a terminal and prints text into a pipe. No flag decides
it. `memdoor run`, `workflow run`, `cron add`, `guards` and the rest take a
prompt or arguments and write text or events. Each returns an exit code. Stdin
is the input when it is a pipe or a file. A terminal is never read.

The gateway is a background process on the machine the run belongs to, your
laptop or a CI runner. Nothing else is needed. `memdoor run`, `workflow run`,
`workflow resume`, `cron add`, `resume`, `tui` and `acp` start one when none
is running and set the machine up the first time. A run that must survive a
closed laptop belongs on [a box that stays on](/docs/gateway): run `memdoor
gateway install` there and your laptop, ssh and your phone are doors onto it.

## Runs that need no window

- **A scheduled check.** Inside a turn the agent schedules a re-check
  (`cron(every: "20s", task: "is CI green?", times: 6)`) and the turn ends.
  From the shell, `memdoor cron add --id nightly --schedule "0 6 * * *"
  --agent coder --message "…"` runs a turn on a schedule. `--workflow
  <name>` runs a workflow on one. See [Scheduled checks](/docs/cron).
- **A workflow from the shell or CI.** `memdoor workflow run <name>` starts
  the project's workflow and prints the run. `memdoor workflow status
  <run-id>` shows each task's state and `history <name>` what earlier runs
  did. On a CI runner the job is the installer, the key in the environment
  and that command with `--wait`: it starts the gateway, sets the runner up
  and exits with the run's outcome. On GitHub that is one step,
  `uses: guregodevo/memdoor-action@v1` with the workflow's name and your key. `pr-triage`, `pr-review`, `security-review`, `write-tests`,
  `pr-description`, `explain-failure`, `issue-triage`, `fix-tests` and
  `release-notes` are built in and need no file in the repo. See
  [Workflows](/docs/workflows).
- **An editor session.** The coder in VS Code, Zed or JetBrains is a
  conversation on the same gateway. See [In your editor](/docs/editor).
- **The phone, from a script.** `memdoor remote <id>` prints the link
  `/remote` would, on stdout, and returns. The gateway holds the link, so the
  run a script or a CI job started is on your phone for as long as that
  gateway runs. See [Remote control](/docs/remote).

Each answers into a conversation. A check's answer wakes the conversation
that asked for it, so the agent acts on it without you.

## A turn from a script

`memdoor run` is one coder turn with no window. The prompt comes from the
arguments or stdin, the answer goes to stdout, everything else to stderr, and
the exit code is the turn's receipt.

```bash
memdoor run "make the retry wait configurable and add a test"
git diff | memdoor run "review this diff; one line per finding"
memdoor run --json "fix the failing test" | jq -r 'select(.event=="done") | .receipt'
```

With both arguments and a pipe, what is piped in is the input the prompt is
about. `--json` streams NDJSON events (`text`, `tool_start`, `tool_done`,
`question`, `done`). The `done` event carries the conversation id, the receipt
and the exit code. The codes:

| Exit | Meaning |
|---|---|
| 0 | the turn ended and what it changed was checked, or nothing needed a check |
| 1 | the gateway, the provider or the turn failed |
| 2 | it ended unverified: a change with no check after it, or a failing check |
| 3 | in a pipe, the agent asked a question nobody could answer and took the default; `--yes` answers the first option instead. At a terminal you answer it there. |
| 130 | ctrl-c or a TERM signal ended it; the conversation keeps what ran, `memdoor resume <id>` continues it |

![A turn from a script: the answer on stdout, the exit code, then the same run opened in a window](/headless.cast)

Each run is a conversation. Its id is on stderr at the end, and `memdoor
resume <id>` opens it in a window. `--conversation <id>` continues one.

`memdoor workflow run <name> --wait` is the same contract for a workflow:
progress on stderr, the files the run produced on stdout, exit 0 done, 2
failed, 1 stopped. A gate is reported and waited on. `--stop-at-gate` ends it
at a gate instead, exit 3 with the gate's review on stderr, for a job with
nobody to approve. The run stays waiting on the gateway.

## A walk through the doors

The commands below were run in this order on 11 October 2026, against a
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

In the second row nothing changed and the test still failed, so the exit is 2.

## In CI

On GitHub it is one step, `uses: guregodevo/memdoor-action@v1`, with a
workflow (one of the nine built in, or yours) and your key or a model inside
the runner. The run's outcome is the job's. The page: [In CI](/docs/ci).
Elsewhere the same run is three shell lines:

```bash
curl -fsSL "https://memdoor.ai/install.sh?ci=1" | MEMDOOR_CI=1 bash
export PATH="$HOME/.local/bin:$PATH"
memdoor workflow run pr-review --wait --stop-at-gate
```

To finish a CI run from your phone, the job starts its gateway
(`memdoor gateway &`, `memdoor gateway status` until it answers, then `memdoor
run`), prints the link with `memdoor remote --last` and `wait`s on the
gateway. The link and its QR code are in the job's log, and the job stays up
until you are done or its timeout ends it. The full step is on the
[Remote control](/docs/remote) page. A runner's address is not reachable from
a phone, so the job keeps its gateway to localhost (`MEMDOOR_HOST: localhost`
in the step's env) and the link goes through the memdoor.ai relay. That needs
the hosted gateway's account token on the runner: `MEMDOOR_BILLING_TOKEN`, or
`~/.memdoor/billing-token` written by `memdoor login`.

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
opening a window. No flag decides it:

```bash
memdoor resume 41af4ac5 | head -40
memdoor resume --last --follow     # text, and keep printing what the running turn does
```

Your lines start with `>`. The agent's are plain. A tool call is one `⏺` line.
`--gateway` takes a bare host or IP as well as a URL, so the same commands
reach a gateway on another machine: `memdoor --gateway 192.168.1.10 resume
--all | head`.

The window opens on that conversation with its saved messages in view and the
same session underneath. Your next line is the next turn of that run. If a
turn is running when you attach, you see it from that moment on, tool frames
and answer, and `Esc` interrupts it. What it did before is in the saved
messages, not replayed as frames. `memdoor resume <id>` takes the id a
run printed, from any directory. Without `--all` the list is this folder's
conversations.

## A run waiting on you

A workflow gate shows ⏸ with the diff. Approve from the window (`a`) or from
anywhere:

```bash
memdoor workflow approve <run-id> <task>
memdoor workflow changes <run-id> <task> "the notes miss the migration"
```

`changes` reruns the task with your comment and waits again. A failed run is
resumed at the failed step with `memdoor workflow resume <run-id>`. What it
finished is skipped.

## From your phone

`/remote` in a window gives a link that is the terminal itself, end to end
encrypted, so a run can be watched and answered from a phone. See [Remote
control](/docs/remote).

## Letting a turn run

A turn judged unfinished by its receipts continues until it is shown done or
a token budget is spent. The budget is what makes an unattended turn safe to
leave. See [the harness](/docs/no-babysitting).

## If the gateway restarts

Conversations and runs are on disk. A window reconnects. A turn
that was mid-run may have finished while it was away; its result is in the
saved messages, not replayed. A workflow run that was cut off continues from
its last finished task with `memdoor workflow resume <run-id>`.

## Next

- **[The gateway](/docs/gateway)** — the process every door opens onto.
- **[Scheduled checks](/docs/cron)** · **[Workflows](/docs/workflows)** · **[In your editor](/docs/editor)**
- **[Boundaries](/docs/boundaries)** — what a run may touch while you are away.
