# Memdoor Features

Feature reference for Memdoor — a terminal coding agent with a decision
model in front of the chat model to cut the bill: judged reads, a model
ladder per agent, stops instead of caps, `/model` with real prices
([features/DECIDE.md](features/DECIDE.md)). Chat goes through a provider you
connected — your own key (OpenRouter, a vendor, your company's gateway) —
never a seat's broker. API models only.
Your key and the models are in the served docs
([Your key and the models](../web/public/docs/your-key.md)); this page covers
the gateway and agent features.

---

## Core Features

### Multi-Agent Collaboration

Agent-to-agent (A2A) mentions for cases where one agent needs another's tool surface, such as the planner handing a plan to the coder.

- **Agent Mentions**: `@agent-name` syntax for A2A delegation
- **Depth Control**: Prevents infinite agent loops
- **Threading**: A2A replies stay threaded under the parent message

**Example**:
```bash
memdoor agent --message "@coder please run the tests and fix what fails" --channel general --agent-id chief
```

[Read more →](features/multi-agent-collaboration.md)

---

### [Stdin Piping](features/stdin-piping.md) ⭐ Killer Feature

Pipe any command output directly to agents for real-time analysis.

**Examples**:
```bash
# Production error analysis
tail -f production.log | grep ERROR | memdoor agent -m "Analyze and suggest fixes" -c general -a chief

# Real-time log monitoring
kubectl logs -f pod-name | memdoor agent -m "Summarize system activity" -c general -a chief

# Git diff review
git diff | memdoor agent -m "Review this code change for issues" -c general -a chief

# Pattern analysis
grep -r "TODO" src/ | memdoor agent -m "Prioritize these TODOs by impact" -c general -a chief
```

[Read more →](features/stdin-piping.md)

---

### [Authentication](features/authentication.md)

Sign in to memdoor.ai with an emailed code; the same command
sets up this Mac's engine session. Signing out forgets the seat only.

```bash
memdoor login you@example.com     # code by email, typed here
memdoor account status            # this Mac's engine user, and the memdoor.ai account
memdoor logout                    # sign out of memdoor.ai; the agent keeps working
```

**Features**:
- Secure token storage in `~/.memdoor/credentials.json`
- 30-day session tokens (`memdoor auth logout` or expiry is what ends one; the email-verification link lives 24 hours)
- Cross-platform browser support
- Headless mode for servers

[Read more →](features/authentication.md)

---

### Memory & RAG (removed 2026-10-03)

Two complementary systems for information storage and retrieval:

**Memory**: Per-agent BM25 keyword search
```bash
memdoor agent -m "Remember: API rotation happens on the 1st of each month" -c general -a chief
memdoor agent -m "When do we rotate API keys?" -c general -a chief
```

**RAG**: workspace-wide semantic search with vector embeddings, used by the agents' search tools (no command of its own since 2026-10-03).

| Feature | Memory | RAG |
|---------|--------|-----|
| Search | Keyword (BM25) | Semantic (embeddings) |
| Scope | Per-agent | Per-workspace |
| Use Case | Agent facts | Knowledge base |

Read more → (removed 2026-10-03)

---

### [Structured Logging](features/logging.md)

Powerful log querying and analysis with SQLite-based storage.

```bash
# Query recent logs
memdoor logs query --limit 50

# Search by pattern
memdoor logs query --regex "ERROR|WARN" --limit 20

# Recent errors only
memdoor logs errors --since 5m

# Follow logs real-time
memdoor logs tail
```

**Features**:
- Structured fields with metadata
- Full-text search (FTS5)
- Time-based filtering
- Component/level filtering
- Cron job tracking

[Read more →](features/logging.md)

---

### [Thinking Mode](features/thinking-mode.md)

Control whether agents show their reasoning process.

```bash
# Show reasoning
memdoor agent --message "Debug this issue" --thinking enabled

# Direct answers only
memdoor agent --message "Quick question" --thinking disabled

# Auto (agent decides)
memdoor agent --message "Help me" --thinking auto
```

**Output with thinking**:
```
<thinking>
1. Error at line 42
2. Variable undefined
3. Missing import
</thinking>

Add this import: import { utils } from './utils';
```

[Read more →](features/thinking-mode.md)

---

### [Cron Jobs](features/cron-jobs.md)

Schedule agents to run tasks automatically.

```yaml
# AGENTS.md
buddies:
  - id: daily-reporter
    cron_jobs:
      - name: daily-standup
        schedule: "0 9 * * 1-5"  # 9 AM weekdays
        prompt: "Generate daily standup summary"
        channel: team-updates
```

**CLI**:
```bash
memdoor cron list
memdoor cron history daily-standup --limit 20
memdoor logs query --regex 'daily-standup' --limit 20
```

[Read more →](features/cron-jobs.md)

---

### [Channels & Threading](features/channels-and-threading.md)

Slack-like channels with threading support for organized conversations.

**Channels**:
```bash
memdoor agent --message "Start feature" --channel dev --agent-id coder
memdoor messages --channel dev --limit 20
```

**Threading**:
```bash
# Reply in thread: the thread is the channel turn it was spawned from
# (parent_message_id is set by the gateway, not by a CLI flag)
memdoor messages --channel dev --include-threads
```

**Features**:
- Unlimited channels per workspace
- Threaded conversations
- Channel-based permissions
- Message search and export

[Read more →](features/channels-and-threading.md)

---

### Remote Agents

Build OpenAI-compatible remote agents with custom backends.

**Features**:
- Standard OpenAI API compatibility
- Custom tool definitions
- Secure authentication
- Webhook support

**Use Cases**:
- Integrate external AI services
- Build custom agent backends
- Connect proprietary AI models
- Create specialized domain agents

**Documentation**: [Building Agents](developers/BUILDING_AGENTS.md), [Client API](developers/CLIENT_API.md)

---

### Multiple Interfaces

Access Memdoor through your preferred interface:

**1. TUI** - The coding agent, on the directory you launch it from
```bash
cd your-project && memdoor tui
```

**2. CLI** - Command-line for scripting and automation
```bash
memdoor agent -m "Hello" -c general -a chief
```

**3. Web** - Login + admin UI at `http://localhost:18789`. The TUI + CLI are the primary surfaces; the web UI is for credentials, workspace settings, and reading. The coder uses MCP servers the person connects (`/mcp`, `memdoor mcp`; `features/MCP.md`).

All interfaces share the same gateway and database.

---

## Performance Features

### WebSocket Compression
- Automatic compression for messages > 8KB
- ~70% bandwidth reduction
- Transparent to clients

### Context Compaction
- Automatic message history compression
- Keeps conversations within context limits
- Preserves important information

### Concurrent Requests
- Non-blocking agent execution
- Parallel tool execution
- Rate limiting: 10 WebSocket messages per minute per client, the rest refused

---

## Security Features

### Sandbox Isolation
- Workspace-level file access control
- Restricted system access
- Safe tool execution

### Authentication & Authorization
- JWT-based authentication
- Workspace-based access control
- Secure credential storage
- Token expiration

### Audit Logging
- All agent actions logged
- Tool execution tracking
- User activity monitoring
- Searchable log database

---

## Agent Features

### Agent Icons
- Custom pixel art icons (base64 PNG/JPG)
- Displayed in all interfaces
- Fallback to emoji avatars
- 80s retro aesthetic support

### Agent Tools
Built-in tools available to agents:
- `memory`: Store/retrieve agent facts
- `bash`: Execute shell commands
- `read_file`: Read files from workspace
- `write_file`: Write files to workspace
- `list_files`: List directory contents
- Custom tools via configuration

---

## Coming Soon

### Multi-Tenant SaaS
- PostgreSQL backend
- Workspace isolation
- Team collaboration
- Cloud deployment

### Advanced RAG
- Multi-modal embeddings
- Document ingestion pipeline
- Code understanding
- Knowledge graphs

### Enhanced Web UI
- Real-time updates
- Rich media support
- Mobile responsive
- Dark mode

### Additional Features
- Voice interface
- Mobile apps
- IDE integrations
- Enterprise SSO

---

## Quick Links

### User Documentation
- [Getting Started](README.md)
- [CLI Reference](reference/CLI.md)
- [Agent Configuration](developers/BUILDING_AGENTS.md)
- [Configuration Guide](architecture/CONFIG_SYSTEM_DESIGN.md)

### Developer Documentation
- Contributing: `AGENTS.md` at the repo root (coding principles, testing, git workflow)
- [Architecture](reference/ARCHITECTURE.md)
- [Client API](developers/CLIENT_API.md)

### Feature Details
- [Multi-Agent Collaboration](features/multi-agent-collaboration.md)
- [Stdin Piping](features/stdin-piping.md)
- [Authentication](features/authentication.md)
- [Logging](features/logging.md)
- [Thinking Mode](features/thinking-mode.md)
- [Cron Jobs](features/cron-jobs.md)
- [Channels & Threading](features/channels-and-threading.md)

---
