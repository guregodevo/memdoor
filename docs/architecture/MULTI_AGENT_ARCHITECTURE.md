# Multi-agent architecture

Several agents share one gateway. Each has its own tool palette, its own
prompt and its own transcript; they meet in a channel, where a mention of one
by another is a turn, and a run one of them spawns reports back by waking the
conversation that asked for it. This page names the code that does each part.

## The agents

A gateway seeds its agents into every active workspace at boot and at setup
(`gateway/agent_seeding.go`, `ensureDefaultAgents`): rows of the `buddies`
table, `INSERT OR IGNORE`, so a person's edits to one survive a restart.
`memdoor agent list` prints what a workspace has.

| Agent | Palette (`buddies.tools`) | Role |
|-------|---------------------------|------|
| coder | `coderToolPalette`: bash, read_file, apply_patch, grep, jgrep, jread, glob, todo_write, todo_read, locate, skill, ask_user_question, notes, recall, web_search, web_fetch, mcp, cron, workflow, sessions_spawn | reads, writes and runs code in the project the window was launched from |
| planner | read_file, grep, glob, todo_write, ask_user_question, task_flow | researches read-only and writes a spec for the coder |
| verifier | verify, read_file | runs build/test/run through `verify` and reports PASS/FAIL |
| runner | bash, read_file | runs a program and reports what it printed |
| narrator | none | speaks only; its prompt turns the coder's tool activity into commentary |
| chief | web_fetch, web_search, memory, channels, send_email | the workspace's general agent |

The palette is the allowlist for every turn the agent runs.
`gateway/agent_runtime_tools.go` `filteredToolsFor` reads `buddy.Tools` from
the turn's context (`sharedctx.BuddyToolsKey`) and keeps only those names out
of the tools registered for the turn; the entry `mcp` admits every tool an MCP
server contributes, and `memory` is added to any palette wider than one tool.
A turn woken on an agent's behalf carries the palette on its job
(`queue.AgentJob.BuddyTools`), so no turn runs as a blank default agent.

