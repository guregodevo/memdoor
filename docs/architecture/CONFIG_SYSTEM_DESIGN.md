# Memdoor Configuration System Design

**Date**: 2026-02-20
**Status**: Design Phase
**Priority**: HIGH (Week 22)
**OpenClaw Reference**: `~/.openclaw/openclaw.json`

> ⚠ **Historical design.** The agent schema has no `Model` field: the
> model comes from a provider on the person's own key
> ([`../features/PROVIDERS.md`](../features/PROVIDERS.md)). See
> `gateway/config/types.go` for the live schema.

This document outlines the design for Memdoor's JSON5 configuration system, based on OpenClaw's architecture.

---

## Executive Summary

**Goal**: Replace environment variable configuration with a structured JSON5 config file at `~/.memdoor/memdoor.json`.

**Key Benefits:**
- **Multi-Agent Support** - Multiple isolated agents with separate workspaces
- **Channel Routing** - Route different channels/accounts to different agents
- **Per-Agent Configuration** - Different models, tools, permissions per agent
- **Structured Validation** - Schema-based validation prevents misconfiguration
- **Hot Reload** - Config changes without full restart
- **Config Includes** - Split large configs across multiple files

**Current State**: Memdoor uses environment variables only
**Target State**: JSON5 config file with env var fallback

---

## Configuration File Location

**Primary**: `~/.memdoor/memdoor.json` (JSON5 format - comments + trailing commas allowed)

**Environment Override**: `MEMDOOR_CONFIG_PATH` environment variable

**Fallback Behavior**:
1. If `~/.memdoor/memdoor.json` exists → use it
2. If not → use environment variables (current behavior)
3. Config file values override environment variables

---

## Minimal Configuration (Recommended Starting Point)

```json5
{
  // Single agent, basic setup
  agents: {
    defaults: {
      workspace: "~/.memdoor/workspace",
    },
  },

  // WhatsApp DM allowlist
  channels: {
    whatsapp: {
      allowFrom: ["+15551234567"],
    },
  },
}
```

**Equivalent to current environment variables**:
```bash
# No config needed - this is the default behavior
# WhatsApp pairing + ~/.memdoor/workspace
```

---

## Multi-Agent Configuration Examples

### Example 1: Two Agents - Home & Work (Separate WhatsApp Accounts)

```json5
{
  agents: {
    list: [
      {
        id: "home",
        name: "Home Assistant",
        default: true,
        workspace: "~/.memdoor/workspace-home",
        agentDir: "~/.memdoor/agents/home/agent",
        model: "anthropic/claude-sonnet-4-5",
        identity: {
          name: "HomeBot",
          emoji: "🏠",
        },
      },
      {
        id: "work",
        name: "Work Assistant",
        workspace: "~/.memdoor/workspace-work",
        agentDir: "~/.memdoor/agents/work/agent",
        model: "anthropic/claude-opus-4-6",
        identity: {
          name: "WorkBot",
          emoji: "💼",
        },
      },
    ],
  },

  // Route WhatsApp accounts to agents
  bindings: [
    { agentId: "home", match: { channel: "whatsapp", accountId: "personal" } },
    { agentId: "work", match: { channel: "whatsapp", accountId: "business" } },
  ],

  // Two WhatsApp account configurations
  channels: {
    whatsapp: {
      accounts: {
        personal: {
          // Session stored in ~/.memdoor/channels/whatsapp/personal.db
          authDir: "~/.memdoor/credentials/whatsapp/personal",
        },
        business: {
          // Session stored in ~/.memdoor/channels/whatsapp/business.db
          authDir: "~/.memdoor/credentials/whatsapp/business",
        },
      },
    },
  },
}
```

**Key Features**:
- Two isolated agents with separate workspaces
- Two WhatsApp accounts (personal phone + work phone)
- Each agent has its own model, identity, and workspace
- Bindings route WhatsApp accounts to correct agent

