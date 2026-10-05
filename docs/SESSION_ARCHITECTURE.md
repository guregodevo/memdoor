# Session Architecture

## Overview

Memdoor uses **two different session models** that serve distinct purposes: one for authentication/workspace management, and one for agent conversation history.

---

## Two Session Systems

### 1. Authentication Sessions 

**Purpose**: User authentication and workspace management

**Concept**: UUID-based sessions that link users to workspaces and manage authentication tokens.

**Key Features**:
- UUID-based identity for uniqueness
- Links users to workspaces (multi-tenancy)
- Handles session expiry and auth tokens
- Tracks session type (websocket, agent, API)
- **NOT used for conversation history**

**Storage**: Database (SQLite/PostgreSQL)

### 2. Conversation Sessions 

**Purpose**: Conversation history for agent execution

**Concept**: String-based session keys that organize agent conversations by channel and agent.

**Key Features**:
- String-based session identifiers
- Messages persisted to JSONL files
- Per-agent, per-channel isolation
- In-memory caching for performance
- Thread-safe concurrent access

**Storage**: JSONL files on disk

---

## Session Key Patterns

### Channel Agent Sessions

**Format**: `chat:{channelID}:agent:{agentName}`

**Example**: `chat:93b0b1bb-fe14-4888-91d2-120d26c89bcb:agent:researcher`

**Isolation Strategy**:
- `@researcher` in channel A → session `chat:channelA:agent:researcher`
- `@writer` in channel A → session `chat:channelA:agent:writer`
- `@researcher` in channel B → session `chat:channelB:agent:researcher`

**Key Insight**: Each agent has its own session PER CHANNEL, ensuring complete conversation isolation.

---

## Session Persistence

### In-Memory (SessionManager)

**Concept**: Fast access to active sessions

**Characteristics**:
- Stores session metadata (timestamps, session ID)
- Lightweight session registry
- **Wiped on gateway restart**
- Thread-safe with mutex locking

### On-Disk (SessionPersistence)

**Concept**: Durable storage for conversation history

**Storage Path**: `~/.memdoor/agents/{agentId}/sessions.json`

**Format**: JSONL (JSON Lines) - one message per line

**Example Structure**:
```json
{"message": {"role": "user", "content": [{"type": "text", "text": "Hello"}]}, "timestamp": 1772404089}
{"message": {"role": "assistant", "content": [{"type": "text", "text": "Hi!"}]}, "timestamp": 1772404092}
```
(Each line is a separate JSON object)

**Benefits**:
- Append-only pattern (fast writes)
- Easy to read and debug
- Human-readable format
- Incremental persistence

---

## Multi-Agent Channel Behavior

### Question: What happens when multiple agents are in the same channel?

**Answer**: Complete session isolation per agent

### Example Scenario

Channel `#engineering` has:
- `@writer` (channel scope)
- `@researcher` (channel scope)
- Human user posts: "Hello @writer and @researcher"

**Sessions Created**:

**Session 1**: `chat:#engineering:agent:writer`
- Conversation: [user: "Hello @writer...", assistant: "Writer's response"]

**Session 2**: `chat:#engineering:agent:researcher`
- Conversation: [user: "Hello @researcher...", assistant: "Researcher's response"]

**Isolation Characteristics**:
-  Each agent sees ONLY its own conversation history
-  Writer cannot see researcher's responses
-  No cross-agent visibility
-  Agents cannot collaborate or see each other's work

**Storage Separation**:
- `~/.memdoor/agents/writer/sessions.json` - writer's history
- `~/.memdoor/agents/researcher/sessions.json` - researcher's history

---

## Session Lifecycle

```
User sends message to @researcher in channel
         ↓
SessionManager looks up or creates session "chat:channelID:agent:researcher"
         ↓
         ├─ In-memory: Create/retrieve Session object
         └─ On-disk: Load messages from JSONL file
         ↓
Agent processes message (added to conversation)
         ↓
Compaction check (60% → local, 90% → AI, overflow → emergency)
         ↓
SessionPersistence appends message to JSONL file
         ↓
Response sent to channel
```

