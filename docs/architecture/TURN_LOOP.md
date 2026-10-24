# One agent turn

What happens between a message arriving and its answer, as the code does it
today, by file and function; the thread through [QUEUE](../reference/QUEUE.md),
[SKILLS](../reference/SKILLS.md) and [DECIDE](../features/DECIDE.md).

## Arrival

A window posts a turn as a message. `cmd/cli/cmd/tui_wiring.go tuiPoster` sends
`POST /api/messages` with the channel, the text prefixed `@<agent> `, the
workspace, the `permission_mode` the window is in and `workdir`, the directory
the window was launched from. A leading `@name` the person typed routes the
turn to that agent (`cmd/tui/ui/model_ops.go RouteByMention`); `memdoor agent
-m` posts the same way.

`gateway/message_handlers.go handleMessages` requires a workspace and an
authenticated actor, refuses a `workdir` that is not an absolute, existing
directory on the gateway's machine, checks `CanWriteToChannel` and calls
`pkg/message/service.go PostMessage`. PostMessage checks membership, resolves
the `@mentions` from the text (`resolveMentions`; a DM with one agent mentions
it by itself, `addDMAutoMentions`), keeps the mentions of channel members only
(`validateMentions`), saves the message and, when there is a mention or a
message gate is configured, starts the turn on a detached context carrying the
`ExecutionContext` (actor, workspace, mention depth) and three values from
`pkg/shared/context/keys.go`: `PermissionModeKey`, `WorkdirKey`,
`AskedByPeerKey`.

`HandleAgentMentions` decides who runs. An agent-authored message past
`maxMentionDepth` (2, `pkg/shared/execution_context.go CanMentionAgents`) wakes
nobody; a human's always does. The agents are the mentioned ones, plus the
thread lead when the message is a reply (`getThreadLead`), or, when nobody was
addressed and `decision_message_gate` is on, the ones the decision model picks
(`pkg/message/decision_gate.go gatedAgents`). A name nobody answers to fails
the turn visibly instead of hanging the window (`unansweredMention`).

## One turn at a time

`executeAgentAndSaveResponse` takes the `turnGate` for (channel, agent)
(`pkg/message/turn_gate.go`): a second message for the same agent in the same
channel waits for the running turn, in arrival order. The ledger records the dispatch
(`TurnLedger.BeginTurn`, `gateway/turn_ledger.go` over
`repository.AgentTurnRepository`); the same trigger dispatched twice is skipped.

Turns that do not start from a posted message (a scheduled check, a spawned
run, a workflow task, a wake) go through `gateway/queue` instead
(`lanes.go`): a FIFO `SessionLane` per session key
serializes the jobs on one session, the global lanes (`main`, `cron`,
`subagent`, `nested`) bound concurrency, and `Server.executeAgentJob`
(`gateway/server_jobs.go`) runs each job through the same
`AgentRuntime.ProcessMessage`, posting a channel session's answer with
`PostSystemAnnouncement`.

## Building the turn

`executeAgentTurn` (`pkg/message/service.go`) builds the session key
`shared.NewChannelSessionID(workspace, channel)`, the key the window's websocket
subscribes to, names the run `run-<message id>-<agent>`, broadcasts
`execution.started` and looks the agent up (`buddyRepo.GetByName`); the palette
rides on the context (`BuddyToolsKey`) with its name. The agent's own prompt is
`pkg/prompts/agent_extra_prompt.go BuildAgentExtraPrompt`: personality, the
agent instructions (a workspace `<ws>/agents/<name>.prompt.md` wins), the
language. The turn runs under an idle timeout, not a total:
`sharedctx.WithIdleTimeout` is reset by every model answer and tool result
(`sharedctx.Progress`), so a working turn is never reaped.
`gateway/agent_runtime_adapter.go AgentExecutorAdapter.ProcessMessage` carries
the session's route (a pinned model, `persistent_pin.go`), registers the run's
cancel under the session key and calls `gateway/agent_runtime_process.go
processMessage`, which does the rest.

**Session.** `gateway/agent_runtime_session.go buildConversation` loads the
agent's own transcript: the key is the channel session plus `:agent:<name>`
(`transcriptKey`), the store
`~/.memdoor/workspaces/<ws>/channels/<channel>/agents/<agent>/sessions.json`
(`gateway/session_persistence.go resolveSessionStorePath`), JSONL, append-only.
A codebase agent, one whose palette has bash or file tools
(`agent_runtime_coder.go agentWorksOnCodebase`), loads the whole transcript; a
chat agent the last ten messages (`recentMessageLimitForAgent`). After every
tool round and at the end, `updateSession` appends what the transcript does not
hold yet, so a crash never leaves a tool call unanswered.

