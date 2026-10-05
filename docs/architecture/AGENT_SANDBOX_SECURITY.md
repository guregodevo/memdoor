# Agent Sandbox Security Architecture

**Status:** **PARTIALLY IMPLEMENTED** — the scope model (user / channel / workspace) and tool-level path scoping are real (`tools/agent.go` `ToolDefinition.FunctionWithContext`, `pkg/sandbox`); container isolation and a Slack creation UI were designed here but never built (see the notes in those sections).
**Last Updated:** 2026-08-28 (status corrected; design written 2026-03-05)
**Priority:** 🔴 **CRITICAL**

---

## Overview

Memdoor implements a **scope-based sandbox architecture** that provides multi-tenant isolation for AI agents while allowing controlled access to user data and company resources.

**Core Principle:** Every agent has a **sandbox scope** that determines what data it can access and whose credentials it can use.

---

## Sandbox Scope Hierarchy

### Three-Tier Isolation Model

```
┌─────────────────────────────────────────────────────────────┐
│                    PLATFORM LEVEL                           │
│  (Hard limits, cannot be exceeded by any workspace)         │
│  • Max 4GB memory per agent                                 │
│  • Max 4 CPUs per agent                                     │
│  • Max 300 seconds runtime                                  │
│  • Max 10,000 actions per day                               │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                  WORKSPACE LEVEL                            │
│  (Company-wide settings, configurable by workspace admin)   │
│  • Default resource limits for new agents                   │
│  • Maximum limits any agent can request                     │
│  • Allowed tools for workspace                              │
│  • Network policy (disabled/restricted/enabled)             │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                   CHANNEL LEVEL (NEW)                       │
│  (Channel-specific settings, configurable by channel admin) │
│  • Max agent resources for this channel                     │
│  • Allowed tools in this channel                            │
│  • Network access policy                                    │
│  • Agent creation policy (who can create)                   │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                     AGENT LEVEL                             │
│  (Per-agent configuration, set at creation)                 │
│  • Sandbox scope: user/channel/workspace ← KEY              │
│  • Requested resources (must be <= channel max)             │
│  • Enabled tools (must be subset of channel tools)          │
│  • Network access (must be within channel policy)           │
└─────────────────────────────────────────────────────────────┘
```

---

## Sandbox Scope Types

### 1. User Scope 🔒 (Most Secure)

**Agent accesses ONLY the initiating user's private data.**

#### Characteristics:
- **Filesystem:** `/user/{initiating_user_id}/`
- **Credentials:** Only initiating user's OAuth tokens (Gmail, Calendar, Drive)
- **Data Access:** User's private files, emails, calendar
- **Cross-User Isolation:**  YES - Bob cannot access Alice's data even if he @mentions the same agent

#### Use Cases:
- Personal Gmail assistant
- Calendar management bot
- Private note-taking agent
- Individual task manager

#### Example:
```go
agent := &domain.Agent{
    Name:         "my-gmail-assistant",
    ChannelID:    "dm-alice",
    SandboxScope: sandbox.ScopeUser,  // ← User scope
    Tools:        []string{"gmail_search", "calendar_read"},
}

// When Alice @mentions this agent:
// - Uses Alice's Gmail token
// - Accesses /user/alice/ filesystem
// - Cannot see Bob's emails

// When Bob @mentions this agent:
// - Uses Bob's Gmail token
// - Accesses /user/bob/ filesystem
// - Cannot see Alice's emails
```

---

### 2. Channel Scope  (Shared Team Data)

**Agent accesses channel-shared data and can use channel-level credentials.**

#### Characteristics:
- **Filesystem:** `/channel/{channel_id}/` + `/user/{initiating_user_id}/` (read-only)
- **Credentials:** Channel-scoped OAuth tokens (shared Slack bot, team API keys)
- **Data Access:** Team knowledge base, shared documents, channel files
- **Cross-Channel Isolation:**  YES - #engineering agent cannot access #marketing data

#### Use Cases:
- Team knowledge base bot
- Channel-specific assistant
- Shared document search
- Team task tracker

#### Example:
```go
agent := &domain.Agent{
    Name:         "team-knowledge-bot",
    ChannelID:    "channel-engineering",
    SandboxScope: sandbox.ScopeChannel,  // ← Channel scope
    Tools:        []string{"read_channel_files", "web_search"},
}

// When anyone in #engineering @mentions this agent:
// - Accesses /channel/engineering/ (shared docs)
// - Can read team knowledge base
// - Cannot access other channels' data
// - Cannot access users' private Gmail/Calendar
```

