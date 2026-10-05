# Authorization & Access Control Reference

**Version**: 1.0
**Date**: 2026-04-18

## Overview

Memdoor enforces access control at multiple layers: HTTP authentication, channel membership RBAC, workspace admin privileges, agent tool filtering, sandbox path enforcement, agent-to-agent policies, and per-agent secrets isolation.

```
HTTP Request
  |
  v
Auth Middleware ---- Bearer token --> VerifyToken() --> ExecutionContext
  |
  v
Gateway Handler --- actorID, channelID --> Authorization Service
  |                                            |
  |                                    CanReadChannel()
  |                                    CanWriteToChannel()
  |                                    IsWorkspaceAdmin()
  v
Agent Runtime ----- buddy_tools --> getFilteredTools() --> Tool Allowlist
  |                                                           |
  |                                                    Tool Profiles
  |                                                    (coding, minimal, full)
  v
Tool Execution ---- SandboxContext --> CanAccessPath() --> Filesystem Boundary
  |
  v
A2A Policy -------- from/to rules --> IsAllowed() --> Cross-Agent Gate
```

## 1. Authentication

**Package:** `pkg/auth/` | **Middleware:** `pkg/authorization/middleware.go`

### Token Flow

1. User calls `POST /api/auth/login` with email + password
2. Server verifies bcrypt hash, creates 24-hour session token
3. Token stored as SHA-256 hash in `sessions` table
4. Client sends `Authorization: Bearer <token>` on subsequent requests
5. Middleware calls `VerifyToken()`, creates `ExecutionContext` in request context

### ExecutionContext

**File:** `pkg/shared/execution_context.go`

Every authenticated request carries:

| Field | Description |
|-------|-------------|
| `ActorID` | `human:<uuid>` or `agent:<name>` |
| `WorkspaceID` | Tenant boundary |
| `UserEmail` | For audit logging |
| `MentionDepth` | 0=user, 1=mentioned agent, 2=max (stops chains) |

Follows the Always-Valid pattern: validated at construction, guaranteed valid thereafter.

### User Roles

**File:** `pkg/domain/models.go`

| Role | Description |
|------|-------------|
| `admin` | Workspace administrator. Can create agents, manage secrets. |
| `user` | Regular user. Can use channels, mention agents. |

Checked via `authzService.IsWorkspaceAdmin(ctx, actorID)`.

Default users from `memdoor setup`: `admin@localhost` (admin), `alice@localhost` (user), `quentin@localhost` (user).

## 2. Channel RBAC

**Package:** `pkg/authorization/permissions.go`

### Roles

| Role | Permissions |
|------|------------|
| `admin` | All: read, write, delete channel, manage members, delete/edit any message, reactions |
| `member` | Read, write, edit own messages, view/add members, reactions |
| `guest` | Read only: channels, messages, member list |

### Permissions

| Permission | Admin | Member | Guest |
|-----------|-------|--------|-------|
| `channel:read` | Y | Y | Y |
| `channel:write` | Y | Y | - |
| `channel:delete` | Y | - | - |
| `channel:manage` | Y | - | - |
| `message:read` | Y | Y | Y |
| `message:write` | Y | Y | - |
| `message:delete` | Y | - | - |
| `message:edit` | Y | Y (own) | - |
| `members:add` | Y | Y | - |
| `members:remove` | Y | - | - |
| `members:view` | Y | Y | Y |
| `reaction:add` | Y | Y | - |
| `reaction:remove` | Y | Y | - |

### Special Rules

- **Public channels** are readable by anyone, even non-members (`CanReadChannel` checks `channel.IsPrivate`)
- **Self-removal** is always allowed (users can leave channels)
- **Last admin protection** prevents removing the only admin from a channel
- **System operations** use `RequesterID = nil` to bypass authorization (setup, migrations)
- **Channel creator** is automatically added as admin

### Enforcement Points

| Operation | Check Location | Method |
|-----------|---------------|--------|
| List channels | `chat_server.go` handleGetChannels | `CanReadChannel()` per channel |
| Read messages | `chat_server.go` handleGetRoomMessages | `CanReadChannel()` |
| Post message | `chat_server.go` handleMessages | `CanWriteToChannel()` |
| Post message (service) | `pkg/message/service.go` PostMessage | `IsMember()` (defense-in-depth) |
| Add member | `pkg/channel/service.go` AddMember | `CanAddMembers()` predicate |
| Remove member | `pkg/channel/service.go` RemoveMember | `CanRemoveMembers()` predicate |

## 3. Workspace Admin Operations

Only workspace admins (`domain.UserRoleAdmin`) can:

| Operation | Endpoint | Check |
|-----------|----------|-------|
| Create agent | `POST /api/agents` | `IsWorkspaceAdmin()` |
| Set agent secret | `POST /api/agents/:name/secrets` | `IsWorkspaceAdmin()` |
| List agent secrets | `GET /api/agents/:name/secrets` | `IsWorkspaceAdmin()` |
| Delete agent secret | `DELETE /api/agents/:name/secrets/:key` | `IsWorkspaceAdmin()` |


