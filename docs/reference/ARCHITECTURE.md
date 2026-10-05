# Memdoor Architecture

**Version**: 2.0

## Overview

Memdoor is a terminal coding agent (`memdoor tui`) with a decision model (Jev) in front of the chat model to reduce what it reads, written in Go. The client talks to a local gateway process; the gateway runs inference against a provider on the person's own key — OpenRouter (`gateway/providers/byok.go`), a company AI gateway or a vendor's API (`gateway/providers/vendor.go`, `registry.go`: Anthropic, OpenAI, Gemini, Groq, xAI, DeepSeek, Baseten), or any provider added with `memdoor connect` — every request goes through a provider (`gateway/providers/factory.go`). Underneath sits the agent runtime (tools, sessions, channels, the person's MCP servers). No Docker; the web UI is login/admin only.

This document describes core architectural concepts and design patterns.

---

## System Architecture

```
   ┌──────────────────┐   ┌──────────────────┐
   │  CLI / TUI       │   │  Web UI          │
   │  (memdoor tui)   │   │  (login/admin)   │
   └────────┬─────────┘   └────────┬─────────┘
            │ HTTP/WS               │ HTTP/WS
   ┌────────▼───────────────────────▼──────────┐
   │        Gateway Server (Go, :18789)        │
   │  • HTTP REST + WebSocket (/ws)            │
   │  • Agent runtime: tools, sessions, A2A    │
   │  • Event broadcasting                     │
   └──────┬──────────────────┬─────────────────┘
          │                  │ gateway/providers
   ┌──────▼──────┐   ┌───────▼───────────────────────────────┐
   │ Agent state │   │ LLM client (pkg/llm)                  │
   │ sessions,   │   │  └─ the providers registry: your key  │
   │ memory,     │   │     (OpenRouter, Anthropic, OpenAI, … │
   │ skills      │   │     or a company AI gateway)          │
   └──────┬──────┘   └───────────────────┬───────────────────┘
          │ ~/.memdoor/workspaces/       │ HTTPS
   ┌──────▼──────────────────┐   ┌───────▼───────────────────┐
   │ SQLite + Filesystem     │   │ The provider's API        │
   │ messages, users, memory │   └───────────────────────────┘
   │ sessions                │
   └─────────────────────────┘

   Separate process on memdoor.ai: memdoor-billing (gateway/billingsvc) —
   sign-in, the plan and the Pro seat (Stripe). It serves no model: every
   gateway runs on its own provider key.
```

---

## Core Components

### 1. Gateway Server

The central server process managing all communication.

**Purpose**: Single entry point for all clients

**Key Features**:
- HTTP REST API for CRUD operations
- WebSocket server for real-time messaging
- RPC endpoints for CLI communication
- Rate limiting and connection pooling
- Event broadcasting to subscribed clients

### 2. Agent Runtime

AI agent execution engine.

**Purpose**: Execute LLM calls and manage tool execution

**Key Features**:
- One `LLMClient` interface, resolved by `gateway/providers`: the provider registry (OpenRouter, the company gateway, Anthropic, OpenAI, Gemini, Groq, xAI, DeepSeek, Baseten — each read from its own metadata API)
- Tool execution (bash, file ops, browser, etc.)
- Concurrent agent execution with rate limiting
- Context window management
- Automatic conversation compaction

### 3. Message Service

Channel and direct message management.

**Purpose**: Route messages and manage conversations

**Key Features**:
- Channels (public, private)
- Direct messages (1-on-1, group)
- Threading (Slack-like nested replies)
- @Mentions for agent triggering
- Agent-to-Agent (A2A) communication
- Full-text search (SQLite FTS5)

### 4. Session Manager

Conversation context and history.

**Purpose**: Maintain conversation state across interactions

**Key Features**:
- Per-channel session isolation
- Message history persistence
- Token counting and context limits
- Automatic compaction when approaching limits
- Multi-agent session coordination

### 5. Billing (`gateway/billingsvc`)

**`memdoor-billing`**: a separate process on memdoor.ai built from the same binary. It owns sign-in, the plan (token → workspace → plan) and the Pro seat (Stripe), and answers the remote-control relay's plan check. It serves no model: every model call runs on the person's own provider key, from their own gateway (section 4, `docs/features/PROVIDERS.md`).

### 6. Database Layer

Persistent storage for all data.

**Purpose**: Single source of truth for messages, users, sessions, and agent memory

**Key Features**:
- SQLite for self-hosted deployments
- Migrations for schema versioning
- Full-text search (FTS5)

### 7. API models only

Memdoor runs on API models only (2026-10-03): every request goes through a
provider on the person's own key ([`../features/PROVIDERS.md`](../features/PROVIDERS.md)).

---

## Storage Layout

```
~/.memdoor/
├── config.json                            # Configuration
├── data/
│   ├── memdoor.db                         # Main database (SQLite)
│   └── logs.db                            # Structured logs (SQLite)
├── workspace/                             # Agent working directory (bootstrap files)
└── workspaces/
    └── {workspace_id}/                    # Per-workspace data

```

---

## Key Design Patterns

### 1. Domain-Driven Design (DDD)

**Concept**: Separate domain logic from infrastructure

**Structure**:
- `pkg/` - Domain layer (entities, value objects, factories)
- `gateway/` - Infrastructure layer (HTTP, WebSocket, database)
- `pkg/` NEVER imports `gateway/` (`grep -rl '"memdoor/gateway/' pkg` is empty)
- Gateway composes domain services, repositories and value objects; business rules stay in `pkg/`
- Duck typing covers the one circular case — a domain package calling back into the gateway declares the interface it needs in the consumer

**Benefits**:
- Clean separation of concerns
- Testable business logic
- Swappable infrastructure

### 2. Factory Pattern

**Concept**: Constructors that validate and fail fast

**Principles**:
- Factories return interfaces, not concrete types
- All required fields must be provided
- Invalid state cannot be constructed
- No partial initialization

**Benefits**:
- Type safety at compile time
- No runtime nil pointer errors
- Clear contracts

### 3. Duck Typing for Circular Dependencies

**Concept**: Define interfaces where they're used, not where they're implemented

**Pattern**:
- Gateway defines `SystemAnnouncementPoster` interface
- Message service implements interface
- No direct import between layers

**Benefits**:
- Breaks circular dependencies
- Maintains clean architecture
- Enables testing with mocks

### 4. Event Streaming

**Concept**: Real-time event broadcasting via WebSocket

**Event Types**:
- `lifecycle` - Agent execution start/complete/error
- `assistant` - LLM thinking and text responses
- `tool` - Tool execution start/complete/result
- `context` - Token usage and context updates
- `error` - Error events

**Benefits**:
- Live progress updates
- Multiple concurrent clients
- Transparent tool execution

### 5. Per-Channel Session Isolation

**Concept**: Each channel has independent conversation history

**Isolation Levels**:
- Separate session per channel
- Agents see only channel messages
- Context preserved across interactions
- No data leakage between channels

**Benefits**:
- Privacy and security
- Clear conversation boundaries
- Predictable agent behavior

---

## Concurrency Model

### Queue System

**Concept**: Lane-based job queuing with per-session serialization

**Lanes**:
- **Main** - Interactive user messages
- **Cron** - Scheduled tasks
- **Subagent** - Background agent spawns

**Serialization**: Jobs in same lane + same channel run sequentially
**Concurrency**: Jobs in different lanes run in parallel

### Rate Limiting

**Concept**: Protect against DoS and API overuse

**Limits**:
- 10 agent requests per minute (default)
- Configurable per agent
- Token bucket algorithm

### Connection Pooling

**Concept**: Reuse database and HTTP connections

**Pools**:
- Database connection pool (max 100)
- HTTP client pool for remote vLLM endpoints
- WebSocket connection tracking

---

## Message Flow

### Inbound Message (User → Agent)

```
User sends message
  ↓
HTTP POST /api/messages
  ↓
Parse @mentions
  ↓
Save to database
  ↓
Broadcast via WebSocket
  ↓
Trigger mentioned agents
  ↓
Agent processes message
  ↓
Agent response saved
  ↓
Broadcast agent response
```

### Outbound Message (Agent → User)

```
Agent generates response
  ↓
Create message entity
  ↓
Save to database
  ↓
Get channel members
  ↓
Get active WebSocket connections
  ↓
Broadcast to each connection
  ↓
Apply compression if >8KB
  ↓
Send via WebSocket
```

---

## Security

### Authentication

**Concept**: JWT token-based authentication

**Flow**:
- User registers/logs in via HTTP POST
- Server returns JWT token
- Client includes token in Authorization header
- Gateway validates token on each request

### Authorization

**Concept**: Channel-based access control (ACL)

**Rules**:
- Public channels: Anyone can read/write
- Private channels: Members only
- Direct messages: Participants only
- System channels: Read-only for users

### Rate Limiting

**Concept**: Prevent abuse and API quota exhaustion

**Strategy**:
- Per-agent rate limits
- Global concurrency limits
- Connection limits (max 1000 WebSocket connections)

---

## Performance Optimizations

### Context Window Management

**Problem**: every model has its own window, and a coding conversation fills
it with tool output.

**Solution** (`gateway/agent_runtime_fit.go`, `gateway/compaction/`):
- The window is the answering model's own (the catalogue's `context_length`),
  the system prompt as sent included in every count.
