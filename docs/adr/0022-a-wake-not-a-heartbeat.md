# ADR-0022: A wake, not a heartbeat

Date: 2026-10-05 · Status: accepted, in force. Record:
`gateway/server_jobs.go` (`wakeConversation`), `tools/subagent_tools.go`,
`docs/features/cron-jobs.md`, `docs/features/WORKFLOWS.md`.

## Context

The first design gave every agent an hourly checklist: a runner, a table, an
API, a tool and a `HEARTBEAT.md`, with the agent reading its own list on a
timer. It woke agents with nothing to say, produced answers nobody had
asked for, and put the schedule in a place the conversation could not see.
The alternative, taken from OpenClaw: nothing wakes on a timer; a
conversation is woken by what it was waiting on.

## Decision

- **A conversation is woken by what it waited for**, as a turn of its own
  in that conversation (`wakeConversation`): a spawned run's report, a
  scheduled check's answer, a peer's reply (ADR-0023).
- **A check with nothing to report wakes nobody.** It answers
  `HEARTBEAT_OK`; an answer identical to the last run's wakes nobody either.
- **The schedule belongs to the conversation.** The coder has a `cron`
  tool: it schedules its own re-check ("every 20s, 6 times: is CI green?")
  instead of sleeping in bash; each run answers into the window that asked,
  runs in that project, and stops itself at its bound. `memdoor cron add`
  does the same from the shell, and `--workflow <name>` schedules a
  workflow run.
- **Subagents are the same mechanism.** `sessions_spawn` runs a
  self-contained subtask in its own session, in the same project; its
  result wakes the conversation that spawned it, and while it runs the
  status line shows its beats.
- **A woken turn inherits the turn before it**: the project directory and
  the permission mode (`asyncTurnContext`, `pkg/message/peers.go`), never
  the scratch directory.
- **The hourly checklist is deleted**: its runner, table, API, tool,
  settings and `HEARTBEAT.md`.

## Consequences

- Nothing runs on a timer that no conversation asked for; the log shows a
  wake with its cause.
- A wake is a turn like any other: it goes through the queue, the approval
  gate and the guards, and a window on the conversation sees it stream.
- A wake that arrives while the gateway is down is lost with the gateway's
  events; a scheduled check runs again at its next time, a spawned run's
  report is in its session.
