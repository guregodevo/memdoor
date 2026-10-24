# Agent Sandbox Security Architecture

What keeps an agent inside its lane, as the code does it. Every tool an agent
calls is a Go function run by the gateway process itself, as the user who
started the gateway: there is no container, no VM, no seccomp profile, and
no per-agent resource limit. The boundary is a set of checks on each tool
call in `gateway/agent_runtime_tools.go` `executeTool`, a write root for the
coder, a virtual directory tree for the other agents, encrypted per-agent
secrets, redaction of what leaves a tool, and a bounded agent-to-agent
chain. This page names each check and where it lives. Where prompts go, and
every host the binary can contact, is `docs/SECURITY.md`.

## Two kinds of agents

`gateway/agent_runtime_coder.go` `agentWorksOnCodebase` decides which rules
apply: an agent whose palette holds `bash`, `edit_file`, `write_file`,
`read_file`, `grep` or `glob` is a codebase agent (the coder, the planner)
and works on the real filesystem under a write root. Every other agent has
its three file tools mapped into a virtual tree under `~/.memdoor/sandbox`.
The palette is the agent's `tools` column, carried on the turn's context as
`sharedctx.BuddyToolsKey` (`gateway/server_jobs.go`,
`gateway/server_websocket.go`); the turn adds MCP, cron and workflow tools
(`gateway/mcp_tools.go` `turnTools`).

## The coder's write root

The coder works on the directory the window was launched from. The TUI sends
its launch directory with every turn; `gateway/server_websocket.go` accepts
it only as an absolute, existing directory on the gateway's machine and
refuses anything else, and `gateway/message_handlers.go` does the same for
the HTTP path. `turnWorkdir` (`gateway/agent_runtime_coder.go`) returns that
directory, or `~/memdoor-coder` when the turn carried none (a CLI channel
message, a schedule without a project). It is never the gateway's own
working directory.

Writes stay inside it; reads go anywhere (`coderPathEscape`'s comment
records the decision: a read refused by `read_file` was available through
`sed` in bash anyway, so refusing it fenced nothing). Concretely, before the
tool runs:

- `coderPathEscape` refuses `write_file` and `edit_file` when `path` or
  `file_path` is an absolute path outside the root, with a message telling
  the model to use a relative path.
- `confineToCoderWorkdir` rewrites relative paths to land under the root,
  injects the root as bash's working directory (`gateway/bash_stream.go`
  takes `cwd` from the harness, not from a `cd` in the command), and hands
  `apply_patch`, `locate`, the verify step and the skill loader the root as
  their directory.
- `escapesWorkdir` refuses a bash command that `cd`s to an absolute
  directory outside the root; a relative `cd` stays inside by construction.
- `apply_patch` receives the set of files this turn has read or written
  (`turnReadsKey`), so it can refuse a blind rewrite of an existing file.

For a codebase agent, `read_file`, `write_file` and `list_files` run their
plain `Function` on the real filesystem (`doerFilesystemTools` and the
`realFS` branch of `executeTool`), so what `write_file` wrote is what
`bash go build` sees. `edit_file`, `grep` and `glob` have always worked on
the real filesystem.

## The virtual tree for every other agent

`pkg/sandbox` is the whole model. An agent has a `SandboxScope`
(`pkg/sandbox/scope.go`): `user`, `channel` or `workspace`, stored in the
`buddies.sandbox_scope` column with a `CHECK` on those three values and a
default of `user` (`pkg/repository/sqlite/factory.go`), validated again
when an agent is created or updated over `/api/agents`
(`gateway/agent_handlers.go`); `memdoor agent add` and `update` have no flag
for it, so an agent made from the CLI keeps the default.
`Allows` orders them: `workspace` reaches channel and user paths, `channel`
reaches user paths, `user` reaches only its own.

`pkg/message/service.go` builds a `sandbox.SandboxContext` for each agent
turn from the channel's workspace, the channel id, the actor who sent the
message, the agent's id and the agent's own scope, and carries it on the Go
context under `sharedctx.SandboxContextKey`. `executeTool` hands it to any
tool that has a `FunctionWithContext` (`tools/agent.go` `ToolDefinition`);
a remote agent's tools get it the same way
(`gateway/remote_tool_adapter.go` `ExecuteToolCall`).

The three file tools with a context (`tools/agent.go` `ReadFileWithContext`,
`ListFilesWithContext`; `tools/core_tools.go` `WriteFileWithContext`) map a
path in two steps (`pkg/sandbox/path.go`):

- `NormalizePath` turns a relative path into a virtual one under the scope's
  own directory: `/user/<user id>/`, `/channel/<channel id>/` or
  `/workspace/<workspace id>/`.
- `ResolvePath` cleans the path first, so `..` cannot pass the prefix check,
  tests it against the scope with `CanAccessPath`, joins it under
  `~/.memdoor/sandbox` (`shared.MemdoorHome("sandbox")`), and checks once
  more that the real path is still under that root.

A denied path returns an error naming the scope and the directories it may
use (`GetAllowedPathsDescription`) and logs `Sandbox security: Path access
denied` at WARN. A context with no scope (a TUI turn carries only the
workspace slug, `gateway/server_websocket.go`) denies every path, which is
why codebase agents take the real-filesystem branch above. Tools that need
only the workspace id, such as the agent and channel listings, read it from
the same context through `inWorkspace` (`tools/core_tools.go`).

That is all a scope decides: which directory tree three tools see. It does
not select credentials, set limits or change the palette.

## Checks on every tool call

`executeTool` runs these in order; each refusal is returned to the model as
the tool's error so it can choose another way, and logged at WARN.