- A conversation compacts at 60% of it, never past 200K tokens, at the
  start of a turn. Settings under `agents.defaults.compaction`: `enabled`
  (unset is on; off still fits a turn over the window), `compactionPercent`,
  `thresholdTokens` (a fixed trigger, taking precedence), `keepRecentTokens`
  (what `/compact` keeps word for word, 12K). `/context` shows the effective
  values.
- One ladder, cheapest first, no model call: drop spent calls with their
  results (failures and empty searches of earlier turns), stub output a
  later identical call superseded (a file read again); stub older tool
  output — each stub names its call, and the `recall` tool returns the
  original from the transcript instead of running it again; summarize older messages (what was asked and answered, and
  the receipts of the work), keeping the recent ones by tokens; aim at 70%
  of the trigger so several turns fit before the next.
- The reshaped conversation is saved in the transcript as a block the next
  turn loads, so the prompt prefix stays cached; the history above it stays.
- Notes are saved once per conversation before the first summary.
- `/context` shows what fills the window. `/compact [focus]` compacts now,
  the older part summarized by the answering model (one call, the focus
  leading it; the digest stands in if the model cannot be asked); a written
  summary is carried word for word through later compactions. `/handoff
  [request]` has the model write goal, decisions, progress and next steps,
  wipes the conversation and starts the next from that.

