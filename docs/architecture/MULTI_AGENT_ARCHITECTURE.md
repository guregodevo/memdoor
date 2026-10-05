# Multi-Agent Architecture Design

**Date**: 2026-02-19
**Status**: Design Phase
**Priority**: HIGH (Phase 1, Week 22)

> ⚠ **Historical design.** YAML/JSON config examples below showing
> per-agent `model: "anthropic/..."` fields are historical: the agent
> schema has no `Model` field; the model comes from a provider on the
> person's own key (`/model` pins one). The multi-agent structure
> (per-agent workspace, tool palette, sandbox scope) is
> unchanged.

This document outlines the design for implementing OpenClaw-style multi-agent support in Memdoor.

---

## Executive Summary

OpenClaw supports **multiple isolated agents** within a single gateway instance, each with:
- Separate workspaces and configuration
- Independent session storage
- Dedicated channel accounts (accountId)
- Deterministic message routing via bindings

**Memdoor Current State**: Single agent ("main") only

**Goal**: Implement full multi-agent support to enable:
1. Multiple people sharing one gateway
2. Different personalities per agent
3. Different models per agent
4. Isolated workspaces and auth

---

## OpenClaw Multi-Agent Architecture

### Directory Structure

```
~/.openclaw/
├── openclaw.json                    # Main config with agents.list[] and bindings[]
├── agents/
│   ├── main/
│   │   ├── agent/                  # Agent state (auth, model cache, etc.)
│   │   └── sessions/               # Per-agent sessions SQLite
│   ├── work/
│   │   ├── agent/
│   │   └── sessions/
│   └── family/
│       ├── agent/
│       └── sessions/
├── workspace/                       # Default workspace (main agent)
├── workspace-work/                  # Work agent workspace
├── workspace-family/                # Family agent workspace
└── credentials/
    └── whatsapp/
        ├── default/                 # WhatsApp account "default"
        └── personal/                # WhatsApp account "personal"
```

### Session Key Format

**Pattern**: `agent:<agentId>:<rest>`

**Examples**:
- `agent:main:main` - Default agent, main session
- `agent:work:main` - Work agent, main session
- `agent:main:whatsapp:dm:+15551234567` - DM session
- `agent:main:discord:group:123456789` - Group session
- `agent:work:telegram:default:dm:tg:123456` - Per-account DM
- `agent:main:main:thread:abc123` - Thread session

**Key Components**:
1. **agentId**: Which agent "brain" to use
2. **mainKey**: Default session key (usually "main")
3. **channel**: Channel name (whatsapp, discord, etc.)
4. **accountId**: Channel account identifier
5. **peerKind**: dm, group, channel
6. **peerId**: User/group ID
7. **threadId**: Optional thread suffix

---

## Bindings System

**Purpose**: Route incoming messages to the correct agent

### Binding Structure

```typescript
{
  agentId: "work",
  match: {
    channel: "whatsapp",
    accountId: "personal",
    peer: {
      kind: "direct",
      id: "+15551234567"
    }
  }
}
```

### Routing Priority (Most-Specific Wins)

1. **Peer match** - Exact DM/group/channel ID
2. **ParentPeer match** - Thread inheritance
3. **GuildId + roles** - Discord role routing
4. **GuildId** - Discord server
5. **TeamId** - Slack workspace
6. **AccountId match** - Channel account
7. **Channel-level match** - Any message from channel
8. **Fallback** - Default agent (first in `agents.list[]` with `default: true`)

### Example Bindings Config

```json5
{
  agents: {
    list: [
      {
        id: "main",
        default: true,
        workspace: "~/.memdoor/workspace",
        model: "anthropic/claude-sonnet-4-5"
      },
      {
        id: "work",
        workspace: "~/.memdoor/workspace-work",
        model: "anthropic/claude-opus-4"
      },
      {
        id: "family",
        workspace: "~/.memdoor/workspace-family",
        tools: {
          allow: ["read", "sessions_list"],
          deny: ["write", "exec", "browser"]
        }
      }
    ]
  },

  bindings: [
    // Specific WhatsApp group to family agent
    {
      agentId: "family",
      match: {
        channel: "whatsapp",
        accountId: "personal",
        peer: { kind: "group", id: "120363999999999999@g.us" }
      }
    },

    // Work WhatsApp account to work agent
    {
      agentId: "work",
      match: { channel: "whatsapp", accountId: "work" }
    },

    // All Discord to main agent
    {
      agentId: "main",
      match: { channel: "discord" }
    },

    // Specific Telegram user to work agent
    {
      agentId: "work",
      match: {
        channel: "telegram",
        peer: { kind: "direct", id: "tg:123456789" }
      }
    }
  ]
}
```

