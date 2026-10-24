# Memdoor Architecture

## Overview

Memdoor is a terminal coding agent written in Go, one binary (`cmd/cli`, module
`memdoor`). The gateway process owns the work: it runs the turns, holds the
sessions, the event log, the schedules, the workflow runs and the remote links,
and every way in is a door onto it: the window (`memdoor tui`), a headless turn
(`memdoor run`, `memdoor resume`), the editor (`memdoor acp`), a schedule
(`memdoor cron add`), a workflow (`memdoor workflow run`), the phone (`/remote`),
a box reached over `--gateway <host>` or `--gateway ssh://box`, and a peer
gateway's agent (`@box:coder`). Every door onto a gateway of your own is free;
memdoor.ai hosts what a single machine cannot (the relay behind a laptop's NAT,
the hosted workflow state). ADR-0021 is the record;
[`../architecture/GATEWAY_AND_DOORS.md`](../architecture/GATEWAY_AND_DOORS.md)
describes the process and each door.

Every model request goes through a provider on the person's own key
(`gateway/providers/factory.go`, ADR-0020): OpenRouter, Anthropic, OpenAI,
Gemini, xAI, Groq, DeepSeek, Baseten, a company AI gateway, or any provider
added with `memdoor connect` (`gateway/providers/registry.go`,
`~/.memdoor/providers.json`). There is no broker brain, no local model and no
GPU; a gateway with no key has no model. A decision model (Jev, `pkg/decision`,
ADR-0015; `gateway/decision_model.go` picks its provider on the same key) sits
in front of the chat model to cut what it reads and which tools a turn carries.
[`../architecture/MODEL_ROUTING.md`](../architecture/MODEL_ROUTING.md) has the
routing, the pins and the context-window data in full.

## System Architecture

```
  memdoor tui      memdoor run / resume / acp     cron, workflow      phone (/remote)
  (cmd/tui/ui)     (cmd/cli/cmd)                  (gateway/cron,      peers (@box:coder)
       │                 │                         workflow_runs.go)       │
       │  REST /api/* + WebSocket /ws              │                       │
  ┌────▼─────────────────▼─────────────────────────▼───────────────────────▼────┐
  │  Gateway (gateway/, :18789, binds 0.0.0.0 by default)                       │
  │  turn driver · tools · sessions · queue · A2A · event stream · remote host  │
  └──────────────┬────────────────────────────────────┬─────────────────────────┘
                 │ ~/.memdoor                         │ gateway/providers (HTTPS)
  ┌──────────────▼──────────────┐        ┌────────────▼────────────────────────┐
  │ SQLite data/memdoor.db      │        │ The provider's API, on your key      │
  │ sessions.json per agent     │        │ (OpenRouter, Anthropic, OpenAI, …)   │
  │ logs/events.db · skills/    │        └──────────────────────────────────────┘
  └─────────────────────────────┘
```

The web app (`web/`, embedded by `web/embed.go`, routes in
`gateway/spa_routes.go`) is the landing and docs site, sign-in and first-run
setup, the `/r/<id>` remote page and the `/s/<id>` share page; the coding
work happens in the terminal. memdoor.ai runs `memdoor-billing`
(`gateway/billingsvc`) from the same binary; it serves no model.

## Core Components

### The gateway process

`gateway/server.go` composes everything; `gateway/server_lifecycle.go`
registers the HTTP routes and the WebSocket (`/ws`, `gateway/server_websocket.go`)
and wraps them in the auth middleware from `pkg/authorization/middleware.go`.
`gateway/listen.go` decides the addresses a `/remote` link can be reached on;
`cmd/cli/cmd/gateway.go` binds `0.0.0.0` unless `MEMDOOR_HOST` or `--host`
says `localhost`. `memdoor gateway install` (`cmd/cli/cmd/gateway_service.go`)
makes it a launchd or systemd user service on a box that stays on.

### The turn

`gateway/agent_runtime_process.go` drives one turn: assemble the prompt,
run the tool loop (`gateway/agent_runtime_tools.go`), apply the honesty guards
(`gateway/agent_runtime_guards.go`, `turn_verdict.go`, `result_acceptance.go`),
persist. The decision model chooses the tool family per turn
(`gateway/tool_routing.go`), the reasoning effort (`gateway/turn_effort.go`)
and whether a question to the person is needed (`gateway/ask_gate.go`).
Approval mode (`gateway/approval.go`, `MEMDOOR_APPROVE=changes` or the
`approve` setting) asks before a tool that changes or runs something; it is
the one mode beside always-auto. The turn, end to end, is
[`../architecture/TURN_LOOP.md`](../architecture/TURN_LOOP.md).

### Tools