---

### 3. Workspace Scope 🏢 (Company-Wide Access)

**Agent accesses workspace-wide data using company-level credentials.**

#### Characteristics:
- **Filesystem:** `/workspace/{workspace_id}/` + `/channel/{channel_id}/` + `/user/{initiating_user_id}/` (all accessible)
- **Credentials:** Workspace-level credentials (company API keys, admin tokens)
- **Data Access:** Company-wide analytics, all channels, aggregate data
- **Approval Required:**  YES - Workspace admin must approve creation

#### Use Cases:
- Company analytics dashboard
- Cross-team data aggregation
- Admin tools
- Compliance reporting

#### Example:
```go
agent := &domain.Agent{
    Name:         "company-analytics",
    ChannelID:    "channel-admin",
    SandboxScope: sandbox.ScopeWorkspace,  // ← Workspace scope
    Tools:        []string{"analytics", "database_query"},
    CreatedBy:    workspaceAdminUserID,  // ← Must be admin
}

// When used:
// - Accesses /workspace/acme-corp/ (company-wide data)
// - Uses workspace-level credentials
// - Can aggregate data across all channels
// - Requires workspace admin approval
```

---

## Security Context Propagation

### SandboxContext Structure

**Every operation includes complete security context:**

```go
type SandboxContext struct {
    // Tenant isolation
    WorkspaceID      uuid.UUID  // Which company (Acme Corp)
    ChannelID        uuid.UUID  // Which Slack channel (#engineering)

    // User identity (NEVER changes during A2A chain)
    InitiatingUserID uuid.UUID  // Who started the request (Alice)

    // Agent identity
    CurrentAgentID   string     // Current agent in chain (agent:writer)

    // Sandbox scope (determines access level)
    AgentScope       SandboxScope  // user/channel/workspace
}
```

### Context Flow Through A2A Chain

**Critical: InitiatingUserID NEVER changes, even through multi-agent chains.**

```
User (Alice) → Agent A → Agent B → Agent C
    ↓              ↓         ↓         ↓
InitiatingUserID = alice (preserved throughout)
CurrentAgentID changes: agent-a → agent-b → agent-c
AgentScope: Each agent has its own scope
```

**Example A2A Chain:**
```go
// Alice asks agent:writer to help
// Context: {workspace: acme, channel: eng, user: alice, agent: writer, scope: user}

// agent:writer delegates to agent:researcher
a2aMessage := &A2AMessage{
    FromAgent:        "agent:writer",
    ToAgent:          "agent:researcher",
    InitiatingUserID: ctx.InitiatingUserID,  // ← Preserved: alice
    Content:          "Research topic X",
}

// agent:researcher executes with:
// Context: {workspace: acme, channel: eng, user: alice, agent: researcher, scope: channel}
// - Still knows Alice initiated this
// - Uses its own scope (channel)
// - Cannot use Alice's Gmail even if it wanted to (scope doesn't allow)
```

---

## Tool Execution Security

### Tool Scope Requirements

**Every tool declares its required scope:**

```go
type ToolDefinition struct {
    Name             string
    Description      string
    RequiredScope    sandbox.SandboxScope  // ← What scope this tool needs
    NetworkRequired  bool
    ParameterSchema  ParameterSchema
    ExecuteFunc      ToolExecutor
}
```

### Scope Validation

**Tools are only available to agents with sufficient scope:**

| Tool | Required Scope | Available To |
|------|---------------|--------------|
| `gmail_search` | User |  User scope agents only |
| `calendar_read` | User |  User scope agents only |
| `read_channel_files` | Channel |  Channel + Workspace scope agents |
| `company_analytics` | Workspace |  Workspace scope agents only |
| `web_search` | User |  All agents (lowest scope) |

**Validation at Agent Creation:**
```go
func (s *AgentService) CreateAgent(req *CreateAgentRequest) error {
    toolRegistry := s.getToolRegistry()

    for _, toolName := range req.Tools {
        tool := toolRegistry.GetTool(toolName)

        // Check tool's required scope <= agent's scope
        if tool.RequiredScope > req.SandboxScope {
            return fmt.Errorf(
                "tool '%s' requires scope '%s' but agent has scope '%s'",
                toolName,
                tool.RequiredScope,
                req.SandboxScope,
            )
        }
    }

    return nil
}
```