**Session Keys**:
- `agent:home:main` - Home agent main session
- `agent:work:main` - Work agent main session
- `agent:home:whatsapp:dm:+15551234567` - Home WhatsApp DM
- `agent:work:whatsapp:dm:+15559876543` - Work WhatsApp DM

---

### Example 2: Split by Channel (WhatsApp → Fast, Telegram → Powerful)

```json5
{
  agents: {
    list: [
      {
        id: "chat",
        name: "Everyday Chat",
        workspace: "~/.memdoor/workspace-chat",
        model: "anthropic/claude-sonnet-4-5",
      },
      {
        id: "opus",
        name: "Deep Work",
        workspace: "~/.memdoor/workspace-opus",
        model: "anthropic/claude-opus-4-6",
      },
    ],
  },

  bindings: [
    { agentId: "chat", match: { channel: "whatsapp" } },
    { agentId: "opus", match: { channel: "telegram" } },
  ],

  channels: {
    whatsapp: {
      allowFrom: ["+15551234567"],
    },
    telegram: {
      enabled: true,
      botToken: "${TELEGRAM_BOT_TOKEN}",
      allowFrom: ["123456789"],
    },
  },
}
```

**Use Case**: Route WhatsApp (fast queries) to Sonnet, Telegram (deep work) to Opus.

---

### Example 3: One WhatsApp, Multiple Users via DM Routing

```json5
{
  agents: {
    list: [
      { id: "alice", workspace: "~/.memdoor/workspace-alice" },
      { id: "bob", workspace: "~/.memdoor/workspace-bob" },
    ],
  },

  // Route WhatsApp DMs by sender phone number
  bindings: [
    {
      agentId: "alice",
      match: {
        channel: "whatsapp",
        peer: { kind: "dm", id: "+15551230001" },
      },
    },
    {
      agentId: "bob",
      match: {
        channel: "whatsapp",
        peer: { kind: "dm", id: "+15551230002" },
      },
    },
  ],

  channels: {
    whatsapp: {
      dmPolicy: "allowlist",
      allowFrom: ["+15551230001", "+15551230002"],
    },
  },
}
```

**Use Case**: Two people sharing one WhatsApp bot number, isolated workspaces.

---

### Example 4: Family Agent with Restricted Tools

```json5
{
  agents: {
    list: [
      {
        id: "family",
        name: "Family Bot",
        workspace: "~/.memdoor/workspace-family",
        identity: { name: "FamilyBot", emoji: "👨‍👩‍👧‍👦" },

        // Mention gating for groups
        groupChat: {
          mentionPatterns: ["@family", "@familybot"],
        },

        // Restricted tool access
        tools: {
          allow: ["read", "sessions_list", "sessions_history"],
          deny: ["write", "edit", "exec", "bash"],
        },

        // Always sandboxed
        sandbox: {
          mode: "all",
          scope: "agent",
        },
      },
    ],
  },

  bindings: [
    {
      agentId: "family",
      match: {
        channel: "whatsapp",
        peer: { kind: "group", id: "120363999999999999@g.us" },
      },
    },
  ],

  channels: {
    whatsapp: {
      groups: { "*": { requireMention: true } },
    },
  },
}
```

**Key Features**:
- Read-only agent (no write/edit/exec)
- Sandbox isolation
- Mention gating for group messages
- Bound to specific WhatsApp group

---

## Full Configuration Schema

### Top-Level Structure