`tools/` holds the palette (`read_file`, `bash`, `write_file`, `edit_file`,
`apply_patch`, `glob`, `grep`, `locate`, `ask_user_question`, `todo_write`,
`sessions_spawn`, the judged readers `jgrep`, `jread`, `jlogs` in
`tools/jev_core.go`, the browser DSL in `tools/browser.go`); `cron` and
`workflow` are the gateway's (`gateway/cron_tool.go`, `workflow_tool.go`); the
person's MCP servers join as `mcp__<server>__<tool>` (`pkg/mcp`,
`gateway/mcp_tools.go`). Writes are rooted in the directory the window was
launched from (`gateway/agent_runtime_coder.go`); workspace `tool_guards`
(`gateway/tool_guards.go`) block a matching call with an error the model can
act on.

### Agents, sessions and the queue

Each agent has its own transcript per channel, keyed
`workspace:<ws>:channel:<ch>:agent:<id>` and saved by
`gateway/session_persistence.go`. `gateway/queue` serializes jobs per session
on lanes (`main`, `cron`, `subagent`, `nested`, `gateway/queue/lanes.go`);
[`QUEUE.md`](QUEUE.md) is the reference. A spawned run, a scheduled check or a
peer's reply wakes the conversation that waited on it as a turn of its own
(`gateway/server_jobs.go` `wakeConversation`, ADR-0022).
[`../architecture/MULTI_AGENT_ARCHITECTURE.md`](../architecture/MULTI_AGENT_ARCHITECTURE.md)
covers agents, mentions, spawned runs, wakes and peers.

### Workflows and schedules

A workflow is a DAG of agent tasks in `.memdoor/workflows/<name>/`, run on
mario's engine (`github.com/guregodevo/mario`); `pkg/workflow` is the agent
`TaskFactory` and callbacks, `gateway/workflow_runs.go` the Runner, and the
gateway keeps no task state of its own (ADR-0019,
[`../features/WORKFLOWS.md`](../features/WORKFLOWS.md)). Cron jobs live in
`gateway/cron/scheduler.go` with their store in SQLite; a scheduled workflow
is a cron job whose message is `/workflow run <name>`.

### Remote, share, peers and billing

`gateway/remote_host.go` hosts the phone's terminal in the gateway's own
process, so a `/remote` link outlives the window; `gateway/remote_relay.go` is
the sealed relay on memdoor.ai, used only when the gateway is kept to
localhost. `gateway/share_store.go` keeps a `/share` transcript as ciphertext
whose key never leaves the `#` fragment. `gateway/peers.go` and
`pkg/message/peers.go` run `@<peer>:<agent>` on another gateway and bring the
reply back into the thread (ADR-0023, experimental). `gateway/billingsvc`
owns sign-in on memdoor.ai, the plan (token to workspace to plan, `pkg/plan`),
the Pro seat through Stripe and the relay's plan check; never a model key.

## Storage Layout

Everything lives under `~/.memdoor` (`pkg/shared/paths.go`, `MemdoorHome`):

```
~/.memdoor/
├── config.json                 gateway configuration (gateway/config/loader.go)
├── data/memdoor.db             SQLite: workspaces, users, channels, messages, agents, cron,
│                               runs, turns, memories (pkg/repository/sqlite/factory.go)
├── logs/events.db              the event log memdoor logs query reads (gateway/logs)
├── workspaces/<ws>/channels/<ch>/agents/<agent>/sessions.json
│                               one transcript per agent per channel (session_persistence.go)
├── workspace/                  bootstrap files AGENTS.md, SOUL.md, TOOLS.md (gateway/workspace)
├── skills/                     the skill library, seeded at boot, disk wins (skills_seed.go)
├── providers.json  mcp.json  peers.json  remote-hosted.json  credentials.json
├── notes/  memory/  secrets/  master.key
└── meter.jsonl  savings.jsonl  the usage and savings ledgers (pkg/metering, pkg/savings)
```

A project adds `.memdoor/workspace` (the pinned workspace slug),
`.memdoor/workflows/` and `.memdoor/runs.db` (mario's run state, or the
hosted state on memdoor.ai for a Pro workspace, `gateway/hosted_state.go`).

## Key Design Patterns

**Domain-driven design.** `pkg/` is the domain (entities, value objects,
services, repository interfaces); `gateway/` is infrastructure and
composition. `pkg/` never imports `gateway/`:
`grep -rl '"memdoor/gateway/' pkg | grep -v _test.go` is empty. A domain
package that must call back into the gateway declares the interface it needs
where it is consumed (`SystemAnnouncementPoster` in `gateway/server.go`,
satisfied by `pkg/message/service.go`).