---

## Memdoor Current Architecture

### Directory Structure (Current)

```
~/.memdoor/
├── agents/
│   └── main/                       # Only one agent
│       ├── agent.json              # Agent metadata
│       └── workspace -> ../../workspace/
├── channels/
│   └── whatsapp.db                 # WhatsApp session (global)
├── sessions/                       # Global sessions (not per-agent!)
│   └── sessions.db
├── workspace/                      # Global workspace
│   ├── memory/
│   ├── skills/
│   └── .git/
├── config.json                     # Gateway config
└── memdoor.yaml                  # User config (legacy)
```

### Session Management (Current)

**File**: `gateway/session.go`

```go
type SessionManager struct {
    sessions map[string]*Session
    mu       sync.RWMutex
}

type Session struct {
    ID           string
    AgentID      string
    CreatedAt    time.Time
    LastActivity time.Time
    Messages     []Message
}
```

**Issues**:
-  Sessions stored globally, not per-agent
-  No session key format (agent:agentId:rest)
-  Single workspace reference
-  No bindings system
-  No accountId support

---

## Implementation Plan

### Phase 1: Agent Infrastructure (Week 22)

#### 1.1 Agent Manager

**File**: `gateway/agents/manager.go`

```go
type Agent struct {
    ID         string
    Name       string
    Default    bool
    Workspace  string
    AgentDir   string
    Model      string
    Sandbox    *SandboxConfig
    Tools      *ToolsConfig
}

type AgentManager struct {
    agents     map[string]*Agent
    defaultID  string
    mu         sync.RWMutex
}

func (m *AgentManager) GetAgent(id string) (*Agent, error)
func (m *AgentManager) ListAgents() []*Agent
func (m *AgentManager) GetDefaultAgent() *Agent
```

#### 1.2 Session Key Parser

**File**: `gateway/agents/session_key.go`

```go
type ParsedSessionKey struct {
    AgentID   string
    MainKey   string
    Channel   string
    AccountID string
    PeerKind  string
    PeerID    string
    ThreadID  string
}

func ParseSessionKey(key string) (*ParsedSessionKey, error)
func BuildAgentMainSessionKey(agentID, mainKey string) string
func BuildAgentPeerSessionKey(params PeerSessionParams) string
```

#### 1.3 Bindings System

**File**: `gateway/agents/bindings.go`

```go
type Binding struct {
    AgentID string
    Match   BindingMatch
}

type BindingMatch struct {
    Channel   string
    AccountID string
    Peer      *PeerMatch
    GuildID   string
    TeamID    string
}

type PeerMatch struct {
    Kind string // "direct", "group", "channel"
    ID   string
}

type BindingResolver struct {
    bindings []Binding
    mu       sync.RWMutex
}

func (r *BindingResolver) ResolveAgent(msg *IncomingMessage) (string, error)
```

#### 1.4 Per-Agent Sessions

**File**: `gateway/agents/session_store.go`

```go
type AgentSessionStore struct {
    agentID string
    dbPath  string
    db      *sql.DB
}

func NewAgentSessionStore(agentID string) (*AgentSessionStore, error)
func (s *AgentSessionStore) GetSession(sessionKey string) (*Session, error)
func (s *AgentSessionStore) SaveSession(session *Session) error
```

**Database Path**: `~/.memdoor/agents/<agentId>/sessions/sessions.db`

### Phase 2: Configuration System (Week 22)

#### 2.1 Config Structure

**File**: `gateway/config/config.go`

```go
type Config struct {
    Agents   AgentsConfig
    Bindings []Binding
    Channels ChannelsConfig
    Tools    ToolsConfig
}

type AgentsConfig struct {
    List []AgentConfig
}

type AgentConfig struct {
    ID        string
    Name      string
    Default   bool
    Workspace string
    AgentDir  string
    Model     string
    Sandbox   *SandboxConfig
    Tools     *ToolsConfig
}
```

