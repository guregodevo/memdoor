# ExecutionContext: who acts, and where

`shared.ExecutionContext` (`pkg/shared/execution_context.go`) is the request-scoped
record of who is acting and in which workspace. The auth middleware builds it from
the bearer token and puts it on the `context.Context`; handlers, services and the
agent runtime read it from there. A request body never names its own user: the actor
is whatever the token proved (`docs/reference/AUTHORIZATION.md`), and the workspace
is the one on that user's row. Beside it, a turn carries a handful of typed context
keys (`pkg/shared/context/keys.go`) for what the auth layer does not know: the
project directory, the interaction mode, the model route, the peer flag.

## What it carries

| Field | Set from | Read by |
|-------|----------|---------|
| `ActorID` | the token's user (`human:<uuid>`), or `system:internal` for an in-process tool call | authorization checks, session keys, the sandbox's `InitiatingUserID` |
| `WorkspaceID` | the user's row (one user, one workspace) | every workspace-scoped query; `WorkspaceFromContext` |
| `WorkspaceSlug` | `AuthMiddleware.SetSlugResolver`, the `/{slug}/api/...` prefix, or the message body | session paths, the sandbox root, workspace-scoped tools |
| `UserEmail` | the token's user | logs |
| `UserRole` | the user's row (`admin` / `user`) | `message.Service.validateMentions`: a regular user mentions channel members only |
| `MentionDepth` | 0 for a person's request; `WithIncrementedDepth()` on each agent-to-agent mention | `CanMentionAgents()` stops the chain at depth 2 |

`NewExecutionContext(actorID, workspaceID)` and `NewExecutionContextWithEmail` are the
constructors: an invalid actor id or an empty workspace id is an error, so a value
that exists is one that passed (`docs/architecture/ALWAYS_VALID_PATTERN.md`).
`MustNewExecutionContext` panics instead, for the one place that builds a fallback.
`IsHuman()` and `IsAgent()` read the actor kind. `WithIncrementedDepth()` copies
`ActorID`, `WorkspaceID`, `UserEmail` and the depth; the slug is backfilled further
down (below) and `UserRole` is not carried into an agent's own mentions.

`WithExecutionContext(ctx, ec)` stores it under `shared.ExecutionContextKey`;
`GetExecutionContext(ctx)` returns it, or nil when the request was anonymous.

## Where it is built

`pkg/authorization/middleware.go`, `AuthMiddleware.Handler`, is the one place a
signed-in request gets one. It verifies the bearer token with `AuthService.VerifyToken`,
calls `NewExecutionContextWithEmail(ActorID(user.ID), user.WorkspaceID, user.Email)`,
sets `UserRole` from the row and `WorkspaceSlug` through the slug resolver, and puts
the result on the request context. A request without a token, or with one this gateway
cannot verify, passes through with no context at all: public reads answer anonymously,
protected handlers refuse through `requireAuth` / `requireAdmin`.

The same handler has a second door for the gateway's own process: a request with
`X-Internal-Service` from a loopback address (a tool calling the gateway's API from
inside a run) gets `ActorID: "system:internal"` and the workspace named in
`X-Forwarded-Workspace`, `"default"` when none, meaning the whole gateway.

Three smaller sites add or supply one:

- `gateway/server_lifecycle.go`, `workspaceMiddleware`: a leading `/{slug}/` that names
  an existing workspace (or `X-Forwarded-Workspace`) sets `WorkspaceSlug` on the
  context's value, or a value with only the slug when the request was anonymous.
- `gateway/message_handlers.go`, `handleMessages`: the required `workspace` field of a
  posted message sets `WorkspaceSlug` the same way.
- `gateway/server_websocket.go`, `wsMayFollowChannel`: a websocket subscriber's user
  row becomes `{ActorID, WorkspaceID}` so `authorization.CanReadChannel` applies the
  same workspace rule as a REST read.

When a turn reaches `message.Service.executeAgentTurn` without one (a websocket turn's
job context carries none, so an agent reply it posts that mentions another agent
arrives bare), it builds a fallback with
`MustNewExecutionContext` from a fixed CLI user id and the channel's workspace, and
logs a warning. Whatever the source, an empty `WorkspaceSlug` is then backfilled from
the channel's workspace through `channelRepo.GetWorkspaceSlug`, because the session
key, the sandbox root and the workspace-scoped tools all read it.

## How it travels

The HTTP context is cancelled when the response is written, so anything that outlives
the request copies the value onto a fresh `context.Background()`:

- `message.Service.PostMessage` (`pkg/message/service.go`) detaches it before
  `HandleAgentMentions`, and adds the turn's keys from the request body: the
  `PermissionMode`, the `Workdir` (kept only when absolute and an existing directory),
  and `AskedByPeer`.
- `gateway/thread_handlers.go` does the same for a thread reply.
- `asyncTurnContext` (`pkg/message/peers.go`) is the detach for a turn woken by an
  agent's reply: it carries the execution context (with its mention depth), the
  `WorkdirKey` and the `PermissionModeKey` of the turn before it, so a woken turn runs
  in the thread's project and mode rather than the scratch directory.

Agent-to-agent mentions deepen the context instead of copying it:
`HandleAgentMentions` refuses an agent author whose context says `!CanMentionAgents()`,
and wraps each mention it does run in `WithIncrementedDepth()`. A person is never
depth-limited.