---

## Architecture Decisions

### Per-Agent, Per-Channel Sessions

**Benefits**:
- Clear isolation boundaries
- Agent-specific conversation history
- Sandbox security aligned with session scope
- No data leakage between agents

**Tradeoffs**:
- No multi-agent collaboration in channels
- Duplicate messages stored (same user message in multiple agent sessions)
- Higher storage overhead

### JSONL Persistence

**Benefits**:
- Append-only log pattern (simple and reliable)
- Easy to read/debug (human-readable)
- Incremental writes (no need to rewrite entire file)
- Crash-safe (partial writes are still valid JSONL)

**Tradeoffs**:
- No compaction at storage level (files grow unbounded)
- No indexing or fast lookups
- Need manual cleanup for old sessions

---

## Context Window Management

### Problem

LLMs have token limits (e.g., 200K tokens for Claude). Long conversations can exceed these limits.

### Solution: Three-Tier Compaction

**Tier 1: Local Compaction (60% threshold)**
- Keep last 20 messages
- Drop older messages
- Fast (<1ms)
- Free (no API cost)

**Tier 2: AI Compaction (90% threshold)**
- Summarize older messages via LLM
- Keep last 15 messages + summary
- Slower (~2-3s)
- Small cost (~$0.003 per compaction)

**Tier 3: Emergency Compaction (100% overflow)**
- Keep last 10 messages
- Emergency fallback when AI compaction fails
- Prevents context overflow errors

### Compaction Triggers

**When compaction happens**:
- After each message is added to conversation
- Before sending to LLM API
- Based on token count, not message count

**Why token-based?**:
- Messages vary in size (short text vs. large tool results)
- Tool outputs can be 10K+ tokens
- Accurate threshold calculation prevents API errors

---

## Session Metadata

Each session tracks:

**Timestamps**:
- Created at
- Last updated
- Last activity

**Conversation State**:
- Message count
- Token count (approximate)
- Last compaction time

**Agent Context**:
- Agent ID
- Channel ID
- Session type (main, group, channel)

---

## Thread Safety

**Concept**: Multiple concurrent requests can access same session

**Protection Mechanism**:
- Read-write mutex (sync.RWMutex)
- Allows multiple readers OR single writer
- Prevents race conditions

**Critical Sections**:
- Adding messages to session
- Reading conversation history
- Compacting messages
- Persisting to disk

---

## Session Expiry

### Authentication Sessions

**Expiry Policy**:
- Default: 7 days
- Configurable per session type
- Automatic cleanup of expired sessions

### Conversation Sessions (Gateway)

**Expiry Policy**:
- No automatic expiry
- Manual cleanup via CLI
- Archival after N days (future)

**Rationale**: Conversation history should persist until explicitly cleared by user.

---

## Recommendations

### Short Term

1. **Session metrics** - Track conversation length, token count per session
2. **Session clear API** - Expose session clear endpoint in Web UI
3. **Compaction logging** - Log compaction events for debugging

### Medium Term

1. **Storage-level compaction** - Compact JSONL files when they exceed threshold
2. **Session metadata tracking** - Store token count, last compaction time
3. **Session archival UI** - Allow users to archive old sessions

### Long Term

1. **Shared channel context** - Optional flag to allow agents to see each other's messages
2. **Database-backed persistence** - Migrate from JSONL to SQLite for better query performance
3. **Multi-workspace sessions** - Full multi-tenancy support with workspace isolation

---

## References

- **[Architecture](reference/ARCHITECTURE.md)** - Overall system architecture
- **[Gateway](reference/gateway/README.md)** - Gateway server details
- **[Authorization](reference/AUTHORIZATION.md)** - Security and permissions
