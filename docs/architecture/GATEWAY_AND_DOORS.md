# The gateway and its doors

The gateway is the process that owns the work. Every run is a conversation on
it; the window, a script, a schedule, an editor, a phone and another gateway
are doors onto that one process, which is why a run started through one door
can be watched, answered or finished through another. This page is the
architecture: what the process is, what it holds, each door's code path, the
gate on every door, and what memdoor.ai hosts for it. The user-facing guides
are `web/public/docs/gateway.md`, `docs/features/REMOTE.md` and
`docs/features/PEERS.md`.

```
                 memdoor tui ──── /ws: turns in, events out, the workdir on every turn
                 (laptop, --gateway box: /api/fs reads the box's files)
                        │
  memdoor run ──────────┤  /api/messages + /ws              phone  /r/<id>#k=<key>
  memdoor resume        │  exit code = the receipt           │  sealed frames, xterm.js
  memdoor acp (editor)  │                                    │  /api/relay/browser
                        ▼                                    ▼
  ssh://you@box ─ tunnel ─►  ┌──────────────────────────────────────┐ ◄─ /api/relay (its own
  (sshBoxSetup installs      │  memdoor gateway   :18789  0.0.0.0    │    terminal end, hosted
   a gateway on a bare box)  │                                      │    by this process)
                             │  conversations, turns, tools         │
  @box:coder ──── peers ───► │  ~/.memdoor/data/memdoor.db          │ ──► the provider on the
  (the reply lands in this   │  ~/.memdoor/logs/events.db           │     person's own key
   thread, A2A)              │  workspaces/…/sessions.json          │
                             │  remote-hosted.json, peers.json      │ ──► memdoor.ai only for:
  cron ── a schedule wakes ─►│  cron scheduler, workflow runs       │     the relay of a localhost
  a conversation of its own  └──────────────────────────────────────┘     gateway, the hosted
                                every door: a bearer token                 workflow state, /share
```

## The process

