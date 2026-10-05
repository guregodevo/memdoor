# Scheduled checks and cron jobs

Two things run on a schedule: a check the **agent** sets for itself, and a
job a **person** configures. Both run on the gateway's scheduler
(`gateway/cron`, SQLite-backed), both show in `memdoor cron list`, and both
are bounded the same way. Research and the live receipt:
`../internal/HOOKS_AND_CRON_2026_10.md` (removed 2026-10-03).

## The agent's own checks: the `cron` tool

Waiting inside a turn burns the turn — a `sleep` holds the model, the window
and the person. So the coder has a tool that hands the time back:

```
cron(action: "every", every: "20s", task: "is CI green? run gh run list", times: 6)
```

The turn ends at once. Twenty seconds later the gateway starts a fresh turn
with that task, **in the project the first turn ran in, as the same agent**,
and its answer lands in the window that asked, as a note:

```
⏱ poll-9970000 — READY is still missing (test -f READY returned MISSING).
⏱ poll-9970000 — READY exists.
```

The run that finds what it waited for stops its own job
(`cron(action: "stop", id: "poll-9970000")`); otherwise the job stops itself
at its bound.

**The answer wakes the conversation.** The note is for you; the answer is
also handed to the agent that scheduled the check, as a turn of its own in
that conversation, so it can act on it — stop the job, go on with the next
step, tell you. A run with nothing to report yet answers `HEARTBEAT_OK`: the
note shows, nobody is woken. An answer identical to the previous run's wakes
nobody either. This is the heartbeat, as OpenClaw has it: a conversation is
woken by what it was waiting on — a scheduled check, or a spawned subagent's
report — and keeps no checklist of its own.

**Bounds, none of them optional.** Every account of a runaway agent loop ends
with the same fix — a counter, not a better prompt — so:

- `times`: how many runs, default 10, at most 60; the job is removed the
  moment its last run completes;
- an expiry of 12 hours from creation, whatever `times` says;
- an interval between **10 s and 24 h**: `every 1s` is refused in words
  (`1s is too often — 10s is the shortest interval`) and nothing is scheduled;
- no overlapping runs: a tick that arrives while the previous run is still
  going is skipped, so a slow check never piles up beside itself.

`cron(action: "list")` shows every job with its run count — `(run 2 of 6)` —
and `cron(action: "stop", id: …)` cancels **any** job, not only the agent's
own. A failing check is still a run: it reports the exact error, counts, and
ends at its bound.

The tool is offered on turns that investigate or wait (tool routing's
`investigate` family), not on every edit, so its schema is not paid for on a
turn that just writes code. It is in the coder's palette; another agent gets
it by adding `cron` to its tools.

## A person's jobs: `memdoor cron`

```bash
memdoor cron add --id nightly --schedule "0 9 * * *" --agent coder --message "Summarise yesterday's commits"
memdoor cron list        # every job: id, schedule, agent, enabled, message
memdoor cron remove nightly
memdoor cron history     # each run: time, duration, ok or the error
memdoor cron stats
```

Schedules are 5-field crontab, 6-field with a leading seconds field,
`@hourly`/`@daily`/`@weekly`, or `@every 90s`. Jobs persist across restarts.

A job added this way has no workdir and no window: it runs in the agent's
default directory and its answer goes to the job's own session and the log
(`memdoor logs query --regex "Job completed"`). That is the shape the
research says survives — read-heavy, low-stakes work: a summary, a triage,
a docs-drift check — not unattended code-writing.

## How it works

- `domain.CronJob` carries `workdir`, `session_key`, `max_runs`, `run_count`
  and `expires_at` (a migration adds the columns to an older database); a
  person's job leaves them empty.
- `gateway/cron_tool.go` builds the tool **per turn** with the turn's
  directory, session and agent (read from the context — the shared runtime's
  own config is the default profile, which has no shell).
- `gateway/server_jobs.go` resolves the job's agent against the stored
  buddies first, so the turn gets that agent's palette; the scheduler's own
  resolver only knows config agents.
- Every run logs `Executing cron job` and `Job completed` at Info, and the
  removal logs `Cron job finished: ran 6 of 6 times`. A run nobody can see is
  a run that did not happen.
- A run's answer reaches the window through a session broadcast
  (`cron_answer`), which the TUI shows as a note — not a turn: no spinner, no
  `esc to interrupt`.