#### 2.2 Config Loader

**File**: `gateway/config/loader.go`

```go
func LoadConfig(path string) (*Config, error)
func WatchConfig(path string, onChange func(*Config)) error
func ValidateConfig(cfg *Config) error
```

**Config Paths**:
1. `~/.memdoor/memdoor.json` (primary, JSON5 format)
2. `~/.memdoor/config.yaml` (legacy, YAML format)
3. `MEMDOOR_CONFIG_PATH` env var

### Phase 3: Channel Account Support (Week 23)

#### 3.1 Account Manager

**File**: `gateway/channels/accounts.go`

```go
type AccountManager struct {
    accounts map[string]map[string]*Account // channelID -> accountID -> Account
    mu       sync.RWMutex
}

type Account struct {
    ID        string
    ChannelID string
    AuthDir   string
    Config    map[string]interface{}
}

func (m *AccountManager) GetAccount(channelID, accountID string) (*Account, error)
func (m *AccountManager) RegisterAccount(account *Account) error
```

#### 3.2 Update WhatsApp Adapter

**File**: `gateway/channels/whatsapp/adapter.go`

- Add `AccountID` field
- Support multiple WhatsApp accounts
- Store auth in `~/.memdoor/credentials/whatsapp/<accountId>/`

### Phase 4: Workspace Per Agent (Week 23)

#### 4.1 Workspace Manager

**File**: `gateway/workspace/agent_workspace.go`

```go
type AgentWorkspace struct {
    AgentID   string
    Path      string
    MemoryDir string
    SkillsDir string
    GitRepo   *git.Repository
}

func NewAgentWorkspace(agentID string, path string) (*AgentWorkspace, error)
func (w *AgentWorkspace) Initialize() error
func (w *AgentWorkspace) GetMemoryPath(date time.Time) string
```

**Workspace Paths**:
- Default: `~/.memdoor/workspace` (main agent)
- Named: `~/.memdoor/workspace-<agentId>`
- Custom: User-specified path in config

### Phase 5: Integration (Week 23)

#### 5.1 Update Server

**File**: `gateway/server.go`

```go
type Server struct {
    port          int
    agentManager  *agents.AgentManager
    bindings      *agents.BindingResolver
    sessionStores map[string]*agents.AgentSessionStore // agentID -> store
    // ... rest
}

func (s *Server) handleChannelMessage(msg *adapters.IncomingMessage) error {
    // 1. Resolve agent via bindings
    agentID, err := s.bindings.ResolveAgent(msg)

    // 2. Get agent config
    agent, err := s.agentManager.GetAgent(agentID)

    // 3. Build session key
    sessionKey := agents.BuildAgentPeerSessionKey(...)

    // 4. Get session store for this agent
    store := s.sessionStores[agentID]
    session, err := store.GetOrCreateSession(sessionKey)

    // 5. Execute with agent's runtime
    response, err := s.executeAgentJob(ctx, agent, session, msg.Text)

    // 6. Send response
    return s.channelRouter.SendMessage(msg.ChannelID, msg.ChannelUserID, response.Text)
}
```

---

## Migration Strategy

### Step 1: Add Multi-Agent Support (Non-Breaking)

**Week 22**:
1.  Add `AgentManager` (default to "main")
2.  Add `BindingResolver` (empty bindings = use default)
3.  Add per-agent session stores
4.  Keep existing single-agent behavior as default

**Backward Compatibility**:
- If no `agents.list[]` in config → create default "main" agent
- If no bindings → all messages go to default agent
- Existing sessions in `~/.memdoor/sessions/` → migrate to `~/.memdoor/agents/main/sessions/`

### Step 2: Enable Config File (Week 22)

1. Create `~/.memdoor/memdoor.json` loader
2. Support YAML fallback for legacy configs
3. Validate config structure
4. Add `memdoor config` commands

### Step 3: Add Multi-Account Support (Week 23)

1. Update channel adapters to support `accountId`
2. Store credentials per-account
3. Update bindings to match on `accountId`

### Step 4: Documentation (Week 23)

1. Multi-agent setup guide
2. Bindings examples
3. Migration guide from single to multi-agent

---

## Example Use Cases