**Factories return interfaces and fail fast.** `pkg/repository/sqlite/factory.go`
returns `repository.RepositoryFactory`; `providers.ClientFactory` returns the
`pkg/llm` client interface; required fields are checked in the constructor.
Identities are named types parsed once at the boundary
(`pkg/channel/channel_id.go`, `pkg/shared/execution_context.go`):
[`../architecture/ALWAYS_VALID_PATTERN.md`](../architecture/ALWAYS_VALID_PATTERN.md),
[`../architecture/EXECUTION_CONTEXT.md`](../architecture/EXECUTION_CONTEXT.md).

**Configuration over code.** Agents are prompt plus palette; settings are
workspace-scoped and read per call
([`../architecture/CONFIG_SYSTEM_DESIGN.md`](../architecture/CONFIG_SYSTEM_DESIGN.md));
skills resolve disk-first ([`SKILLS.md`](SKILLS.md)).

**Event streaming.** A turn emits `lifecycle`, `assistant`, `tool` and
`context` events (`gateway/streaming/types.go`) that `gateway/broadcast`
fans out over the WebSocket to every client on the conversation; the window,
the ACP agent and `memdoor run --json` all read the same stream.

## Concurrency Model

One job at a time per session: `gateway/queue` gives every session a FIFO
lane, so two turns never write the same transcript, and jobs on different
sessions run in parallel across the lanes. `gateway/queue/modes.go` names what
a message arriving mid-turn does (`steer`, `followup`, `collect`,
`interrupt`, `queue`) and `cap.go` what happens when the queue is full. The
question broker (`gateway/question_broker.go`) turns a server-side
`ask_user_question` into a request over the event stream and a reply on the
WebSocket. Spawned runs are tracked in `gateway/run_tracker.go` and
`gateway/subagents/registry.go`, backed by SQLite. The guards that end a
runaway turn (loop-breaker, repeated reads) are in
`gateway/agent_runtime_guards.go`.

## Message Flow

A turn starts as a message: the window posts to `/api/messages`
(`cmd/cli/cmd/tui_wiring.go`) with its launch directory; `memdoor run`, the
ACP agent, a cron job, a workflow task and a peer's ask all reach the same
handler (`gateway/message_handlers.go`). The handler resolves the mentions
(`gateway/a2a/resolver.go`), saves the message, broadcasts it, and queues a
job per mentioned agent. The turn runs under `agent_runtime_process.go`;
its events stream over `/ws`; the answer is saved as a message (threaded under
the parent when the turn was a reply) and broadcast. A client joining mid-turn
sees the saved messages, then the live stream; there is no replay.

## Security

**Authentication.** A bearer token in the `Authorization` header (or
`?token=` on the WebSocket) names the user; tokens are random, stored hashed
(`pkg/auth/service.go`), validated by `pkg/authorization/middleware.go`. The
first account is made from the machine itself (`gateway/setup_handlers.go`).
`memdoor.ai` sign-in (email code, Google, GitHub, "Sign in with ChatGPT",
`gateway/auth_social.go`, `pkg/chatgpt`, `pkg/oidc`) is a separate account
system from a gateway's own users.

**Authorization.** Role-based access with channel membership
(`pkg/authorization`, `pkg/channel/membership.go`);
[`AUTHORIZATION.md`](AUTHORIZATION.md) lists the rules. The agent's filesystem
scope is `pkg/sandbox`
([`../architecture/AGENT_SANDBOX_SECURITY.md`](../architecture/AGENT_SANDBOX_SECURITY.md)).

**Doors.** The gateway binds the network by default; every door is
token-gated, and `MEMDOOR_HOST=localhost` closes it to the machine. A remote
or share link's key sits after `#` and never reaches a server.

**Egress.** Every host the binary can name is listed in
[`../SECURITY.md`](../SECURITY.md); `gateway/egress_test.go` fails the build
on a new one. On a vendor key nothing else leaves the machine.

**Limits and secrets.** Sign-in, registration and password reset are
token-bucket limited per client (`gateway/ratelimit`); secrets at rest are
AES-256-GCM under `~/.memdoor/master.key` (`pkg/secrets`).

## Context Window and Compaction

The window is the answering model's own (`gateway/providers/reference.go`).
`gateway/context/preflight.go` measures the turn before it is sent;
`gateway/agent_runtime_fit.go` and `gateway/compaction/` shrink a conversation
in one ladder, cheapest rung first: drop spent calls, stub superseded tool
output (the `recall` tool returns the original), summarize older messages,
keep the recent ones by tokens. The reshaped conversation is saved in the
transcript so the prompt prefix stays cached. `/context`, `/compact [focus]`
and `/handoff` are the person's handles (`gateway/session_compact.go`).

## Agent-to-Agent (A2A)