Mention authorization reads `UserRole` here too: `validateMentions` drops a mention of
a non-member when the actor is a regular user.

## The turn's context keys

`pkg/shared/context/keys.go` declares every key as a `sharedctx.ContextKey`. Each one
is set in one or two places and read with a typed `ctx.Value(...)` assertion.

| Key | Set by | Read by |
|-----|--------|---------|
| `WorkdirKey` | the TUI's launch directory: `server_websocket.go` (`msg.Data["workdir"]`, refused when it is not a directory on the gateway's machine), `PostMessage` (`req.Workdir`), `server_jobs.go` (`job.Workdir`, and the requester's directory on a spawned job), `asyncTurnContext` | `turnWorkdir` (`gateway/agent_runtime_coder.go`), the coder's write root; `cron_tool.go`, `workflow_tool.go`, `gateway/providers/attribution.go` |
| `PermissionModeKey` | the TUI's `permission_mode` (`default`, `acceptEdits`, `plan`) in `server_websocket.go`; `req.PermissionMode` in `PostMessage`; copied by `server_jobs.go` and `asyncTurnContext` | `permissionModeFromContext` (`agent_runtime_coder.go`): plan mode makes the turn read-only |
| `AskedByPeerKey` | `PostMessage` when the body says `asked_by_peer` (a `@<peer>:<agent>` turn from another gateway) | `askedByPeer` (`pkg/message/peers.go`): the reply wakes nobody here, its names are the asker's |
| `SandboxContextKey` | `executeAgentTurn` (a `sandbox.SandboxContext` with the channel's workspace, the slug, the initiating user and the agent); `server_websocket.go` with the connection's workspace slug | `agent_runtime_tools.go`, `agent_runtime_inference.go`, `remote_tool_adapter.go`: the workspace a tool runs in |
| `ActorIDKey`, `WorkspaceIDKey` | `AuthMiddleware.Handler` beside the struct; `server_jobs.go` copies the actor onto a job's run for attribution | `authorization.GetActorID`, hence `requireAuth` and `requireAdmin` |
| `AgentIDKey` | `server_jobs.go` from `job.AgentID` | `cron_tool.go`: whose schedule it is |
| `IsBuddyChatKey`, `BuddyToolsKey`, `BuddyLearningKey`, `BuddyTemperatureKey` | `executeAgentTurn` and `server_jobs.go` from the buddy row | the agent runtime's prompt mode, palette and sampling |
| `SessionIDKey`, `TierKey`, `ModelKey`, `SortKey`, `OrderKey`, `EffortKey` | `agent_runtime_process.go` from the session's route metadata; `workflow_runs.go` and `model_check.go` pin a model | the provider factory and the ladder (`gateway/providers`) |

The approval mode (`approve` setting, `MEMDOOR_APPROVE`) is not a context value:
`AgentRuntime.approvalMode` in `gateway/approval.go` reads it per tool call.

## How handlers and tools read it

- `shared.GetExecutionContext(ctx)` for the struct; nil means anonymous, and the
  caller decides (`setup_handlers.go`, `invite_email_handlers.go`,
  `workspace_settings_handlers.go` take `WorkspaceSlug` from it when present).
- `shared.WorkspaceFromContext(ctx)` for the workspace a request acts in: the user's
  id, or `""` for the whole gateway when the value is absent or the internal
  `"default"`. `cronWorkspaceID` (`gateway/cron_handlers.go`) maps that case to
  `domain.DefaultWorkspaceID`.
- `authorization.GetActorID(ctx)` for the actor alone; `requireAuth`
  (`gateway/chat_server.go`) answers 401 without one, `requireAdmin`
  (`gateway/authz_helpers.go`) 403 unless `IsWorkspaceAdmin`, which counts
  `system:internal` as admin.
- `authorization.Service.outsideActorsWorkspace` (`pkg/authorization/permissions.go`)
  refuses a human's read of a channel in another workspace, using `WorkspaceID`.
- Inside a run, `turnWorkdir`, `permissionModeFromContext` and `askedByPeer` read
  their keys as above.

## In tests

A test puts the value on the context the way the middleware would, no constructor
needed when the fields are known good:

- `gateway/channel_scope_test.go`, `requestIn(method, path, actor, workspace)`: an
  `httptest` request with `ActorIDKey` and a
  `&shared.ExecutionContext{ActorID, WorkspaceID}` on its context.
- `gateway/cron_workspace_test.go`, `withWS(id)`: `shared.WithExecutionContext`
  on `context.Background()` with only a `WorkspaceID`.
- `gateway/workspace_middleware_test.go` runs `workspaceMiddleware` and reads
  `WorkspaceSlug` back with `GetExecutionContext`.
- `authorization.WithActorID(ctx, actorID)` sets the actor key alone.

## See also

- `pkg/shared/actor_id.go`: the `ActorID` value object (`human:<uuid>` / `agent:<name>`; `system:internal` is the gateway's own)
- `pkg/shared/context/keys.go`: the keys and what each means
- `pkg/authorization/middleware.go`: where the value is built
- `pkg/message/service.go`, `pkg/message/peers.go`: the detach, the depth, the wake
- `docs/reference/AUTHORIZATION.md`: the rules the value feeds
