# Scheduled checks

The agent can come back to something later instead of waiting for it now.

Ask it to watch CI, a deploy, a long build, or a file that is going to
appear, and it schedules a check instead of sleeping:

```
cron(every: "20s", task: "is CI green?", times: 6)
```

Your turn ends at once. Every twenty seconds the gateway runs the check — in
your project, as the same agent — and the answer lands in your window:

```
⏱ poll-9970000 — CI is still running (2 of 3 jobs done).
⏱ poll-9970000 — CI is green.
```

The answer does more than show: it wakes the agent that scheduled the check,
as a turn of its own in your conversation, so it can act on it — stop the
job, carry on with the next step, tell you. A run with nothing to report yet
answers `HEARTBEAT_OK`: the note shows, nothing is woken; so does an answer
identical to the previous one. (A spawned subagent's report wakes the
conversation the same way.)

The run that finds what it waited for stops the job. If nothing does, the
job stops itself: it runs at most the number of times it was given (default
10, never more than 60), never for more than 12 hours, never faster than
every 10 seconds, and never twice at once.

![Every 20 seconds, 3 times: does the build pass? The ⏱ answers land in the window](/demo-cron.cast)

`memdoor cron list` shows what is scheduled, with each job's run count;
`memdoor cron remove <id>` stops one from the shell, and the agent can stop
any job itself.

## Jobs you schedule

```bash
memdoor cron add --id nightly --schedule "0 9 * * *" --agent coder --message "Summarise yesterday's commits"
memdoor cron history
```

Standard crontab, a leading seconds field, `@daily`, or `@every 90s`. A job
you add runs in the agent's default directory and answers in its own
session and the log — good for a summary or a triage, not for unattended
code-writing.