### WebSocket Compression

**Problem**: Large messages consume bandwidth

**Solution**: RFC 7692 per-message deflate
- Messages >8KB automatically compressed
- ~60-70% size reduction for tool results and logs

### Database Indexing

**Problem**: Slow queries on large tables

**Solution**: Strategic indexes
- Compound indexes on (channel_id, created_at)
- Full-text search indexes (FTS5)
- Covering indexes for common queries

---

## Agent-to-Agent (A2A) Communication

### Concept

Agents can mention other agents using @mentions, triggering collaborative workflows.

### Patterns

**Fire-and-Forget**:
- Agent mentions another agent
- Second agent processes independently
- No response expected

**Ping-Pong**:
- Agent mentions another agent
- Second agent responds
- First agent receives response
- Back-and-forth continues

**Anti-Loop Protection**:
- Track mention depth (max 3 levels)
- Prevent infinite mention chains
- Log warnings when approaching limit

---

## Threading Model

### Concept

Slack-like threaded replies for organizing conversations.

**Structure**:
- Top-level message (`parent_message_id = NULL`)
- Thread reply (`parent_message_id = <parent_id>`)
- Nested threads NOT supported (flat threading only)

**Benefits**:
- Organize multi-agent discussions
- Keep main channel clean
- Preserve conversation context

---

## Workspaces and Multi-Tenancy

### Concept

Each workspace represents a separate knowledge domain (e.g., a project, a team). Settings (`tool_guards` and the rest) are workspace-scoped. The plan (free / pro) is owned by the billing service — token → workspace → plan; the gateway's `workspaces.plan` column is vestigial.

**Current Isolation**:
- Workspace settings and channels scoped to individual workspaces
- Usage counting per workspace per tool

**Future Isolation**:
- Workspace ID on all database tables
- Row-Level Security (RLS) in PostgreSQL
- Resource quotas and billing per workspace

---

## Error Handling

### Tool Execution Errors

**Strategy**: Return error as tool result, continue conversation

**Rationale**: LLM can see error and adapt strategy

### API Errors

**Strategy**: Log error, emit event, return to client

**Rationale**: User sees error immediately, can retry or adjust

### Storage Errors

**Strategy**: Fall back to stderr if database logging fails

**Rationale**: Never lose logs due to database issues

---

## Testing

### Test Coverage

**Current**: 35.9% (195+ tests)

**Major Suites**:
- Agent-to-Agent (A2A): 71 tests
- Event streaming: 27 tests
- WebSocket RPC: 11 tests
- Broadcasting: 12 tests
- Queue system: 12 tests

### Testing Philosophy

**Principles**:
- Test via CLI, not direct DB access
- Integration tests over unit tests
- Real LLM calls for critical paths
- Mock LLM for fast tests

---

## Future Enhancements

### Planned Features

1. **Additional connectors** - PDF ingestion, RSS feeds (browser extension covers most daily inputs already)
2. **PostgreSQL backend** - Production-ready cloud database
3. **Redis pub/sub** - Multi-gateway broadcasting
4. **Multi-workspace hosting** - Full multi-tenant support with usage billing

### Nice to Have (COULD)

- **gRPC API** - Efficient binary protocol

### Architecture Principles

- **Interface-driven design** - All major components use interfaces
- **Swappable backends** - Storage, broadcast, search, etc.
- **Backward compatibility** - Migrations, not breaking changes
- **Performance first** - Optimize for throughput and latency

---

## References

- **[CLI Reference](CLI.md)** - Complete command reference
- **[Managed Lane ADR](../adr/0001-managed-lane-architecture.md)** - Why the marketplace is built this way
- **[Gateway Architecture](gateway/README.md)** - Gateway server deep dive
- **[Authorization](AUTHORIZATION.md)** - Security and permissions
- **[Logs](LOGS.md)** - Structured logging and debugging
- **[Client API](../developers/CLIENT_API.md)** - REST & WebSocket API

---