One binary, one command. `memdoor gateway` (`cmd/cli/cmd/gateway.go`,
`gatewayCmd`) probes `/health` on its host and port first and leaves a
running gateway alone, registers the two callbacks the process provides to
the `gateway` package (`gateway.SetRemoteHost`, `gateway.SetPeerCaller`), and
calls `gateway.Start`. The port is `--port`, 18789 by default. The bind
address is `--host`, defaulting to `MEMDOOR_HOST` and then to
`defaultBindHost = "0.0.0.0"` (`gateway.go`): the gateway listens on the
machine and its network, and the banner names the network addresses a phone
can dial (`networkURLs`, built on `gateway.ListenAddresses` in
`gateway/listen.go`, private addresses first, then a VPN's, then the rest).
`MEMDOOR_HOST=localhost` keeps it to the machine. `/health` reports where it
listens (`health.ListenHealth` in `gateway/server_http.go`), which is how a
window later decides where a phone's link can live.

Nobody starts it by hand on a laptop. The first command that needs it
(`firstRunReady` in `cmd/cli/cmd/tui_firstrun.go`, called by `memdoor tui`,
`run`, `resume`, `acp`, `workflow run`, `cron add`) finds no gateway, and
`ensureGatewayRunning` in `cmd/cli/cmd/setup.go` spawns `memdoor gateway
--port <port>` detached, with its output in `~/.memdoor/gateway.log` and its
pid in `~/.memdoor/gateway.pid`. That gateway lives as long as the login
session. On a machine that stays on, `memdoor gateway install`
(`cmd/cli/cmd/gateway_service.go`, `installService`) writes a unit for the
user's service manager and starts it: `~/.config/systemd/user/memdoor.service`
on Linux (`Restart=always`, `EnvironmentFile=-~/.memdoor/env`,
`WantedBy=default.target`), `~/Library/LaunchAgents/ai.memdoor.gateway.plist`
on macOS (`RunAtLoad`, `KeepAlive`); both run `<binary> gateway --port N
--host H`. No root. `memdoor gateway status` asks `/api/status`, prints the
version, the pid and who manages the process (`serviceState`, or the pid
file), and exits 1 when nothing answers, so a script can wait on it.

The gateway speaks plain HTTP; `scripts/install.sh` reads `MEMDOOR_PORT` for
the port it probes and restarts, and the ssh door passes the same variable
to the installer it runs on a box.

## What it holds

Everything a door shows is on the gateway's disk, under `~/.memdoor`
(`shared.MemdoorHome`):

| What | Where | Code |
|---|---|---|
| Workspaces, users, channels, messages, agent events, cron jobs and history, subagent runs, turns, memories, secrets, files | `data/memdoor.db` | `pkg/repository/sqlite/factory.go` `NewSQLiteFactory`, one repository per aggregate behind `repository.RepositoryFactory`; the default path is set in `gateway/server.go` and `gateway/config/loader.go` |
| The log store every `memdoor logs` command reads | `logs/events.db` | `gateway/logs/logger.go` `NewEventLoggerWithSQLite`, `sqlite_storage.go`; pruned by retention (`gateway/server_lifecycle.go`, 30 days by default) and served at `/api/logs/*` |
| The model's view of each conversation | `workspaces/<ws>/channels/<ch>/agents/<agent>/sessions.json`, `workflow-sessions/<id>.json`, `subagents/<id>.json` | `gateway/session_persistence.go` `resolveSessionStorePath`, `SaveConversation`, `LoadMessages` |
| The phone links this process hosts | `remote-hosted.json` (0600) | `gateway/remote_host.go` `remoteHosted`, revived at start by `revive()` |
| The peers this gateway may ask | `peers.json` (0600) | `gateway/peers.go` `LoadPeers`, `SavePeers` |
| Providers and keys | its environment, `providers.json` from `memdoor connect` | `gateway/providers/registry.go` |

Schedules and workflow runs are state too. The cron scheduler
(`gateway/cron`, built in `gateway/server.go` on `repoFactory.CronJobs()`)
runs `Server.executeCronJob` in `gateway/server_jobs.go`; a job whose message
is `/workflow run <name> [partition]` starts that workflow in the job's
project (`scheduledWorkflow`, `startWorkflow` in `gateway/workflow_runs.go`),
and the agent's own `cron` tool schedules on the same scheduler. A workflow's
task state is mario's repository (`pkg/workflow` hands it callbacks and keeps
none), in the project's `.memdoor/runs.db`, or on memdoor.ai for a Pro
account (`gateway/hosted_state.go`, below).

## The doors

Each door is a client of the same HTTP and WebSocket surface
(`gateway/server_lifecycle.go` registers every route). A turn is posted as a
message; its events stream back on `/ws`; the working directory travels with
the turn (`server_websocket.go` reads `workdir` from each message), so the
coder writes where the door says.

**The window.** `memdoor tui` (`cmd/cli/cmd/tui.go`; the bare `memdoor`
opens it too) runs the Bubble Tea program in `cmd/tui/ui`.
`cmd/tui/ui/websocket_adapter.go` dials `/ws` through the shared client
`gateway/client/websocket.go` (`client.New`), with the session token and the
launch directory; `cmd/cli/cmd/tui_wiring.go` builds the URL. The gateway
validates the token before upgrading the socket
(`Server.handleWebSocket` in `gateway/server_websocket.go`).

**The window on a laptop, the project on a box.** `--gateway <box>` (or
`MEMDOOR_GATEWAY`; `cmd/cli/cmd/root.go` accepts a URL, `host:port`, a bare
host, or `ssh://`) points every command at another machine's gateway. The
project is then there, so the window's files panel, `@`-mentions and `/dir`
read through `remoteFS` in `cmd/cli/cmd/remote_fs.go`: `GET
/api/fs?op=stat|list|read`, answered by `ChatServer.handleFS` in
`gateway/fs_handlers.go` for the authenticated user with the same walk and
preview as the local panel (`pkg/filelist`). `--dir` names the directory on
the box; `remoteProjectDir` refuses one the gateway does not have, and
`remoteDefaultDir` falls back to the same path as here when the box has it,
else its home. `memdoor run --dir`, `workflow run` and `resume --last` use
the same resolution.