---

## Credential Isolation

### User-Scoped Credentials

**User-scoped tools ONLY use the initiating user's credentials:**

```go
// GmailSearchTool - User-scoped only
func (t *GmailSearchTool) Execute(params map[string]interface{}, ctx SandboxContext) (string, error) {
    // Enforce scope requirement
    if ctx.AgentScope != sandbox.ScopeUser {
        return "", fmt.Errorf(
            "Gmail tool requires user-scoped agent (current scope: %s)",
            ctx.AgentScope,
        )
    }

    // Fetch INITIATING user's Gmail token (not current agent's, not channel's)
    token, err := t.credentialStore.GetUserCredential(
        ctx.WorkspaceID,
        ctx.InitiatingUserID,  // ← Always the user who started request
        "gmail",
    )
    if err != nil {
        return "", fmt.Errorf("Gmail not connected for user %s", ctx.InitiatingUserID)
    }

    // Search using ONLY this user's token
    return t.searchGmail(token, params["query"].(string))
}
```

### Channel-Scoped Credentials

**Channel-scoped tools use channel-level credentials:**

```go
func (t *ChannelKnowledgeTool) Execute(params map[string]interface{}, ctx SandboxContext) (string, error) {
    // Channel or workspace scope required
    if ctx.AgentScope < sandbox.ScopeChannel {
        return "", fmt.Errorf("requires channel or workspace scope")
    }

    // Use channel-level credentials (e.g., shared Slack bot token)
    token, err := t.credentialStore.GetChannelCredential(
        ctx.WorkspaceID,
        ctx.ChannelID,
        "slack_bot",
    )

    return t.searchChannelDocs(token, params["query"].(string))
}
```

### Workspace-Scoped Credentials

**Workspace-scoped tools use company-wide credentials:**

```go
func (t *AnalyticsTool) Execute(params map[string]interface{}, ctx SandboxContext) (string, error) {
    // Workspace scope required
    if ctx.AgentScope != sandbox.ScopeWorkspace {
        return "", fmt.Errorf("requires workspace scope")
    }

    // Use workspace-level credentials (e.g., company API key)
    apiKey, err := t.credentialStore.GetWorkspaceCredential(
        ctx.WorkspaceID,
        "analytics_api",
    )

    return t.queryAnalytics(apiKey, params["metric"].(string))
}
```

---

## Filesystem Isolation

### Directory Structure

```
/sandbox/
├── workspace/{workspace_id}/        # Company-wide data
│   ├── analytics/
│   ├── reports/
│   └── shared/
├── channel/{channel_id}/            # Channel-shared data
│   ├── knowledge-base/
│   ├── docs/
│   └── uploads/
└── user/{user_id}/                  # User private data
    ├── gmail/
    ├── calendar/
    └── private/
```

### Path Access Control

**Based on agent scope:**

```go
func (ctx SandboxContext) CanAccessPath(path string) bool {
    switch ctx.AgentScope {
    case ScopeUser:
        // Can only access own user directory
        allowedPrefix := fmt.Sprintf("/user/%s/", ctx.InitiatingUserID)
        return strings.HasPrefix(path, allowedPrefix)

    case ScopeChannel:
        // Can access channel directory and user's directory
        channelPrefix := fmt.Sprintf("/channel/%s/", ctx.ChannelID)
        userPrefix := fmt.Sprintf("/user/%s/", ctx.InitiatingUserID)
        return strings.HasPrefix(path, channelPrefix) ||
               strings.HasPrefix(path, userPrefix)

    case ScopeWorkspace:
        // Can access workspace, channel, and user directories
        workspacePrefix := fmt.Sprintf("/workspace/%s/", ctx.WorkspaceID)
        channelPrefix := fmt.Sprintf("/channel/%s/", ctx.ChannelID)
        userPrefix := fmt.Sprintf("/user/%s/", ctx.InitiatingUserID)
        return strings.HasPrefix(path, workspacePrefix) ||
               strings.HasPrefix(path, channelPrefix) ||
               strings.HasPrefix(path, userPrefix)

    default:
        return false
    }
}
```

### File Tool Example