**Prompt and skills.** `gateway/agent_runtime_inference.go runInference` builds
the system prompt with `gateway/prompts/system.go SystemPromptBuilder`: the
agent's name, the project's `AGENTS.md` from the turn's workdir
(`project_instructions.go`), the skills loaded at boot (`agent_adapter.go
loadSkills`); a codebase agent gets the identity line only (`PromptModeNone`),
its seeded prompt, sent first, being the instruction. Skills resolve from disk
at call time, first hit wins: `<project>/.agents/skills`, the gateway's `./skills`,
`~/.memdoor/workspace/skills`, `~/.memdoor/skills` (seeded at boot), then the
embedded copy (`tools/advanced_tools.go skillDirs`); `/skill:<name>` injects a
skill's text as the task (`slash_skills.go expandSlashSkill`).

**Toolbox and effort.** The person's MCP tools join the turn (`mcp_tools.go
withMCPTools`), then `gateway/tool_routing.go toolRouter.route` asks the
decision model one question when `decision_tool_routing` is on: which tool
family the request needs. That family plus the agent's always-allowed core is
what the model is offered (`withTurnToolsAllow`, `submittedTools`); the full
palette stays enforced at execution, and the router fails open. Reasoning
effort is `gateway/turn_effort.go turnEffort`: the person's Shift+Tab choice,
else the decision model's low/medium/high for the request, else high.

**Compaction.** Each inference starts with `preflight.Check` on the prompt, the
tool schemas and the conversation; `compactionDecision` fits the conversation
at once over the hard limit, and over the threshold only on a turn's first
inference: mid-turn the tool results are the working set, so a threshold
compaction waits for the next turn (`withMidTurn`). Fitting is one ladder
(`gateway/agent_runtime_fit.go fitLadder`: stub old tool output, then
summarize); `flushNotes` keeps what the dropped part said worth keeping, and
the conversation as sent is appended behind a boundary record so the next turn
sends the same cached prefix.

## The loop

```
message posted ─► PostMessage ─► HandleAgentMentions ─► turnGate ─► ledger: dispatched
   buildConversation ─► slash skill? ─► tool routing ─► effort ─► ledger: running
   ┌──► runInference ──(preflight, compaction, provider)──► assistant message
   │          │                       max_tokens ─► escalate once, then resume
   │          ▼
   │   no tool call ──► empty / reasoning-only retry ─► recap ─► done-when or
   │          │                                   verdict unfinished? ─► continue
   │          ▼ finished
   │   receipts + honesty notes ─► complete event ─► reply posted ─► ledger: responded
   │   tool calls ──► for each: start event ─► executeTool ─► complete event
   │                  (target guard, allowlist, destructive bash, tool_guards,
   │                   ask gate, approval, workdir confinement)
   │                  loop-breakers: first trip nudges, second ends the turn
   │                  shadow score ─► decision model: stuck? exploring?
   └── tool results appended, session saved