1. The allowlist. `gateway/agent_runtime_inference.go` records the names of
   the tools the agent was offered (`getFilteredTools`) under
   `ctxAllowedTools`; a call to any other name is refused (`Tool execution
   blocked by allowlist`).
2. The hardcoded net for bash. `gateway/bash_guard.go`
   `destructiveBashReason` refuses a fork bomb, `--no-preserve-root`, a
   recursive force-delete whose target is `/`, `~` or `$HOME`, `mkfs`, and
   a write to a raw disk device. It protects the machine from an obvious
   misfire and is not configurable; its own comment says it is a safety net,
   not a sandbox.
3. Workspace tool guards. The `tool_guards` workspace setting is a JSON
   array of rules, each a tool name or `*`, a Go regexp matched against the
   call's input JSON, and an optional message (`gateway/tool_guards.go`
   `parseToolGuards`, `guardBlocks`). One invalid rule is dropped alone,
   never the set. `memdoor guards` (`cmd/cli/cmd/guards.go`) lists, adds,
   removes and clears them; writing the setting takes an admin's sign-in
   (`globalSettingKeys`, `gateway/workspace_settings_handlers.go`). The
   rules apply to every agent in the workspace however the turn started.
4. Approval mode. `gateway/approval.go` `askApproval` asks the person before
   `bash`, `apply_patch`, `write_file`, `edit_file` and any `mcp__` tool when
   `MEMDOOR_APPROVE=changes` is in the environment or the `approve`
   workspace setting says `changes`; the environment wins, and both are
   read per call. The question is the same picker the agent uses for its
   own questions, with `approvalSummary` showing the command, the files a
   patch touches, the path a write goes to or the MCP tool's input, and the
   answers Yes, Yes for this tool for the rest of the session, or No. No
   answer in five minutes (`approvalTimeout`) is a no. Reads never ask.
5. The coder's write root, described above, for codebase agents.

## Secrets

An agent's secrets are rows in `agent_secrets` keyed by agent id and name
(`pkg/repository/agent_secret.go` `AgentSecretRepository`,
`pkg/repository/sqlite/agent_secrets.go`), encrypted with AES-256-GCM under
`~/.memdoor/master.key`, a file the gateway creates with mode 0600
(`pkg/secrets/crypto.go` `Encrypt`, `Decrypt`, `getMasterKey`). They are set
and listed over `/api/agents/<name>/secrets` (`gateway/agent_handlers.go`
`handleAgentSecrets`); `/api/secrets` holds the same kind of row under the
`system` agent id. The `Value` field never serializes to JSON, and a listing
returns names only.

When an agent has at least one secret, `pkg/message/service.go` adds the
`get_secret` tool to its palette for the turn. `executeTool` answers that
tool itself: `tools/secret_tool.go` `GetSecretWithRepo` looks the name up
under `SandboxContext.CurrentAgentID`, so an agent can read only secrets
assigned to it, and the transcript records `[secret retrieved]` in place of
the value. The same key encrypts MCP OAuth tokens (`pkg/mcp/tokens.go`) and
the ChatGPT sign-in credential (`pkg/chatgpt/plan.go`).

## Redaction of what leaves a tool

`pkg/secrets/redact.go` `RedactText` replaces secret-shaped text with
`[redacted]`: private key blocks, vendor tokens by prefix (OpenRouter,
Anthropic, OpenAI, Slack, GitHub, GitLab, AWS, Google, Stripe, Resend),
JSON web tokens, and any value announced by a name such as `password=`,
`API_KEY:` or `Authorization: Bearer`, where the name stays and the value
goes. It runs on every tool's output and error and on the text blocks of the
result the model reads (`gateway/agent_runtime_tools.go` `redactToolResult`),
on each streamed chunk of a bash command (`gateway/bash_stream.go`), and on
a conversation about to be shared (`cmd/cli/cmd/tui_share.go`).

## Agent to agent

`pkg/authorization/a2a_policy.go` `A2APolicy` says which agent may message
which. The default allows every pair (`DefaultA2APolicy`: enabled, one rule
`*` to `*`, `MaxPingPongTurns` 0); `tools.agentToAgent.allow` in the
gateway config replaces that roster, and `IsAllowed` denies when the policy
is off or no rule matches. `gateway/server.go` wires the policy into
`sessions_send` and `sessions_spawn`.

The chain is bounded by depth: `pkg/shared/execution_context.go`
`CanMentionAgents` allows a mention while `MentionDepth` is below two (the
person, an agent the person asked, an agent that agent asked, stop), and
`pkg/message/service.go` increments the depth on every agent mention and
logs it as `mention_depth`. A mentioned agent runs under a `SandboxContext`
built from its own scope, not the asker's; the same per-call checks apply
to its tools.

## Network

The gateway binds its port on the network by default and `docs/SECURITY.md`
("Where the gateway listens") states the terms: a token on every door except
`/health` and the sealed relay pages, the first account only from the
machine itself, `MEMDOOR_HOST=localhost` to keep it to one machine. Every
host the binary can name is in that page's table, and
`gateway/egress_test.go` `TestEveryHostTheBinaryNamesIsDocumented` fails
the build when a new host literal appears in non-test Go that the page
does not list.

## Reading the trail

Every refusal above is a log line: `Tool execution blocked by allowlist`,
`Destructive bash command blocked`, `Tool call blocked by workspace guard`,
`Tool call blocked: absolute path outside coder workdir`, `Sandbox security:
Path access denied`, and for approval mode `Approval asked`, `Approval
given`, `Approval refused` and `Approval not answered`.
`memdoor logs query --regex "blocked|Approval|Sandbox security" --since 1h`
shows them. With `MEMDOOR_TELEMETRY_ENABLED=1` the WARN lines also reach the
memdoor.ai monitoring inbox (`docs/internal/TELEMETRY.md`).