```json5
{
  // Environment variables (inline fallback). Provider keys live in the
  // environment or ~/.memdoor/providers.json, not here.
  env: {
    vars: {
      // example: non-LLM secrets, e.g. SMTP creds
    },
  },

  // Gateway server settings
  gateway: {
    mode: "local",           // "local" | "remote"
    port: 18789,
    bind: "loopback",        // "loopback" | "0.0.0.0"
    auth: {
      mode: "token",         // "none" | "token"
      token: "gateway-secret",
    },
  },

  // Logging configuration
  logging: {
    level: "info",           // "debug" | "info" | "warn" | "error"
    file: "~/.memdoor/logs/memdoor.log",
  },

  // Agent configuration
  agents: {
    // Default settings for all agents
    defaults: {
      workspace: "~/.memdoor/workspace",
      model: "anthropic/claude-sonnet-4-5",
      imageModel: "anthropic/claude-sonnet-4-5",
      thinkingDefault: "low",
      verboseDefault: "off",
      timeoutSeconds: 600,
      maxConcurrent: 3,
    },

    // Individual agent definitions
    list: [
      {
        id: "main",
        name: "Main Agent",
        default: true,
        workspace: "~/.memdoor/workspace",
        agentDir: "~/.memdoor/agents/main/agent",
        model: "anthropic/claude-sonnet-4-5",
        identity: {
          name: "Memdoor",
          emoji: "🦞",
          theme: "helpful lobster",
        },
        tools: {
          allow: ["*"],      // All tools allowed
          deny: [],          // None denied
        },
        sandbox: {
          mode: "off",       // "off" | "all" | "non-main"
          scope: "agent",    // "agent" | "session"
        },
        groupChat: {
          mentionPatterns: ["@memdoor", "@bot"],
        },
        subagents: {
          allowAgents: ["work", "data"],
          model: "anthropic/claude-haiku-3-5",
        },
      },
    ],
  },

  // Message routing bindings
  bindings: [
    {
      agentId: "main",
      match: {
        channel: "whatsapp",
        accountId: "personal",
        peer: { kind: "dm", id: "+15551234567" },
      },
    },
  ],

  // Agent-to-agent communication policy
  tools: {
    agentToAgent: {
      enabled: false,
      allow: [
        { from: "main", to: "work" },
        { from: "main", to: "data" },
      ],
      maxPingPongTurns: 5,
    },
  },

  // Channel configurations
  channels: {
    whatsapp: {
      enabled: true,
      dmPolicy: "pairing",   // "pairing" | "allowlist" | "open"
      allowFrom: [],
      groupPolicy: "allowlist",
      groupAllowFrom: [],
      groups: {
        "*": { requireMention: true },
      },
      accounts: {
        personal: {
          authDir: "~/.memdoor/credentials/whatsapp/personal",
        },
      },
    },

    telegram: {
      enabled: false,
      botToken: "${TELEGRAM_BOT_TOKEN}",
      allowFrom: [],
      groups: { "*": { requireMention: true } },
    },

    slack: {
      enabled: false,
      botToken: "${SLACK_BOT_TOKEN}",
      appToken: "${SLACK_APP_TOKEN}",
      channels: {},
      dm: { enabled: true, allowFrom: [] },
    },
  },

  // Session behavior
  session: {
    scope: "per-sender",     // "per-sender" | "per-channel-peer"
    reset: {
      mode: "daily",         // "daily" | "idle" | "manual"
      atHour: 4,
      idleMinutes: 60,
    },
    resetTriggers: ["/new", "/reset"],
    typingIntervalSeconds: 5,
  },

  // Config file includes
  $include: "./additional-config.json5",
}
```

---

## Migration Path: Environment Variables → Config File

### Current Environment Variables

```bash
# Gateway
GATEWAY_PORT=18789
GATEWAY_AUTH_TOKEN=secret

# WhatsApp
WHATSAPP_DB_PATH=~/.memdoor/channels/whatsapp.db

# Workspace
WORKSPACE_PATH=~/.memdoor/workspace
```

Provider keys are read from the environment or `memdoor connect`
(`~/.memdoor/providers.json`), not from this config.

### Equivalent JSON5 Config

```json5
{
  // No LLM API key fields — local-only, no cloud provider.
  gateway: {
    port: 18789,
    auth: {
      mode: "token",
      token: "secret",
    },
  },

  agents: {
    defaults: {
      workspace: "~/.memdoor/workspace",
      model: "anthropic/claude-sonnet-4-5",
    },
  },

  channels: {
    whatsapp: {
      accounts: {
        personal: {
          authDir: "~/.memdoor/channels/whatsapp",
        },
      },
    },
  },
}
```