### Use Case 1: Multiple People, One Gateway

**Scenario**: Alice and Bob share a Memdoor gateway

**Config**:
```json5
{
  agents: {
    list: [
      { id: "alice", workspace: "~/.memdoor/workspace-alice" },
      { id: "bob", workspace: "~/.memdoor/workspace-bob" }
    ]
  },
  bindings: [
    {
      agentId: "alice",
      match: {
        channel: "whatsapp",
        accountId: "alice-phone"
      }
    },
    {
      agentId: "bob",
      match: {
        channel: "whatsapp",
        accountId: "bob-phone"
      }
    }
  ]
}
```

**Benefits**:
-  Separate workspaces (no data mixing)
-  Separate auth/credentials
-  Separate session histories
-  Independent agent personalities

### Use Case 2: Different Models Per Context

**Scenario**: Fast agent for chat, Opus for deep work

**Config**:
```json5
{
  agents: {
    list: [
      {
        id: "chat",
        model: "anthropic/claude-sonnet-4-5",
        workspace: "~/.memdoor/workspace-chat"
      },
      {
        id: "opus",
        model: "anthropic/claude-opus-4",
        workspace: "~/.memdoor/workspace-opus"
      }
    ]
  },
  bindings: [
    {
      agentId: "chat",
      match: { channel: "whatsapp" }
    },
    {
      agentId: "opus",
      match: { channel: "telegram" }
    }
  ]
}
```

### Use Case 3: Restricted Family Bot

**Scenario**: Family WhatsApp group with tool restrictions

**Config**:
```json5
{
  agents: {
    list: [
      {
        id: "family",
        workspace: "~/.memdoor/workspace-family",
        tools: {
          allow: ["read", "sessions_list"],
          deny: ["write", "exec", "browser", "edit"]
        }
      }
    ]
  },
  bindings: [
    {
      agentId: "family",
      match: {
        channel: "whatsapp",
        peer: { kind: "group", id: "120363999999999999@g.us" }
      }
    }
  ]
}
```

---

## Testing Plan

### Unit Tests

1. **Session Key Parsing**
   - Parse various formats
   - Build session keys
   - Extract agent ID

2. **Bindings Resolution**
   - Peer matching
   - Channel matching
   - Account matching
   - Priority ordering

3. **Agent Manager**
   - Get agent by ID
   - List agents
   - Get default agent

### Integration Tests

1. **Multi-Agent Routing**
   - Message from WhatsApp account A → agent A
   - Message from WhatsApp account B → agent B
   - Fallback to default agent

2. **Session Isolation**
   - Agent A session != Agent B session
   - Same channel user → different agents → different sessions

3. **Workspace Isolation**
   - Agent A memory files != Agent B memory files
   - Separate git repos

---

## Success Criteria

### Week 22 Completion Checklist

- [ ] `AgentManager` implementation
- [ ] Session key parser
- [ ] Bindings resolver
- [ ] Per-agent session stores
- [ ] Config file loader (JSON5)
- [ ] Backward compatibility (default "main" agent)
- [ ] Session migration script
- [ ] Unit tests (80%+ coverage)

### Week 23 Completion Checklist

- [ ] Multi-account channel support
- [ ] Per-agent workspaces
- [ ] WhatsApp multi-account
- [ ] Integration tests
- [ ] Documentation
- [ ] Migration guide

---

## Open Questions

1. **Config reload**: Support hot reload like OpenClaw?
   - **Decision**: Yes, implement file watcher in Week 22

2. **Agent-to-agent messaging**: Support cross-agent sessions?
   - **Decision**: Not in Phase 1, add in Phase 3 if needed

3. **Per-agent models**: Allow different LLM providers?
   - **Decision**: Yes, each agent can specify model in config

4. **Shared skills**: Can agents share skill directories?
   - **Decision**: Yes, default to shared `~/.memdoor/workspace/skills/`, allow per-agent override

---

## References

- OpenClaw Multi-Agent Docs: `~/Dev/openclaw/docs/configuration/multi-agent.md`
- OpenClaw Session Keys: `~/Dev/openclaw/src/routing/session-key.ts`
- OpenClaw Bindings: `~/Dev/openclaw/src/routing/bindings.ts`
- OpenClaw Config: `~/Dev/openclaw/src/config/`
