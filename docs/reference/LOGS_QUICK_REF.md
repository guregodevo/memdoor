# Logs, the short version

The gateway's event log, read over its API. Full page:
[LOGS.md](LOGS.md).

| Need | Command |
|---|---|
| Recent errors | `memdoor logs errors --since 1h` |
| A pattern | `memdoor logs query --regex "timeout" --since 24h` |
| The ids behind the lines | `memdoor logs query --regex "failed" --data` |
| One turn's events | `memdoor logs query --run <run_id>` |
| One conversation | `memdoor logs session <session-id>` |
| What caused an event | `memdoor logs trace <event-id>` |
| Counts, levels, size | `memdoor logs stats` |
| Free disk | `memdoor logs prune --days 7` |
| A gateway elsewhere | `memdoor --gateway <box> logs errors` |

`logs query` flags: `--regex`, `--since`, `--limit` (50), `--order asc|desc`,
`--session`, `--run`, `--workspace-filter`, `--data`. The regex is RE2:
`(?i)` for case-insensitive, no backreferences, no lookaround.

```bash
# the id of the last error, then its chain
memdoor logs errors --since 1h --limit 1
memdoor logs query --regex "<that message>" --data | grep id=
memdoor logs trace <id>
```

Where: `~/.memdoor/logs/events.db`. The gateway keeps 30 days on its own;
nothing is written by anything but the gateway, and nothing tails it live.