```go
func (t *ReadFileTool) Execute(params map[string]interface{}, ctx SandboxContext) (string, error) {
    path := params["path"].(string)

    // Check if agent's scope allows accessing this path
    if !ctx.CanAccessPath(path) {
        return "", fmt.Errorf(
            "access denied: agent scope '%s' cannot access path '%s'",
            ctx.AgentScope,
            path,
        )
    }

    // Read file from scope-appropriate sandbox
    sandboxRoot := ctx.GetSandboxPath()
    fullPath := filepath.Join(sandboxRoot, path)

    content, err := os.ReadFile(fullPath)
    if err != nil {
        return "", fmt.Errorf("read file: %w", err)
    }

    return string(content), nil
}
```

---

## Container Isolation — NEVER BUILT

The original design put every tool execution in a container with a
read-only root, resource limits and a per-scope network mode. None of
that exists: Memdoor is a single static binary with no container
runtime dependency, and the only isolation in the code is the
tool-level path scoping described above (`pkg/sandbox`,
`tools/agent.go`). Treat every "container" claim elsewhere in
this document as unimplemented.

---

## Database Schema

### Agent Configuration Table

```sql
-- Agents table with sandbox scope
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,

    -- Agent identity
    name VARCHAR(100) NOT NULL,
    avatar_emoji VARCHAR(10) DEFAULT '',

    -- Sandbox scope (CRITICAL SECURITY FIELD)
    sandbox_scope TEXT NOT NULL DEFAULT 'user'
        CHECK (sandbox_scope IN ('user', 'channel', 'workspace')),

    -- AI configuration. model_name/model_provider columns were dropped
    -- in the 2026-05 byok refactor: the model comes from a provider on
    -- the person's own key.
    system_prompt TEXT,
    tools JSONB DEFAULT '[]',

    -- Resource limits (must be <= channel max)
    max_memory_mb INTEGER NOT NULL,
    max_cpus REAL NOT NULL,
    max_runtime_seconds INTEGER NOT NULL,

    -- Network policy
    network_mode TEXT NOT NULL DEFAULT 'disabled'
        CHECK (network_mode IN ('disabled', 'restricted', 'enabled')),

    -- Metadata
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Constraints
    UNIQUE(workspace_id, channel_id, name)
);

CREATE INDEX idx_agents_workspace ON agents(workspace_id);
CREATE INDEX idx_agents_channel ON agents(channel_id);
CREATE INDEX idx_agents_scope ON agents(sandbox_scope);
```

### Tool Definitions Table

```sql
-- Tool registry with scope requirements
CREATE TABLE tool_definitions (
    name TEXT PRIMARY KEY,
    description TEXT NOT NULL,

    -- Scope requirement (critical for security)
    required_scope TEXT NOT NULL
        CHECK (required_scope IN ('user', 'channel', 'workspace')),

    -- Network requirements
    network_required BOOLEAN DEFAULT false,

    -- Parameter schema
    parameter_schema JSONB NOT NULL,

    created_at TIMESTAMP DEFAULT NOW()
);

-- Example tool data
INSERT INTO tool_definitions (name, description, required_scope, network_required, parameter_schema) VALUES
('gmail_search', 'Search Gmail messages', 'user', true, '{"query": {"type": "string"}}'),
('calendar_read', 'Read Google Calendar', 'user', true, '{"date": {"type": "string"}}'),
('read_channel_files', 'Read channel shared files', 'channel', false, '{"path": {"type": "string"}}'),
('company_analytics', 'Access company analytics', 'workspace', true, '{"metric": {"type": "string"}}'),
('web_search', 'Search the web', 'user', true, '{"query": {"type": "string"}}');
```

### Credentials Table

```sql
-- User OAuth credentials (user-scoped)
CREATE TABLE user_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Service (gmail, calendar, drive, etc.)
    service VARCHAR(50) NOT NULL,

    -- Encrypted OAuth tokens
    access_token_encrypted BYTEA NOT NULL,
    refresh_token_encrypted BYTEA,

    -- Metadata
    scopes JSONB DEFAULT '[]',
    expires_at TIMESTAMP,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(workspace_id, user_id, service)
);

-- Channel credentials (channel-scoped)
CREATE TABLE channel_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,

    -- Service (slack_bot, team_api, etc.)
    service VARCHAR(50) NOT NULL,

    -- Encrypted credentials
    credential_encrypted BYTEA NOT NULL,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(workspace_id, channel_id, service)
);

-- Workspace credentials (workspace-scoped)
CREATE TABLE workspace_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    -- Service (analytics_api, company_db, etc.)
    service VARCHAR(50) NOT NULL,

    -- Encrypted credentials
    credential_encrypted BYTEA NOT NULL,

    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(workspace_id, service)
);
```