## 4. Tool Access Control

**Files:** `gateway/agent_adapter.go` (runtime), `gateway/config/tool_policy.go` (profiles)

### How Tools Are Assigned

Each agent has a `tools` JSON array in the `buddies` table. At runtime, `getFilteredTools()` decides which tools an agent can use:

```
Priority:
1. buddy_tools from database (highest) --> string allowlist
2. Agent config tools (profile/allow/deny) --> from config file
3. Default "coding" profile --> fallback
```

### Tool Profiles

**File:** `gateway/config/tool_policy.go`

The default profile for new agents is `default` (set via `config.DefaultToolProfile` constant).

| Profile | Description | Tools (~count) |
|---------|-------------|----------------|
| `minimal` | Status check only | `session_status` (~1) |
| `default` | Read-only + web + chrome | `group:fs_read`, `group:sessions`, `group:memory`, `group:context`, `group:web`, `chrome_devtools`, `ask_user_question`, `todo_write` (~18) |
| `coding` | Full write + bash + introspection | `group:fs`, `group:runtime`, `group:sessions`, `group:sessions_spawn`, `group:memory`, `group:context`, `group:web`, `group:introspection`, `chrome_devtools`, `ask_user_question`, `todo_write` (~31) |
| `messaging` | Channel communication only | `sessions_list`, `sessions_history`, `sessions_send`, `context` (~4) |
| `full` | All tools (no restrictions) | Empty allowlist = everything (~all) |

### Tool Groups

Tools can be referenced by group in config:

| Group | Tools |
|-------|-------|
| `group:fs_read` | `read_file`, `list_files`, `glob`, `grep` |
| `group:fs_write` | `write_file`, `edit_file`, `search_replace` |
| `group:fs` | `group:fs_read` + `group:fs_write` (all file ops) |
| `group:runtime` | `bash`, `agent_log` |
| `group:web` | `web_fetch`, `web_search` |
| `group:sessions` | `sessions_list`, `sessions_history`, `sessions_send`, `session_status` |
| `group:sessions_spawn` | `sessions_spawn` (separated for security — allows creating sub-agents) |
| `group:memory` | `memory`, `get_secret`, `rag_search` |
| `group:context` | `context`, `status` |
| `group:introspection` | `agents_list`, `gateway`, `agent_log`, `execution_flow` |

### Auto-Injected Tools

Some tools are added automatically and bypass the allowlist:

| Tool | Condition | Injected By |
|------|-----------|-------------|
| `get_secret` | Agent has secrets in `agent_secrets` table | `agent_adapter.go` WireSecretTool + `message/service.go` |
| `rag_search` | Agent has RAG configured | `agent_adapter.go` WireRAGSearchTool |

### Tool Filtering Decision Tree

```
IsToolAllowed(name):
  1. Is tool in denylist? --> DENY
  2. Is allowlist configured? --> Only allow if in allowlist
  3. No restrictions? --> ALLOW
```

### Current Gaps

- No RBAC on tool assignment: any workspace admin can assign any tool to any agent (including `bash`)
- Tool access is silently dropped if not in allowlist (no audit log)
- No concept of tool risk level or locality (local vs remote)
- Special tools (`sessions_send`, `memory`, `get_secret`, etc.) have dedicated code paths in `executeTool()` that bypass generic filtering (see Section 8)
- Cross-agent spawning (`sessions_spawn`) currently allows all (see Section 8)

## 5. Sandbox

**Package:** `pkg/sandbox/`

### Scopes

Each agent has a `sandbox_scope` (stored in `buddies` table):

| Scope | Access |
|-------|--------|
| `user` | `/sandbox/user/{initiatingUserID}/` only |
| `channel` | User path + `/sandbox/channel/{channelID}/` |
| `workspace` | User + Channel + `/sandbox/workspace/{workspaceID}/` |

### Enforcement

- Tools with `FunctionWithContext` receive a `SandboxContext`
- `SandboxContext.CanAccessPath(path)` checks prefix against allowed paths
- `SandboxContext.ValidatePath(path)` returns error if access denied
- `SandboxContext.ResolvePath(path)` maps virtual paths to `~/.memdoor/sandbox/` filesystem
- `InitiatingUserID` never changes in A2A chains (the human who started the request)
- `CurrentAgentID` tracks the currently executing agent

### What's Sandboxed

File operations (`read_file`, `write_file`, `edit_file`, `list_files`, `glob`, `grep`) use `FunctionWithContext` for sandbox enforcement. Tools like `bash` and `web_search` use `Function` (legacy, no sandbox).

## 6. Agent-to-Agent (A2A) Policy

**File:** `pkg/authorization/a2a_policy.go`

### Policy Structure