**ssh.** `--gateway ssh://[user@]box[:port]` (`cmd/cli/cmd/gateway_ssh.go`)
opens a tunnel for the length of the command: `openSSHDoor` runs `ssh -T -L
<local>:127.0.0.1:<gateway port> box cat` in batch mode, so the tunnel ends
when the command's stdin pipe closes, and the command talks to
`http://127.0.0.1:<local>`. Nothing on the box is opened to the network.
When no gateway answers through the tunnel within `sshFirstLook`,
`sshBoxSetup` looks at the box over the same ssh: no memdoor there, and it
runs the installer from memdoor.ai into `~/.local/bin`; no gateway running,
and it runs `memdoor gateway install --port <port>` there, or starts the
gateway with `nohup` where there is no service manager. Then the typed
command runs. `?port=18799` names a gateway on another port on the box.

**Headless.** `memdoor run` (`cmd/cli/cmd/run.go`, `headlessRunCmd`) takes
the prompt from its arguments or stdin, opens a conversation and runs one
turn through `gatewayRunner` (`cmd/cli/cmd/acp.go`: `NewConversation`,
`Run`), which posts the turn and reads its events off a `/ws` feed. Text
streams to stdout, `--json` streams NDJSON events, and the exit code is the
turn's receipt: 0 checked, 1 failed, 2 unverified, 3 a question nobody
answered (`runExitCode`), 130 interrupted. `memdoor resume`
(`cmd/cli/cmd/resume.go`) reopens a recorded conversation in the window, or,
into a pipe or with `--follow`, as a transcript (`resume_headless.go`
`resumeHeadless`). A turn that opens with `@name` runs as that agent
(`ui.RouteByMention`), a peer's included.

**The editor.** `memdoor acp` (`cmd/cli/cmd/acp.go`, `acp_agent.go`) serves
the Agent Client Protocol on stdin and stdout for Zed, JetBrains or a VS Code
ACP client. It is a door like the others: each editor session is a
conversation on the gateway, each prompt a turn in the editor's folder,
driven by the same `gatewayRunner`.

**The phone.** `/remote` in a window (`cmd/cli/cmd/tui_remote.go`,
`tuiRemote`) makes a link `{id, key}` per conversation (`remoteLinkFor`,
kept in `~/.memdoor/remote.json`, 0600) and settles where it lives
(`remoteSession.resolved`): a gateway on another machine is the relay as the
window reaches it; a gateway reached over loopback is asked `/health` for its
addresses and the link goes on the first, or on the box's name through the
ssh door; only a gateway on localhost alone goes through memdoor.ai, with
the reason in `RelayWhy`. The window then hands the link to the gateway
(`hostRemoteOnGateway`, `PUT /api/remote/<channel>`, `gateway/remote_host.go`
`remoteHosted.handle`); the gateway's own process becomes the terminal end
through the `RemoteHost` it was given (`cmd/cli/cmd/remote_host.go`
`gatewayRemoteHost.Start`, which calls `startRemote` as a window would), so
the link outlives the window and a restart revives it. The page at `/r/<id>`
is the embedded SPA (`gateway/spa_routes.go`), drawing a second Bubble Tea
program on the same conversation (`cmd/cli/cmd/tui_remote_tty.go`) in
xterm.js. The relay (`gateway/remote_relay.go`) pairs the terminal on
`/api/relay` with the pages on `/api/relay/browser` and forwards frames
verbatim: up to `remoteRelayMaxBrowsers` (8) driving pages per link,
watch-only pages (`?watch=1`, the `#v=` link) counted apart up to
`remoteRelayMaxWatchers` (200), frames at most `remoteRelayMaxFrame` (4 MiB).

