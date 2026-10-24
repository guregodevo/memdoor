# Logs

What the gateway logged, queried from the command line. The gateway writes
every event to one SQLite file, `~/.memdoor/logs/events.db`, and `memdoor
logs` reads it over the gateway's API, so the same commands work against a
box (`memdoor --gateway <box> logs errors`). There is no text log to tail:
the event store is the log.

`memdoor logs` never reads the database file itself and never needs it to be
on this machine; it asks the gateway. A gateway that is down answers nothing,
and `memdoor gateway status` says so.

## The six commands

| Need | Command |
|---|---|
| What went wrong lately | `memdoor logs errors --since 5m` |
| Lines matching a pattern | `memdoor logs query --regex "timeout\|refused" --since 1h` |
| A conversation, turn by turn | `memdoor logs session <session-id>` |
| One event and what caused it | `memdoor logs trace <event-id>` |
| How many events, by level and source | `memdoor logs stats` |
| Reclaim disk | `memdoor logs prune --days 7` |

Every command takes the global flags: `--gateway` for a gateway elsewhere,
`-w` for the workspace, `-v` for verbose output.

### logs errors

```
memdoor logs errors [--since 5m] [--limit 20]
```

The most recent WARN and ERROR events, one line each with the time, the
`[component]` that emitted it and the message. `--since` takes a Go duration
(`5m`, `1h`, `24h`) and defaults to 24h; `--limit` defaults to 20.

### logs query

```
memdoor logs query [--regex <re>] [--since <dur>] [--limit 50] [--order asc|desc]
                   [--session <id>] [--run <id>] [--workspace-filter <slug>] [--data]
```

One line per event: time, level, `[component]`, message. `--data` adds the
event's id, its `session=` and `run_id=`, and every data attribute, which is
how you find the ids the other flags and `logs trace` take:

```
$ memdoor logs query --regex "Tool call failed" --since 10m --data
14:10:24.512 WARN  [Agent]      Tool call failed: bash — exit status 1
    id=0193f2a1-…
    session=workspace:acme:channel:test:agents:coder
    run_id=run_7f3a
    tool=bash
```

`--regex` is Go's RE2 dialect: character classes, quantifiers, anchors,
groups and alternation, `(?i)` for case-insensitive, no backreferences and no
lookaround. Matching is linear in the input, so a pattern can never hang the
gateway. `--order asc` reads oldest first; `--workspace-filter` keeps the
events a workspace-scoped handler tagged with that slug.

### logs session

```
memdoor logs session <session-id>        # or --session-id <id>
```

The timeline of one conversation as logged: each turn, each tool call and its
result, each error, in order. The id is the `session=` that `logs query
--data` shows.

### logs trace

```
memdoor logs trace <event-id>            # or --event-id <id>
```

Every event has a parent (what triggered it) and a root (the event that
started the chain). `trace` prints the chain around one id: the root, the
path down to this event, its children, and the depth.

### logs stats

```
memdoor logs stats
```

Total events, the count per level, the oldest and newest timestamps, and the
database and index sizes on disk.

### logs prune

```
memdoor logs prune --days 30      # events older than 30 days
memdoor logs prune --before 720h  # the same, as a Go duration
memdoor logs prune --all          # every event
```

Exactly one of the three. Deletes the events and runs `VACUUM` so the freed
pages return to the OS. The gateway also prunes on its own, every six hours,
keeping 30 days (`gateway/logs/retention.go`); `prune` is the manual escape
hatch when the disk is full now.

## What an event is

Every event the gateway writes has the same fields:

```
id          unique id (what logs trace takes)
timestamp   when it was emitted
type        one of the event types below
component   who emitted it: Agent, WebSocket, HTTP, Cron, Workflow, …
level       DEBUG, INFO, WARN or ERROR (DEBUG only with the gateway's --verbose)
session     the conversation it belongs to, when there is one
run_id      the turn it belongs to, when there is one
parent_id   the event that caused this one
root_id     the first event of the chain
message     the text logs query prints
data        structured attributes (logs query --data prints them)
error       the error, when the event is a failure
duration    nanoseconds, on an event that closes a span
success     true or false, on an event that closes a span
```

The event types (`gateway/logs/event.go`):

- lifecycle: `agent_started`, `agent_stopped`, `session_started`,
  `session_ended`, `run_started`, `run_completed`;
- messages: `message_received`, `message_sent`;
- tools: `tool_called`, `tool_completed`, `tool_failed`;
- processing: `thinking_started`, `thinking_finished`,
  `compaction_started`, `compaction_completed`;
- state: `state_change`, `error`, `metric`, `queue`, `websocket`.

## Writing to the log, from Go

Code in the gateway logs through `gateway/logs`, never through `fmt.Println`
and never through a bare `slog` handler (the centralized logger is what gives
an event its parent, root, session and run):

```go
log := logs.New("Agent")                       // one logger per component

log.Info("Tool called", slog.String("tool", name))
log.Warn("Tool call failed", slog.String("tool", name), slog.String("error", err.Error()))

turn := log.WithSession(sessionKey).WithRun(runID)   // every event below carries both
span := turn.StartSpan("bash", logs.EventToolCalled)  // a child logger with timing
// … work …
span.Finish()                                          // or span.FinishWithError(err)
```

Reading it back from Go, when a command needs more than the CLI prints:

```go
q := logs.NewQueryBuilder().Since(time.Hour).Levels(logs.LevelError).Limit(50).Build()
result, err := storage.QueryEvents(ctx, q)          // storage: logs.GetGlobalStorage()
chain, err := storage.TraceChain(ctx, eventID)
events, err := storage.ReconstructSession(ctx, sessionID)
stats, err := storage.GetStats(ctx)
```

`QueryBuilder` also has `Components(...)`, `Session(id)`, `RunID(id)`,
`MessageRegex(re)`, `Workspace(slug)` and `ErrorsOnly()`.

## Where it lives

- `~/.memdoor/logs/events.db`, with its `-wal` and `-shm` files beside it.
  About 500 bytes per event. Writes are buffered (100 events or one second).
- One gateway per database: a second gateway started on the same home
  shows up as "database is locked"; `memdoor gateway status` names the one
  that is running.
- The reader is the gateway's `/api/logs/*` handlers; a WARN or ERROR also
  reaches memdoor.ai's monitoring inbox when `MEMDOOR_TELEMETRY_ENABLED=1`
  (off by default).

## When a query finds nothing

- No events at all: the gateway has not started since the install, or it is
  another gateway you are asking (`--gateway`, `MEMDOOR_GATEWAY`).
- Nothing in the window: widen `--since`; the default is the whole store.
- The pattern: try it without `--regex` first, then add it back; remember
  RE2 has no `\1` and no `(?=`.
- A line you expected at DEBUG: the gateway keeps DEBUG only when started
  with `--verbose`.
