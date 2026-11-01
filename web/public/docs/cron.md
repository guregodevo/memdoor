# Scheduled checks

The agent comes back to something later instead of waiting for it. Asked
to watch CI, a deploy, a long build or a file that will appear, it
schedules a check:

```
cron(every: "20s", task: "is CI green?", times: 6)
```

The turn ends at once. Every twenty seconds the gateway runs the check in
your project, as the same agent. The answer lands in the window:

```
⏱ poll-9970000 — CI is still running (2 of 3 jobs done).
⏱ poll-9970000 — CI is green.
```

The answer wakes the agent that scheduled the check as a turn of its own
in the conversation: it can stop the job, carry on, or tell you. A run
with nothing to report answers `HEARTBEAT_OK`; the ⏱ note shows and nothing
is woken. An answer identical to the previous run's wakes nobody. A spawned
subagent's report wakes the conversation the same way.

The run that finds what it waited for stops the job. Otherwise the job
stops itself: at most the number of times given (default 10, never more
than 60), never more than 12 hours, never faster than every 10 seconds,
never twice at once.

![Every 20 seconds, 3 times: does the build pass? The ⏱ answers land in the window](/demo-cron.cast)

`memdoor cron list`: what is scheduled, with run counts. `memdoor cron
remove <id>`: stop one from the shell. The agent can stop any job.

## Jobs you schedule

```bash
memdoor cron add --id nightly --schedule "0 9 * * *" --agent coder --message "Summarise yesterday's commits"
memdoor cron history
```

Schedule: standard crontab, a leading seconds field, `@daily`, or `@every
90s`. A job you add runs in the agent's default directory and answers in
its own session and the log. Suited to a summary or a triage, not
unattended code-writing.

## Next

- **[Headless, then attached](/docs/headless)** — leave a run alone and come back into it live.