Skills are files, not rows: the `skill` tool (in the coder's palette) loads
`<name>.md` disk-first, `<project>/.agents/skills/` before `~/.memdoor/skills/`
(seeded at boot from the repo's `skills/`: `chrome`, `review`, `workflow`), and
`/skill:<name>` in the TUI injects one as the turn's task. The resolution chain
and the seeding rules are in `docs/reference/SKILLS.md`.

## One transcript per agent

A turn in a channel runs on the session key
`workspace:<ws>:channel:<ch>` (`pkg/message/service.go` `executeAgentTurn`,
`shared.NewChannelSessionID`). Every agent mentioned in that channel shares
the key, because the window subscribes to it for the run's events. The
transcript the model reasons over is not shared: `gateway/agent_runtime_session.go`
`transcriptKey` appends `:agent:<name>` to the key, and
`gateway/session_persistence.go` `resolveSessionStorePath` maps that to

    ~/.memdoor/workspaces/<ws>/channels/<ch>/agents/<agent>/sessions.json

so the coder never loads the planner's tool calls and the planner never loads
the coder's. What an agent does see of the others is the channel itself:
`loadChannelHistory` gives each turn the channel's last twenty messages,
replies included. Two more kinds of session get a file of their own, for the
same reason: a spawned run, keyed `agent:<agent>:subagent:<run-id>`, lives
under `~/.memdoor/subagents/`, and a workflow task's turn under
`~/.memdoor/workflow-sessions/`, so one run never reads the one before it.

## A mention is a turn

The window, `memdoor run` and `memdoor agent -m` post to `/api/messages`
(`gateway/message_handlers.go` `handleMessages`), which calls
`pkg/message/service.go` `PostMessage`. The window prefixes the session's agent
to every turn; a turn that opens with `@name` runs as that agent instead
(`cmd/tui/ui/model_ops.go` `RouteByMention`).

`resolveMentions` turns each `@name` (`pkg/message/mention.go` `ParseMentions`)
into an actor: an agent of the workspace by name, else a person, else a peer's
agent. The message is saved, broadcast, and `HandleAgentMentions` runs
detached from the request:

- every mentioned agent runs, except the author itself;
- a reply in a thread also goes to the thread's lead, the first agent that
  answered in it (`getThreadLead`);
- a message that names nobody, in no thread, is offered to the message gate
  when one is configured (`decision_gate.go`);
- a person's turn that opens with a name nobody answers to fails the turn,
  with the reason on the session's event stream (`unansweredMention`,
  `SetTurnFailed`), instead of leaving the window waiting.

Each agent's turn goes through `executeAgentAndSaveResponse`: the turn gate
(below), the turn ledger (`TurnLedger.BeginTurn`, which skips a duplicate
dispatch of the same trigger), then `executeAgentTurn`, which looks the buddy
up, builds the session, runs the model through `agentExecutor.ProcessMessage`
and saves the answer as a reply in the thread. When that reply itself
mentions agents, `HandleAgentMentions` runs again on it, with the turn's
context carried whole by `asyncTurnContext` (`pkg/message/peers.go`): the
execution context, the project directory and the mode, so an agent a reply
wakes works in the thread's project, not the scratch directory.

### The depth bound

`pkg/shared/execution_context.go` keeps `MentionDepth` on the execution
context. A person's message is depth 0. When an agent's reply mentions
another agent, `HandleAgentMentions` runs the mentioned agent with
`WithIncrementedDepth`; `CanMentionAgents` refuses once the depth reaches
`maxMentionDepth` (2): a person asks the coder, the coder asks the planner,
the planner answers the coder, and the coder's next mention wakes nobody. A
person's message always passes the check, so a person can interrupt any chain.

### One turn fanning out

    window                      gateway                                   agents
      |  POST /api/messages "@coder add the flag"
      |-------------------------> PostMessage -> HandleAgentMentions (depth 0)
      |                              |  turnGate(channel, coder)
      |  events on the session <-----|-----------------------------------> coder turn
      |                              |  reply "@planner outline the change" saved
      |                              |  HandleAgentMentions(asyncTurnContext) (depth 1)
      |                              |  turnGate(channel, planner)
      |  events on the session <-----|-----------------------------------> planner turn
      |                              |  reply "@coder here is the outline" saved
      |                              |  HandleAgentMentions (depth 2)
      |  events on the session <-----|-----------------------------------> coder turn, woken
      |                              |  a further mention in its reply: CanMentionAgents false

Each hop is a message in the same thread and a turn in the agent's own
transcript; the window sees all three because they share the session key.

## Spawned runs

The coder's `sessions_spawn` tool (`tools/subagent_tools.go`
`SessionsSpawnTool.ExecuteWithSession`) runs a self-contained subtask in
parallel, in its own session:

1. The child session key is `agent:<agent>:subagent:<run-id>`; the run is
   registered in `gateway/subagents` `SubagentRegistry` with the requester's
   session key and the thread root to reply under.
2. The child's metadata records who waits for it (`requester_session_key`),
   where the requester works (`requester_workdir`), and the programs the
   requester's check runs (`done_when_programs`, copied so a child cannot
   change its parent's check).
3. The target agent's palette and prompt are resolved at spawn time
   (`resolveConfig`) and carried on the job; a target with no palette refuses
   to spawn. Spawning a different agent is checked against the A2A policy
   (`pkg/authorization/a2a_policy.go`, loaded by `gateway/a2a_integration.go`
   `loadA2APolicy`; the default allows any agent to spawn any agent).
4. The job goes to the queue's subagent lane
   (`gateway/queue/lanes.go` `QueueManager.EnqueueSubagentJob`) with the
   requester's working directory and a timeout (300 s unless the call says
   otherwise); the executor cancels the turn when it expires.

While the child runs, `gateway/agent_runtime_progress.go` `requesterOf` finds
the screen waiting on it and mirrors its tool activity there as `subagent`
events, which the window shows as "coder · Bash 10s…".