---

## Agent Creation UI — NEVER BUILT

The design included a chat-platform modal (Slack) for picking an
agent's sandbox scope and tools. Memdoor has no chat-platform
adapters; agents are created from the CLI (`memdoor agent add`,
`memdoor agent tools`) and the web UI is login/admin only.

---

## Audit Logging

### Security Events

**All security-critical operations are logged:**

```sql
CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES workspaces(id),

    -- Actor
    user_id UUID REFERENCES users(id),
    agent_id UUID REFERENCES agents(id),

    -- Action
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id UUID,

    -- Security context
    sandbox_scope TEXT,
    initiating_user_id UUID,
    channel_id UUID,

    -- Details
    details JSONB DEFAULT '{}',

    -- Metadata
    ip_address INET,
    user_agent TEXT,

    created_at TIMESTAMP DEFAULT NOW()
);

-- Example audit events
INSERT INTO audit_log (workspace_id, agent_id, action, sandbox_scope, details) VALUES
('ws-123', 'agent-456', 'tool.execute', 'user', '{"tool": "gmail_search", "user": "alice"}'),
('ws-123', 'agent-789', 'credential.access', 'channel', '{"service": "slack_bot", "channel": "eng"}'),
('ws-123', 'agent-abc', 'file.read', 'workspace', '{"path": "/workspace/reports/Q4.pdf"}');
```

---

## Security Checklist

### Agent Creation
- [ ] Sandbox scope validated against channel policy
- [ ] Tools compatible with requested scope
- [ ] Resource limits within channel/workspace max
- [ ] Workspace-scoped agents require admin approval
- [ ] Audit log entry created

### Tool Execution
- [ ] Tool's required scope <= agent's scope
- [ ] Credentials fetched for correct scope level (user/channel/workspace)
- [ ] Filesystem paths validated against scope permissions
- [ ] Network access matches tool requirements and policy
- [ ] Execution timeout enforced
- [ ] Audit log entry created

### A2A Message Passing
- [ ] InitiatingUserID preserved through chain
- [ ] Each agent uses its own scope
- [ ] No privilege escalation possible
- [ ] Context includes all required fields
- [ ] Audit trail tracks full chain

---

## Attack Vectors & Mitigations

### 1. Credential Theft
**Attack:** Bob tries to access Alice's Gmail token
**Mitigation:** User-scoped tools ONLY fetch credentials for InitiatingUserID
```go
token, err := store.GetUserCredential(ctx.WorkspaceID, ctx.InitiatingUserID, "gmail")
// Cannot use ctx.CurrentAgentID or arbitrary user_id
```

### 2. Scope Escalation
**Attack:** User-scoped agent tries to access workspace data
**Mitigation:** Path access validation enforced at filesystem level
```go
if !ctx.CanAccessPath("/workspace/secret.pdf") {
    return ErrAccessDenied  // User-scoped agent blocked
}
```

### 3. Tool Bypass
**Attack:** Agent tries to use tool outside its scope
**Mitigation:** Tool execution validates scope requirements
```go
if tool.RequiredScope > agent.SandboxScope {
    return ErrInsufficientScope
}
```

### 4. A2A Privilege Escalation
**Attack:** User-scoped agent delegates to workspace-scoped agent to steal data
**Mitigation:** Each agent enforces its own scope; cannot inherit higher privileges

### 5. Container Breakout
Not applicable — there is no container (see "Container Isolation — NEVER BUILT"). Tool execution runs in the gateway process; the boundary is path scoping plus the workspace `tool_guards` regex rules (`docs/reference/CLI.md`).

---

## Summary

**Sandbox scope-based security provides:**

 **Multi-tenant isolation** - Workspace-level data separation
 **User privacy** - User-scoped agents protect personal data
 **Team collaboration** - Channel-scoped agents enable shared workflows
 **Company analytics** - Workspace-scoped agents access aggregate data
 **Credential isolation** - Users cannot access each other's OAuth tokens
 **Audit trail** - Complete logging of all security events
 **Defense in depth** - Application + database isolation (no container layer)

**Remaining work** (the scope model, `sandbox_scope` column and tool scope requirements are done — see `docs/SANDBOX_IMPLEMENTATION_GUIDE.md`):
1. Implement credential store with scope-level access
2. Add audit logging for all security events
3. Write security tests (scope isolation, credential theft prevention)