**Peers.** `memdoor peers add box <gateway> --dir <path>` names a gateway the
person has signed in to (`cmd/cli/cmd/peers.go`, `gateway/peers.go`). From
then on `@box:coder …` in any conversation is a mention of that gateway's
coder: `pkg/message/mention.go` parses `@<peer>:<agent>`, and
`Service.finishPeerTurn` in `pkg/message/peers.go` asks the peer through the
service's `PeerGateways` (`peerSet.Ask`), whose call is the binary's
`peerAsk`: a `gatewayRunner` against the peer, on the stored session, in the
peer's directory, the text streamed back as this session's events. The
answer is saved as the thread's reply from `box:coder`, and the names in it
are resolved here (`peerReplyMentions`), depth-bounded like any A2A chain; a
turn asked by a peer carries `asked_by_peer` and wakes nobody over there.

**Cron.** A schedule is a door with no person behind it. `executeCronJob`
runs the job's message as a turn of the named agent in the job's project;
`postCronAnswer` and `cronAnswerWakes` drop a `HEARTBEAT_OK` or an answer
identical to the last run's, and `wakeConversation` otherwise posts the
answer as a turn of the conversation that scheduled it, the way a spawned
run's report wakes the conversation that spawned it.

## The gate on every door

Every route under `/api` and both sockets take a bearer token. The
middleware in `pkg/authorization/middleware.go` (`AuthMiddleware.Handler`)
verifies it, builds the `shared.ExecutionContext` (actor, workspace, role;
`docs/architecture/EXECUTION_CONTEXT.md`) and puts it on the request; a
missing or foreign token leaves no actor, and each handler's `requireAuth`
(`gateway/chat_server.go`) then refuses. The one header trusted without a
token, `X-Internal-Service`, is accepted from a loopback peer only and only
with this process's own secret (`shared.InternalServiceToken`, made at start,
kept in memory): a tool running inside the process. Any value from loopback
passed until 2026-10-30, so any program on the machine was the gateway itself. `/health` and the relay's browser socket (which
takes the link's proof instead) are the exceptions.

The first account is made from the machine itself. `Server.handleSetup` in
`gateway/setup_handlers.go` answers 403 unless `fromThisMachine` holds (a
loopback peer and no `X-Real-IP` from a proxy), and 409 once a user exists;
a gateway on the network cannot be claimed by whoever reaches it first. The
first run creates that account with a generated password kept in
`~/.memdoor/local-login` (`localUserEmail`, `ensureLocalEngine` in
`tui_firstrun.go`), and the CLI keeps one session per gateway address in
`~/.memdoor/credentials.json` (`gatewayKey`). The ssh door signs in by
reading that file over ssh (`sshRenewSession` → `loginLocal` →
`/api/auth/login`) and saves the session for the box's http address too, so
whoever may ssh in as that user may use every door onto that box without a
password being typed.

A phone link carries its own gate. The key after `#` never reaches a server:
`web/src/remoteCrypto.ts` and `tui_remote.go` derive the frame key and the
browser's proof from it by HKDF-SHA256 (salt `memdoor/remote-control/v1`),
seal every frame with AES-256-GCM with the direction as additional data, and
the relay holds only the pairing id and `sha256(proof)`. A write key, derived
under `write`, separates the full link from the watch-only one. The terminal
end attaches with the gateway's own token (`relayAuth.local` in
`gateway/remote_relay_auth.go`); only a token the gateway did not issue is
asked of billing, and a Free account is refused there (`errRemoteIsPro`).

## What memdoor.ai hosts, and what it does not

The gateway's model calls go to the provider the person connected, on the
person's own key; `gateway/providers/factory.go` resolves every chat request
to one of those providers, and no seat or account ever supplies a model key
(`AGENTS.md`). memdoor.ai appears in the gateway's traffic for four things,
each listed in `docs/SECURITY.md`: billing and sign-in
(`gateway/broker_client.go` `brokerBaseURL`, `https://memdoor.ai/billing`);
the remote relay for a gateway kept to localhost, sealed end to end; the
hosted workflow state (`gateway/hosted_state.go`, `https://memdoor.ai/state`,
one mario-state table per workspace, chosen when a Pro run starts and never
after, the tasks still running on the gateway); and `/share` links. Every
door onto a gateway of one's own, including the phone on the gateway's own
address and the box over ssh, needs no memdoor.ai account.