### Migration Strategy

**Phase 1: Config File Support (Week 22)**
- Implement JSON5 config parser
- Environment variables still work (fallback)
- Config file overrides env vars

**Phase 2: CLI Commands (Week 22)**
- `memdoor config get <path>`
- `memdoor config set <path> <value>`
- `memdoor config unset <path>`
- `memdoor setup` creates default config

**Phase 3: Documentation (Week 22)**
- Migration guide from env vars
- Multi-agent examples
- Config reference

**Phase 4: Deprecation (Week 30+)**
- Warn if using env vars without config
- Recommend migration to config file

---

## Config File Includes (`$include`)

Split large configs across multiple files:

```json5
// ~/.memdoor/memdoor.json
{
  gateway: { port: 18789 },

  // Include agent definitions from separate file
  agents: { $include: "./agents.json5" },

  // Merge multiple client configs
  bindings: {
    $include: [
      "./clients/alice.json5",
      "./clients/bob.json5",
    ],
  },
}
```

```json5
// ~/.memdoor/agents.json5
{
  defaults: {
    workspace: "~/.memdoor/workspace",
    model: "anthropic/claude-sonnet-4-5",
  },
  list: [
    { id: "main", workspace: "~/.memdoor/workspace" },
  ],
}
```

---

## Implementation Components

### 1. Config Parser (`gateway/config/parser.go`)

```go
package config

import (
    "encoding/json"
    "os"
)

// Config represents the complete Memdoor configuration
type Config struct {
    Env      *EnvConfig      `json:"env,omitempty"`
    Gateway  *GatewayConfig  `json:"gateway,omitempty"`
    Logging  *LoggingConfig  `json:"logging,omitempty"`
    Agents   *AgentsConfig   `json:"agents,omitempty"`
    Bindings []Binding       `json:"bindings,omitempty"`
    Tools    *ToolsConfig    `json:"tools,omitempty"`
    Channels *ChannelsConfig `json:"channels,omitempty"`
    Session  *SessionConfig  `json:"session,omitempty"`
}

// LoadConfig loads configuration from file or environment
func LoadConfig(configPath string) (*Config, error) {
    // 1. Try loading from config file
    if _, err := os.Stat(configPath); err == nil {
        return loadFromFile(configPath)
    }

    // 2. Fallback to environment variables
    return loadFromEnv()
}

// loadFromFile parses JSON5 config file
func loadFromFile(path string) (*Config, error) {
    // Parse JSON5 (allows comments, trailing commas)
    // Validate against schema
    // Process $include directives
    // Return validated config
}

// loadFromEnv creates config from environment variables (current behavior)
func loadFromEnv() (*Config, error) {
    // Read env vars
    // Build config structure
    // Return config
}
```

### 2. Config Validation (`gateway/config/validator.go`)

```go
package config

import "fmt"

// ValidateConfig performs schema validation
func ValidateConfig(cfg *Config) error {
    // Check required fields
    // Validate agent IDs are unique
    // Validate bindings reference existing agents
    // Validate channel configurations
    // Validate tool policies
    // Return validation errors
}
```

### 3. Config CLI (`cmd/cli/commands/config.go`)

```go
package commands

// ConfigGetCmd - get config value by path
func ConfigGetCmd() *cobra.Command {
    // openclaw config get agents.defaults.workspace
}

// ConfigSetCmd - set config value by path
func ConfigSetCmd() *cobra.Command {
    // openclaw config set agents.defaults.model "claude-opus-4-6"
}

// ConfigUnsetCmd - delete config value
func ConfigUnsetCmd() *cobra.Command {
    // openclaw config unset channels.telegram
}
```

---

## Binding Resolution Logic

**Routing Priority** (most specific wins):

1. **Peer match** - Exact DM/group ID
2. **Account match** - Specific channel account
3. **Channel match** - Any account on channel
4. **Default agent** - Fallback