When the child's turn ends, `gateway/server_jobs.go` `handleSubagentCompletion`
asks the result acceptance (`result_acceptance.go`) whether the task is done;
the first unfinished result goes back to the child once. An accepted result
becomes an announcement (`subagents.BuildAnnouncementMessage`) and wakes the
requester (next section); a child with `cleanup: delete`, the default, has its
session removed afterwards.

## A wake is a turn

`gateway/server_jobs.go` `wakeConversation` gives a conversation a turn it did
not type: the message is the thing it was waiting on. It looks up the
conversation's agent for its palette and prompt, and enqueues an `AgentJob` on
the main lane, in the conversation's project directory, with the caller's
context stripped of its cancel (`context.WithoutCancel`) so a run that is
ending does not take the woken turn down with it. Two callers:

- a spawned run's report, from `handleSubagentCompletion`, as the requesting
  agent in the requester's session, threaded under the message that started it;
- a scheduled check's answer, from `postCronAnswer`: the `cron` tool's job
  answers into the session that scheduled it; `cronAnswerWakes` lets it sleep
  on when the answer is `HEARTBEAT_OK` (under 100 characters) or the same as
  the last run's, and otherwise wakes the conversation with "⏱ <job>, the check
  you scheduled, answered: …".

## Peers: an agent on another gateway (experimental)

`@<peer>:<agent>` reaches an agent of a gateway this person has signed in to
and named: `memdoor peers add box http://box:18789 --dir /srv/app` writes
`~/.memdoor/peers.json` (`gateway/peers.go` `LoadPeers`), `memdoor peers` lists
them (`cmd/cli/cmd/peers.go`). The mechanism is the mention loop above:

- `ParseMentions` accepts the two-part name and `PeerAgent` splits it;
  `resolveMentions` makes it the actor `agent:box:coder` when `peerSet.IsPeer`
  knows the peer, else the part before the colon is tried as a local name.
- `executeAgentTurn` branches before the buddy lookup into `finishPeerTurn`
  (`pkg/message/peers.go`): the text without the mention, plus a line saying
  who asked and where the reply lands (`peerAskText`), goes to
  `peerSet.Ask`, which hands it to the binary's `peerAsk` (a run against the
  peer gateway with the stored sign-in, in the peer's directory) and streams
  the peer's text back onto the asking session as a local turn's events.
- The answer is saved as the thread's reply from `box:coder`, and its mentions
  are resolved on the asking gateway (`peerReplyMentions`: this gateway's
  agents, people and peers, never the peer agent itself), so `@coder` in the
  reply wakes this coder and `@vps:coder` asks another peer, under the same
  depth bound.
- The turn is posted on the peer with `AskedByPeer`
  (`sharedctx.AskedByPeerKey`); there, `askedByPeer` stops the reply's
  mentions from waking anyone, because the thread the reply belongs to is the
  asker's.

`docs/features/PEERS.md` has the receipts and the known gaps.

## Concurrency

Two mechanisms order the work; both let different agents and different
conversations run at the same time.

- `pkg/message/turn_gate.go` `turnGate`: one turn at a time per
  (channel, agent), in arrival order. A "continue" typed while the coder is
  mid-turn waits for the turn instead of interleaving with it; the planner in
  the same channel, or the coder in another, is not held. `TurnActive` lets the
  turn janitor tell a long working turn from a lost one.
- `gateway/queue/lanes.go` `QueueManager`: a FIFO lane per session key, and
  global lanes that cap how many session lanes run at once (main 4, cron 4,
  subagent 8, nested 8). Wakes, scheduled checks, spawned runs and the run
  tracker's A2A runs (`gateway/run_tracker.go`) go through it, so two jobs on
  one session never load the same transcript at once. `docs/reference/QUEUE.md`
  has the failure the queue exists to prevent.

The turn ledger (`TurnLedger`) makes a dispatch idempotent: the same trigger
message for the same agent in the same channel runs once.

## Related

- `docs/features/multi-agent-collaboration.md`: mentions from the person's side
- `docs/features/PEERS.md`: peers, receipts and gaps
- `docs/features/cron-jobs.md`: scheduled checks and their wakes
- `docs/reference/QUEUE.md`: the session lanes
- `docs/reference/SKILLS.md`: where skills live and how they resolve