```go
A2APolicy {
    Enabled: bool          // Global kill switch
    Allow: []A2ARule       // Whitelist rules (default: deny all)
    MaxPingPongTurns: int  // 0 = fire-and-forget, max 10
}

A2ARule {
    From: string  // Source agent or "*"
    To:   string  // Target agent or "*"
}
```

### Default: DENY ALL

A2A is disabled by default. Must be explicitly enabled with rules.

### Mention Depth Limiting

**File:** `pkg/shared/execution_context.go`

Independent of A2A policy, `ExecutionContext.MentionDepth` prevents infinite agent chains:
- 0 = user message
- 1 = agent mentioned by user
- 2 = agent mentioned by agent (max, stops here)

`CanMentionAgents()` returns false when depth >= 2.

## 7. Agent Secrets Isolation

**File:** `tools/secret_tool.go`, `pkg/repository/sqlite/agent_secrets.go`

- Secrets are scoped by `agent_id` in the `agent_secrets` table
- `get_secret` tool uses `SandboxContext.CurrentAgentID` to enforce isolation
- Agent A cannot read Agent B's secrets
- Values encrypted at rest with AES-256-GCM (`pkg/secrets/crypto.go`)
- API never returns secret values, only names
- System prompt lists available secret names (not values)

## 8. Known Gaps

### Fixed: Auth Checks Added to Endpoints

The following endpoints previously had missing auth checks and have been fixed:

| Endpoint | Handler | Fix |
|----------|---------|-----|
| `DELETE /api/channels/:id` | `handleDeleteRoom` | Now requires `IsWorkspaceAdmin()`. |
| `POST /api/messages/:id/reactions` | `handleReactions` | Now uses authenticated `actorID` instead of trusting request body `user_id`. |
| `DELETE /api/messages/:id/reactions` | `handleReactions` | Now uses authenticated `actorID`. Users can only remove their own reactions. |
| `GET /api/search` | `handleSearch` | Now requires authentication. TODO: filter results by accessible channels. |
| `GET /api/messages/:id` | `handleGetSingleMessage` | Now requires auth + `CanReadChannel()` on the message's channel. |
| `POST /api/messages/:id/read` | `handleMarkMessageRead` | Now requires authentication. |

### High: Sandbox Gaps

| Tool | Issue |
|------|-------|
| `chrome_devtools` | Only has `Function`, no `FunctionWithContext`. Browser automation bypasses sandbox entirely — can access any URL, take screenshots, fill forms without scope enforcement. |
| `bash` | Has `FunctionWithContext` but sandbox enforcement for shell commands is limited — hard to restrict filesystem access from arbitrary shell commands. |

### Fixed: Tool Filtering vs Execution Mismatch

`executeTool()` now checks `allowed_tools` from context before executing any tool (including special tools like `sessions_send`, `memory`, `get_secret`). If a tool isn't in the filtered allowlist from `getFilteredTools()`, execution is blocked with a warning log.

### Fixed: Cross-Agent Spawning

`sessions_spawn` now checks the A2A policy via `A2AChecker` interface before allowing cross-agent spawning. If no policy is configured, spawning is allowed for backward compatibility. When A2A policy is configured, `checkCrossAgentPermissions()` calls `a2aChecker.IsAllowed(from, to)`.

### Fixed: Agent Channel Membership

Agents are now auto-joined to channels when @mentioned. In `validateMentions()` (message service), if a mentioned agent isn't a channel member, it's automatically added as a `member` role (system operation, no auth required). The `executeAgentMention()` path in `chat_server.go` also auto-joins before execution.

## File Reference

| File | Purpose |
|------|---------|
| `pkg/authorization/permissions.go` | Permission types, roles, authorization service |
| `pkg/authorization/middleware.go` | Auth middleware, token extraction |
| `pkg/authorization/a2a_policy.go` | Agent-to-agent communication policy |
| `pkg/auth/service.go` | Login, register, token verification |
| `pkg/auth/interfaces.go` | Auth service and repository interfaces |
| `pkg/shared/execution_context.go` | Request-scoped auth context |
| `pkg/sandbox/scope.go` | Sandbox scope definitions |
| `pkg/sandbox/context.go` | Sandbox context with path enforcement |
| `pkg/sandbox/path.go` | Path access control |
| `pkg/domain/models.go` | User roles (admin/user) |
| `gateway/config/tool_policy.go` | Tool profiles and groups |
| `gateway/config/types.go` | Tool filtering logic (allow/deny) |
| `gateway/agent_adapter.go` | Runtime tool filtering, secret/RAG wiring |
| `gateway/auth_gateway_adapter.go` | HTTP auth endpoints |
| `gateway/chat_server.go` | Channel/message/agent/secret endpoint auth |
| `tools/secret_tool.go` | get_secret tool with agent isolation |
| `pkg/secrets/crypto.go` | AES-256-GCM encryption for secrets at rest |