**Example Resolution**:

```json5
{
  bindings: [
    // Priority 1: Specific WhatsApp group → family agent
    {
      agentId: "family",
      match: {
        channel: "whatsapp",
        peer: { kind: "group", id: "120363@g.us" },
      },
    },

    // Priority 2: WhatsApp business account → work agent
    {
      agentId: "work",
      match: { channel: "whatsapp", accountId: "business" },
    },

    // Priority 3: Any WhatsApp → home agent
    {
      agentId: "home",
      match: { channel: "whatsapp" },
    },

    // Priority 4: Default agent (home)
  ],
}
```

---

## Agent Directory Structure

Each agent has isolated state directory:

```
~/.memdoor/
├── memdoor.json               # Main config
├── agents/
│   ├── main/
│   │   ├── agent/               # Agent-specific state
│   │   │   ├── auth-profiles.json
│   │   │   ├── model-registry.json
│   │   │   └── agent-config.json
│   │   ├── sessions/            # Session store
│   │   │   └── sessions.jsonl
│   │   └── memory/              # Memory files
│   │       └── 2026-02-20.md
│   ├── work/
│   │   ├── agent/
│   │   ├── sessions/
│   │   └── memory/
│   └── family/
│       ├── agent/
│       ├── sessions/
│       └── memory/
├── workspace/                   # Main agent workspace
├── workspace-work/              # Work agent workspace
├── workspace-family/            # Family agent workspace
└── channels/                    # Channel session storage
    └── whatsapp/
        ├── personal.db
        └── business.db
```

---

## Success Criteria

### Week 22 Completion Checklist

- [ ] JSON5 config parser implementation
- [ ] Schema validation system
- [ ] Config file loading with env var fallback
- [ ] `$include` directive support
- [ ] Multi-agent routing logic
- [ ] Binding resolution (peer → account → channel → default)
- [ ] CLI commands (`config get`, `config set`, `config unset`)
- [ ] `memdoor setup` command creates default config
- [ ] Migration guide documentation
- [ ] Multi-agent examples documentation
- [ ] Unit tests for config parser
- [ ] Integration tests for multi-agent routing

---

## References

**OpenClaw Source Files**:
- `src/config/config.ts` - Config structure
- `src/config/load-config.ts` - Config loading
- `src/config/schema.ts` - Config schema
- `src/config/includes.ts` - Include processing
- `src/agents/routing.ts` - Agent routing logic

**OpenClaw Documentation**:
- the openclaw repo: `docs/concepts/multi-agent.md`
- the openclaw repo: `docs/gateway/configuration.md`
- the openclaw repo: `docs/gateway/configuration-examples.md`

**Related Memdoor Docs**:
- `docs/MULTI_AGENT_ARCHITECTURE.md` - Multi-agent design
- `docs/OPENCLAW_PARITY_ANALYSIS.md` - Feature parity tracker

---

## Open Questions

1. **Default model**: Should we default to `claude-sonnet-4-5` or require explicit config?
   - **Recommendation**: Default to Sonnet 4.5 for backward compatibility

2. **Env var precedence**: Should config file override env vars or vice versa?
   - **OpenClaw**: Config file overrides env vars
   - **Recommendation**: Same (config file wins)

3. **Hot reload**: Should config changes trigger gateway restart?
   - **OpenClaw**: Yes, with `config.apply` RPC
   - **Recommendation**: Implement in Week 23

4. **Schema strictness**: Reject unknown keys or allow them?
   - **OpenClaw**: Strict - rejects unknown keys
   - **Recommendation**: Same (fail-fast on misconfiguration)

---

## Next Steps

1. Review this design document
2. Implement config parser (Week 22)
3. Add multi-agent routing logic
4. Create CLI commands
5. Write migration guide
6. Test with multi-agent examples

**Estimated Effort**: 1 week (Week 22)
**Complexity**: Medium (requires routing logic + parser)
**Value**: High (unlocks multi-agent, A2A communication, subagents)
