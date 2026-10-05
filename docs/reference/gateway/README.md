# Gateway Architecture

**Complete reference for the Memdoor Gateway service**

> ⚠ Examples below with `TASKFORCE_API_KEY` env vars or agent rows with
> `"model": "claude-sonnet-4-5"` are historical. The gateway runs on API
> models only (2026-10-03): a provider on the person's key, a company's AI
> gateway, or the broker for a seat. The internal LLM types stay
> Anthropic-shaped; every provider converts to that shape.

---

## Table of Contents

- [What is the Gateway?](#what-is-the-gateway)
- [Architecture Overview](#architecture-overview)
- [Running the Gateway](#running-the-gateway)
- [Configuration](#configuration)
- [WebSocket Protocol](#websocket-protocol)
- [Agent Execution](#agent-execution)
- [Session Management](#session-management)
- [Message Flow](#message-flow)
- [SPA Routes](#spa-routes)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)

---

## What is the Gateway?

The Gateway is Memdoor's **central server process** that:

- **Manages agent execution** - Spawns and monitors AI agent jobs
- **Handles WebSocket connections** - Real-time messaging between clients
- **Serves HTTP APIs** - REST endpoints for CRUD operations
- **Coordinates sessions** - Multi-agent conversation management
- **Processes messages** - Routing, threading, mentions, A2A
- **Runs cron jobs** - Scheduled agent tasks
- **Manages authentication** - JWT token validation
- **Stores data** - SQLite/PostgreSQL persistence

**Single Entry Point**: All Memdoor clients (Web UI, CLI, TUI, external agents) communicate through the Gateway.

---

## Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Memdoor Gateway                          │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │  HTTP Server │  │  WebSocket   │  │  RPC Server  │    │
│  │  (REST API)  │  │    Server    │  │    (CLI)     │    │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘    │
│         │                  │                  │             │
│         └──────────────────┴──────────────────┘             │
│                            │                                │
│                  ┌─────────▼──────────┐                    │
│                  │  Request Handler   │                    │
│                  └─────────┬──────────┘                    │
│                            │                                │
│         ┌──────────────────┼──────────────────┐            │
│         │                  │                  │            │
│    ┌────▼────┐      ┌──────▼──────┐   ┌──────▼──────┐   │
│    │ Message │      │   Agent     │   │   Session   │   │
│    │ Service │      │  Executor   │   │   Manager   │   │
│    └────┬────┘      └──────┬──────┘   └──────┬──────┘   │
│         │                  │                  │            │
│         └──────────────────┼──────────────────┘            │
│                            │                                │
│                  ┌─────────▼──────────┐                    │
│                  │  Database Layer    │                    │
│                  │  (SQLite/Postgres) │                    │
│                  └────────────────────┘                    │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

#### 1. HTTP Server (Port 18789)
- **REST API** - CRUD for channels, messages, users
- **Authentication** - JWT token validation
- **File uploads** - Avatar uploads, attachments
- **Health checks** - `/health` endpoint
- **Static files** - Serves Web UI (React app)

#### 2. WebSocket Server (Same Port)
- **Real-time messaging** - Bidirectional communication
- **Presence** - Online/offline/typing indicators
- **Message broadcasting** - Channel and DM delivery
- **Compression** - Automatic for large messages (>8KB)

#### 3. RPC Server (Internal)
- **CLI communication** - Direct binary protocol
- **Tool execution** - Agent tool calls
- **Admin operations** - User management, config

#### 4. Agent Executor
- **Job queue** - Manages agent execution queue
- **LLM client** - one `LLMClient` interface resolved by `gateway/providers`: the provider registry (OpenRouter, a company gateway, Anthropic, OpenAI, Gemini, Groq, xAI, DeepSeek, Baseten — `registry.go`), or the broker for a seat
- **Tool execution** - bash, read_file, write_file, etc.
- **Concurrency** - Parallel agent execution with rate limiting

#### 5. Message Service
- **Routing** - Channels, DMs, threads
- **@Mentions** - Agent triggering and A2A
- **Threading** - Slack-like threaded replies
- **Persistence** - SQLite/PostgreSQL storage
- **Search** - Full-text search (FTS5)

#### 6. Session Manager
- **Conversation context** - Message history per session
- **Token counting** - Context window management
- **Compaction** - Automatic history compression
- **Multi-agent** - Concurrent agent sessions

---

## Running the Gateway

### Local Development

```bash
# Start gateway (default: http://localhost:18789)
./memdoor gateway

# Verbose logging
./memdoor gateway --verbose

# Custom port
./memdoor gateway --port 8080

# Custom host (listen on all interfaces)
./memdoor gateway --host 0.0.0.0 --port 18789
```

### Production Deployment

```bash
# Build binary
make build

# Run as systemd service (Linux)
sudo systemctl start memdoor-gateway

# Run as launchd service (macOS)
launchctl load ~/Library/LaunchAgents/com.memdoor.gateway.plist
```

The gateway is a single static binary; there is no container image.

### Process Management

**Start Gateway:**
```bash
make start
# Or: make gateway-verbose
```

**Stop Gateway:**
```bash
make stop
```

**Restart Gateway:**
```bash
make clean stop start
```

**Check Status:**
```bash
curl http://localhost:18789/health
```

---

## Configuration

The Gateway is configured via `~/.memdoor/config.json` (JSON5 format).

### Minimal Configuration

```json5
{
  "agents": {
    "defaults": {
      "workspace": "~/.memdoor/workspace",
      "model": "claude-sonnet-4-5",
      "maxTokens": 4096
    },
    "list": [
      { "id": "writer", "default": true }
    ]
  }
}
```

### Full Configuration Example

```json5
{
  // Gateway settings
  "gateway": {
    "port": 18789,
    "host": "localhost",
    "cors": {
      "enabled": true,
      "origins": ["http://localhost:5173"]
    },
    "rateLimit": {
      "enabled": true,
      "requestsPerMinute": 60
    }
  },

  // Agent configuration
  "agents": {
    "defaults": {
      "workspace": "~/.memdoor/workspace",
      "model": "claude-sonnet-4-5",
      "maxTokens": 4096,
      "temperature": 0.7
    },
    "list": [
      {
        "id": "writer",
        "name": "Writer Agent",
        "default": true,
        "emoji": "✍️"
      },
      {
        "id": "coder",
        "name": "Coder Agent",
        "model": "claude-sonnet-4-5",
        "workspace": "~/projects"
      },
      {
        "id": "remote-agent",
        "remote": {
          "enabled": true,
          "endpoint": "https://my-agent.com/v1/chat/completions",
          "apiKey": "${MY_AGENT_API_KEY}",
          "model": "my-model-1.0"
        }
      }
    ]
  },

  // Cron jobs
  "cron": {
    "enabled": true,
    "store": "~/.memdoor/cron/jobs.json",
    "jobs": [
      {
        "id": "daily-report",
        "schedule": "0 0 9 * * *",
        "message": "Generate daily report",
        "agent": "writer",
        "enabled": true
      }
    ]
  },

  // Database
  "database": {
    "type": "sqlite",
    "path": "~/.memdoor/data/memdoor.db"
  },

  // Logging
  "logging": {
    "level": "info",
    "database": "~/.memdoor/data/logs.db",
    "console": true
  }
}
```

### Environment Variables

```bash
# Inference runs on a provider's key: OPEN_ROUTER_API_KEY, or a vendor's /
# company gateway's through `memdoor connect` (docs/features/PROVIDERS.md).
# MEMDOOR_BILLING_TOKEN is the seat's token, only for a seat.

# Configuration
export MEMDOOR_CONFIG=~/.memdoor/config.json
export MEMDOOR_PROFILE=production

# Gateway
export MEMDOOR_GATEWAY_PORT=18789
export MEMDOOR_GATEWAY_HOST=localhost
```

---

## WebSocket Protocol

### Connection

```javascript
const ws = new WebSocket('ws://localhost:18789/ws');

ws.onopen = () => {
  // Send authentication
  ws.send(JSON.stringify({
    type: 'auth',
    token: 'eyJhbGc...'
  }));
};
```

### Subscribe to Channel

```javascript
ws.send(JSON.stringify({
  type: 'subscribe',
  channel_id: 'channel:uuid'
}));
```

### Receive Messages

```javascript
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  switch (data.type) {
    case 'message':
      console.log('New message:', data.message);
      break;
    case 'typing':
      console.log('User typing:', data.user_id);
      break;
    case 'presence':
      console.log('User status:', data.status);
      break;
  }
};
```

### Message Types

**message** - New message in channel
```json
{
  "type": "message",
  "message": {
    "id": "message:uuid",
    "channel_id": "channel:uuid",
    "sender_id": "user:uuid",
    "content": {
      "type": "text",
      "text": "Hello!"
    },
    "created_at": "2026-03-18T12:00:00Z"
  }
}
```

**typing** - User is typing
```json
{
  "type": "typing",
  "channel_id": "channel:uuid",
  "user_id": "user:uuid",
  "typing": true
}
```

**presence** - User status change
```json
{
  "type": "presence",
  "user_id": "user:uuid",
  "status": "online"
}
```

---

## Agent Execution

### Agent Lifecycle

```
1. Message received with @mention
   ↓
2. Agent executor detects mention
   ↓
3. Create job in queue
   ↓
4. Load conversation history (session)
   ↓
5. Call the engine (the provider on the person's key, or the broker for a seat)
   ↓
6. Process tool calls (if any)
   ↓
7. Execute tools (bash, read_file, etc.)
   ↓
8. Return tool results to LLM
   ↓
9. Get final response
   ↓
10. Post response to channel
   ↓
11. Update session history
   ↓
12. Broadcast via WebSocket
```

### Concurrency & Rate Limiting

**Default Limits:**
- 10 requests per minute per agent
- 5 concurrent agent executions
- 30 second timeout per agent call

**Configuration:**
```json5
{
  "agents": {
    "rateLimit": {
      "requestsPerMinute": 10,
      "maxConcurrent": 5,
      "timeout": 30000
    }
  }
}
```

---

## Session Management

### Session Types

**Per-Channel Sessions** - Default
- Each channel has its own conversation history
- Agents see all messages in the channel
- Context preserved across multiple interactions

**Per-Agent Sessions** - Optional
- Each agent has separate context
- Useful for specialized agents
- Configure via `session.scope: "per-agent"`

**Per-User Sessions** - Optional
- Each user gets isolated context
- Useful for personalized agents
- Configure via `session.scope: "per-user"`

### Context Window Management

**Automatic compaction** — see `docs/reference/ARCHITECTURE.md`, Context
Window Management. At the start of a turn a conversation past 60% of the
answering model's window (never past 200K tokens) is fitted back, cheapest
step first and without a model call; the result is saved so the next turn
sends the same prefix.

**Configuration:**
```json5
{
  "agents": {
    "defaults": {
      "compaction": {
        "compactionPercent": 60,          // share of the window that triggers it
        "memoryFlush": { "enabled": true } // notes once per conversation, before the first summary
      }
    }
  }
}
```

---

## Message Flow

### Inbound Message Flow

```
User sends message
  ↓
HTTP POST /api/messages
  ↓
Authenticate request (JWT)
  ↓
Parse message content
  ↓
Detect @mentions
  ↓
Save to database
  ↓
Broadcast via WebSocket
  ↓
Trigger mentioned agents
  ↓
Agent executor processes
  ↓
Agent response posted
  ↓
Broadcast agent response
```

### Outbound Message Flow

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
Apply compression if needed (>8KB)
  ↓
Send via WebSocket
  ↓
Client receives and displays
```

---

## SPA Routes

The gateway serves the React SPA for the Web UI. Routes that should render the SPA (rather than return a 404) are maintained in an allowlist in `gateway/server_lifecycle.go`.

**Notable entries:**
- `/pricing` was added to the SPA route allowlist so the client-side router handles it.

When adding new client-side routes, remember to add them to the SPA allowlist in `server_lifecycle.go` so the gateway serves `index.html` instead of a 404.

---

## Troubleshooting

### Common Issues

#### 1. Gateway Won't Start

**Symptom**: `Error: address already in use`

**Solution**:
```bash
# Check what's using port 18789
lsof -i :18789

# Kill the process
kill -9 <PID>

# Or use make stop
make stop
```

#### 2. WebSocket Connection Fails

**Symptom**: `WebSocket connection failed`

**Check**:
```bash
# Verify gateway is running
curl http://localhost:18789/health

# Check CORS configuration
# Add your Web UI origin to config.json
{
  "gateway": {
    "cors": {
      "origins": ["http://localhost:5173"]
    }
  }
}
```

#### 3. Agents Not Responding

**Check Logs:**
```bash
./memdoor logs query --regex "agent.*error" --limit 20
./memdoor logs errors --since 5m
```

**Common Causes:**
- Invalid API key (TASKFORCE_API_KEY not set)
- Agent not configured in config.json
- Rate limit exceeded
- Network timeout

#### 4. High Memory Usage

**Monitor:**
```bash
# Check session count
./memdoor sessions list

# Check agent execution
ps aux | grep memdoor
```

**Solutions:**
- `/context` in the TUI shows what fills the window; `/compact [focus]` compacts now
- Lower `agents.defaults.compaction.compactionPercent` to compact earlier
- Restart gateway: `make clean stop start`

#### 5. Database Locked

**Symptom**: `database is locked`

**Solution**:
```bash
# Stop all processes
make stop

# Check for orphaned processes
ps aux | grep memdoor | grep -v grep

# Restart
make start
```

### Debug Logging

**Enable verbose logging:**
```bash
./memdoor gateway --verbose
```

**Check specific components:**
```bash
# Agent execution
./memdoor logs query --regex "agent" --limit 50

# WebSocket
./memdoor logs query --regex "websocket|ws" --limit 50

# HTTP requests
./memdoor logs query --regex "http|api" --limit 50

# Database
./memdoor logs query --regex "database|sql" --limit 50
```

### Performance Monitoring

**Check system metrics:**
```bash
# CPU and memory
top -p $(pgrep memdoor)

# Open connections
netstat -an | grep 18789

# Database size
du -h ~/.memdoor/data/
```

---

## See Also

- **[CLI Reference](../CLI.md)** - Command-line interface
- **[Architecture](../ARCHITECTURE.md)** - System design
- **[Logs Reference](../LOGS.md)** - Logging and debugging
- **[Authorization](../AUTHORIZATION.md)** - Security and permissions

---