An `@mention` of an agent is a turn for that agent; an agent's reply that
mentions another is a turn for the next one (`gateway/a2a/handler.go`). The
mention depth stops the chain at two (`pkg/shared/execution_context.go`,
`maxMentionDepth`): the person, an agent by the person, an agent by an
agent. A mention nobody answers to fails the turn. Across gateways the same
mechanism carries `@<peer>:<agent>`; the peer's reply is resolved where the
thread is, and a turn asked by a peer wakes nobody on the peer
(`gateway/peers.go`, ADR-0023; [`../features/multi-agent-collaboration.md`](../features/multi-agent-collaboration.md)).

## Threading Model

A message is top-level (`parent_message_id` empty) or a reply to one parent.
A reply posted in a thread is delivered to the thread's lead agent
(`pkg/message/service.go`, `getThreadLead`); an agent answering a mention
replies in the thread that mentioned it, and a spawned run's announcement and
a peer's answer land in the thread that asked (`gateway/thread_handlers.go`).
[`../features/channels-and-threading.md`](../features/channels-and-threading.md)
describes the semantics.

## Workspaces and Multi-Tenancy

A workspace is the unit of settings, channels, agents and the plan. The CLI
resolves it by `-w`, `$MEMDOOR_WORKSPACE`, `.memdoor/workspace` walked up from
the current directory, then `config.json` (`cmd/cli/cmd/workspace_discovery.go`). Settings (`tool_guards`, `approve`,
`decision_tool_routing`, `decision_message_gate`) are per workspace
(`gateway/workspace_settings_handlers.go`). Channels and agents are scoped by
workspace ID in SQLite (`pkg/repository/sqlite/factory.go`). The plan is
owned by the billing service, token to workspace to plan; for a Pro workspace
the workflow state is one table per workspace on memdoor.ai
(`gateway/hosted_state.go`), shared by every project and person on it.

## Error Handling

A failed tool call returns its error as the tool result so the model can
adapt; the guards return a teaching error the same way. A tool panic is
recovered per call (`gateway/agent_runtime_tools.go`) and does not take the
gateway down. API errors are logged, emitted as an event and returned to the
client. An empty, unstreamed or truncated reply is repaired or retried by
`gateway/empty_reply.go`, `unstreamed.go` and `truncation_recovery.go`. The
event log writes through `gateway/logs`; when its database cannot be written
the logger falls back to stderr.

## Testing

`make test` runs `go test ./...`: unit and characterization tests beside the
code (`*_test.go`), the palette's contrast test (`cmd/tui/ui/palette_test.go`,
ADR-0018) and `gateway/egress_test.go` on the host list. Live checks go through
the CLI, never the database or raw HTTP (`memdoor run`, `memdoor messages`,
`memdoor logs query`, a TUI driven from tmux per `.agents/skills/test-tui.md`).

## References

- [CLI reference](CLI.md), [skills](SKILLS.md), [logs](LOGS.md), [queue](QUEUE.md), [cron](CRON.md), [authorization](AUTHORIZATION.md)
- [The gateway and its doors](../architecture/GATEWAY_AND_DOORS.md), [the turn loop](../architecture/TURN_LOOP.md), [model routing](../architecture/MODEL_ROUTING.md), [multi-agent architecture](../architecture/MULTI_AGENT_ARCHITECTURE.md)
- [ExecutionContext](../architecture/EXECUTION_CONTEXT.md), [configuration](../architecture/CONFIG_SYSTEM_DESIGN.md), [agent sandbox](../architecture/AGENT_SANDBOX_SECURITY.md), [always valid](../architecture/ALWAYS_VALID_PATTERN.md)
- [Where your data goes](../SECURITY.md), [client API](../developers/CLIENT_API.md), [gateway internals](gateway/README.md)
- ADRs in force: [0015 decisions through a decision model](../adr/0015-decisions-through-jev-on-openrouter.md), [0018 hue and luminance](../adr/0018-hue-is-what-luminance-is-how-much.md), [0019 workflows on mario](../adr/0019-workflow-runs-on-mario.md), [0020 every request through a provider](../adr/0020-every-request-through-a-provider.md), [0021 the gateway owns the work](../adr/0021-the-gateway-owns-the-work-every-door-is-free.md), [0022 a wake, not a heartbeat](../adr/0022-a-wake-not-a-heartbeat.md), [0023 peers](../adr/0023-peers-a2a-across-gateways.md)
- Features: [providers](../features/PROVIDERS.md), [decisions](../features/DECIDE.md), [workflows](../features/WORKFLOWS.md), [cron](../features/cron-jobs.md), [remote](../features/REMOTE.md), [peers](../features/PEERS.md), [ACP](../features/ACP.md), [MCP](../features/MCP.md)