```

**The model call.** `runInference` resolves the client through
`gateway/providers/factory.go ClientFactory.GetClientFor`: the provider that
lists the pinned model, else the one chosen at start. A stream callback turns
every delta into an event (`llm.WithStreamCallback`), and a codebase turn runs
under the prose breaker (`providers.ProseRunawayGuard`): a reply running long
as tag-free prose with no tool call is stopped and retried. A reply cut at the
output cap is retried once at a larger cap, then resumed, a bounded number of
times (`NewTruncationRecovery`).

**A tool call.** Each `tool_use` block gets a `start` event with its id, a
heartbeat every five seconds while it runs (`agent_runtime_progress.go
watchToolProgress`) and a `complete` event with the output. A tool that failed
three times by name is retired for the turn (`toolFailureLedger`).
`gateway/agent_runtime_tools.go executeTool` runs the
gates in order: the target guard (`target_guard.go`: a workflow task may not
edit the program its own check runs), the agent's allowlist,
`destructiveBashReason`, the workspace's `tool_guards` rules (`tool_guards.go
guardBlocks`), the ask gate (`ask_gate.go`: a question the agent could answer
by reading or running goes back to it), approval, then for a codebase agent the
confinement of every path to the turn's workdir (`coderPathEscape`,
`confineToCoderWorkdir`). Plan mode (`permission_mode: "plan"`, `memdoor agent
--mode plan`) is read-only: `planModeRefuses` (`agent_runtime_coder.go`) turns
back bash, apply_patch, write_file, edit_file, sessions_spawn, cron, workflow,
task_flow, chrome, send_email and every MCP tool with the way out, and
`exit_plan_mode` is told to wait for approval in that mode and to proceed
otherwise. `ask_user_question` is emitted as a
`question` event and blocks on `question_broker.go` until the window's `answer`
arrives.

**The approval gate.** `gateway/approval.go askApproval` runs when
`MEMDOOR_APPROVE=changes` or the `approve` workspace setting says so (read per
call): before `bash`, `apply_patch`, `write_file`,
`edit_file` and every `mcp__` tool, the picker asks Yes, "Yes, and don't ask
again for this tool this session", or No. Always grants the tool for the
session; No, or no answer in five minutes, ends the call with a result the
model reads and acts on.

**What ends a turn.** There is no cap on model calls. The loop-breakers in
`processMessage` count the same failing call (3), consecutive failures (4), the
same call with the same result (4), the same result under different commands
(5) and the same ask reworded (6); the first trip puts `loopNudge` in the
model's next input, the second ends the turn with the last failure named. When
the shadow score (`route_shadow.go`) sees steps without progress,
`turn_verdict.go stuck` asks the decision model once, and a yes ends the turn
after one wrap-up round with no tools (`wrapUpTurn`).
Every 30 reads with no edit (`shadowExploreEvery`), `turn_verdict.go exploring`
asks whether to stop exploring: the first yes nudges (`exploreNudge`), the
second ends the turn the same way.

A reply with no tool call is checked before it ends the turn: an empty or
reasoning-only reply is retried once (`empty_reply.go`, `WithForcedToolCall`),
a long turn ending on a line is asked for a recap once (`recap.go`), then
`turn_done.go` reads the receipts: a declared target (`done_when`, `file X` or
`command: …`) is checked on disk or run, else `turn_verdict.go unfinished` asks
the decision model whether the receipts show the request done. Unfinished
sends the model back with its own "next" as the prompt (`continueNudge`); the
first continuation is free, the rest spend `turn_token_budget`
(`continueAllowed`).

## Streaming to the window

The runtime emits on `infra.EventEmitter`; `gateway/streaming/integration.go
ConnectInfraEventEmitter` wraps each event as an `agent_event` and broadcasts
it to the run's and the session's subscribers. A window's websocket
(`gateway/server_websocket.go`) subscribes to `NewChannelSessionID(workspace,
channel)`, the key the run emits on. The streams: `lifecycle` (`start`,
`complete`, `error`, `provider_notice`), `assistant` (`thinking`, `text_delta`,
`text`), `tool` (`start`, `progress`, `complete`, `subagent`), `context`
(`context_update`, the gauge), `question`, `plan`. The chat hub carries
`message.created`, `execution.started`, `execution.completed`,
`execution.failed` (`chat_server.go CreateExecutionBroadcaster`) and
`execution.todo.updated`. The status line reads the `progress` beats and, for a
spawned run, the `subagent` beats mirrored onto the requester's session
(`emitSubagentWork`): "coder · Bash 10s".
A notice the rounds did not stream goes out as one more `text` event before
`complete` (`unstreamed.go`), which closes the message in the window.

## Ending

Before the complete event, a codebase turn's text gets its honesty notes
(`agent_runtime_guards.go`): a turn that used no tools says so; a reply that
claims a change when no file-mutating tool succeeded (`claimsFileChange`,
`fileMutationSucceeded`) says it was not applied; code shown but not written
(`containsNarratedCode`) says so. Then the receipt line: what changed and what
was checked, with the declared target's state when there is one.

Back in `executeAgentTurn`: `REPLY_SKIP` and an empty reply end the turn
silently; otherwise the reply's `@mentions` are parsed (at most 3, channel
members or a peer's agent), a self-mention becomes `@me`, and the reply is
saved with `NewReply` under the thread root (top-level in a DM), detached from
the turn's deadline. The ledger is marked responded (`TurnResponded`);
`execution.completed` and `message.created` go out. A reply with mentions starts the next
turn, `go HandleAgentMentions(asyncTurnContext(ctx), reply)`, where
`pkg/message/peers.go asyncTurnContext` carries the execution context, the
workdir and the permission mode of the turn before it. A turn a peer asked for
(`asked_by_peer`) wakes nobody here; its names are the asker's. The ledger
(`pkg/repository/interfaces.go AgentTurnRepository`, sqlite) holds every
dispatch as dispatched, running, responded or failed; its janitor asks
`Service.TurnActive` before declaring a quiet turn lost.

## What wakes whom

`gateway/server_jobs.go wakeConversation` gives a conversation a turn it did not
ask for, as its own agent, in its project directory, queued on its session lane
behind any running job: a spawned run's report (`handleSubagentCompletion`,
once `result_acceptance.go` has asked the decision model whether the result is
the task done, rerunning the child when not) and a scheduled check's answer
(`cronAnswerWakes`: `HEARTBEAT_OK`, or the same answer as last run, wakes
nobody). A peer's agent (`@box:coder`) runs on that gateway
(`pkg/message/peers.go finishPeerTurn`) and its answer is this thread's reply,
resolved like any other.

## Interruption

Esc sends `cancel` (`protocol.MessageTypeCancel`, `cmd/tui/ui/websocket_adapter.go
SendCancel`); `server_websocket.go` calls `Server.cancelRun(session)`, which
fires the cancel the run registered under that key, by
`AgentExecutorAdapter.ProcessMessage` on the message path and `executeAgentJob`
on the queue path. The cancel travels down the context to the provider stream
and the running tool; the turn ends with the lifecycle `error` and the window
gets `cancelled`. Typing while a turn runs does not post: the window queues the
text (`cmd/tui/ui/model_update.go`, "Queued (n), will send when the current
reply finishes") and flushes it on the run's complete event; a prompt that does
reach the gateway mid-turn waits at the turn gate, in order.
