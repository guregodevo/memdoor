# Memory Tool - Long-Term Knowledge Storage

## Overview

The Memory Tool provides long-term knowledge storage and retrieval using BM25 keyword search. Think of it as the AI agent's personal notebook that persists across conversations and sessions.

**Pattern**: OpenClaw's memory tool with BM25-only fallback mode
**Storage**: `~/.memdoor/memory/{agentId}.sqlite` (per-agent databases)
**Search**: SQLite FTS5 with BM25 ranking (keyword-based)
**Scoring**: Higher scores = better match (e.g., 1.97 for exact match)

---

## Features

 **Phase 1 (Current)**: BM25 Keyword Search
- SQLite FTS5 full-text search with BM25 ranking
- Tag-based organization
- Metadata support (JSON)
- No external dependencies
- Works offline

The memory tool is keyword-only: there is no semantic search.

---

## Actions

### 1. Store a Memory

Save a new memory with optional tags and metadata:

```json
{
  "action": "store",
  "content": "User prefers dark mode for better readability",
  "tags": "preference,ui,theme",
  "metadata": "{\"priority\": \"high\", \"category\": \"ui\"}"
}
```

**Response**:
```
Memory stored successfully
ID: mem_1771710685125870000
Content: User prefers dark mode for better readability
Tags: preference, ui, theme
```

---

### 2. Search Memories

Find memories using BM25 keyword search:

```json
{
  "action": "search",
  "query": "dark mode preference",
  "limit": 10
}
```

**Response**:
```
Found 1 memories for query: dark mode preference

1. [Score: 1.97] mem_1771710685125870000
   Content: User prefers dark mode for better readability
   Tags: preference, ui, theme
   Created: 2026-02-21 22:51:25
```

**Scoring**: BM25 relevance scores - higher scores indicate better matches
- Exact term match: ~1.5-2.0
- Partial match: ~0.5-1.0
- Tag-only match: ~0.00 (tags indexed separately)

---

### 3. Retrieve a Memory

Get a specific memory by ID:

```json
{
  "action": "retrieve",
  "id": "mem_1771710685125870000"
}
```

**Response**:
```
Memory: mem_1771710685125870000
Content: User prefers dark mode for better readability
Tags: preference, ui, theme
Metadata: {
  "priority": "high",
  "category": "ui"
}
Created: 2026-02-21 22:51:25
Updated: 2026-02-21 22:51:25
```

---

### 4. List Memories

List all memories, optionally filtered by tags:

```json
{
  "action": "list",
  "tags": "project,milestone",
  "limit": 10
}
```

**Response**:
```
Found 2 memories with tags: project, milestone

1. mem_1771710690201266000
   Content: User working on memdoor project - AI agent orchestration system
   Tags: project, work
   Created: 2026-02-21 22:51:30

2. mem_1771710690203042000
   Content: Completed Week 28: Ping-pong conversations with A2A messaging
   Tags: project, milestone
   Created: 2026-02-21 22:51:30
```

---

### 5. Delete a Memory

Remove a memory by ID:

```json
{
  "action": "delete",
  "id": "mem_1771710685125870000"
}
```

**Response**:
```
Memory deleted successfully: mem_1771710685125870000
```

---

## CLI Usage

There is no `memdoor memory` command (it was removed 2026-10-03 with the
other thin CRUD wrappers). The tool above is the interface: it is called
inside an agent turn, and the agent's own database is the one it touches.
An agent only has it when the name `memory` (or `group:memory`) is in its
tools — no profile grants it (gateway/config/tool_policy.go).

---

## Use Cases

### 1. User Preferences

```
Store: {"action": "store", "content": "User prefers 2-space indentation", "tags": "coding-style,preference"}
Search: {"action": "search", "query": "indentation preference"}
```

### 2. Project Context

```
Store: {"action": "store", "content": "Working on memdoor - multi-agent AI orchestration", "tags": "project,current"}
Search: {"action": "search", "query": "current project"}
```

### 3. Code Patterns

```
Store: {"action": "store", "content": "Always use CGO_CFLAGS for SQLite FTS5", "tags": "build,sqlite,reminder"}
Search: {"action": "search", "query": "sqlite build flags"}
```

### 4. Meeting Notes

```
Store: {"action": "store", "content": "Discussed memory tool architecture in Week 29", "tags": "meeting,milestone", "metadata": "{\"date\": \"2026-02-21\"}"}
List: {"action": "list", "tags": "meeting"}
```

---

## Architecture

### Interface-Based Design

```go
// MemoryStore interface - swappable implementations
type MemoryStore interface {
    Store(content string, tags []string, metadata map[string]interface{}) (string, error)
    Search(query string, limit int) ([]SearchResult, error)
    Retrieve(id string) (*Memory, error)
    List(tags []string) ([]Memory, error)
    Delete(id string) error
    Close() error
}

// Implementation: BM25Store (SQLite FTS5)
```

### Storage Structure

**Location**: `~/.memdoor/memory/{agentId}.sqlite`
- `main.sqlite` - Default agent / CLI usage
- `agent1.sqlite` - Agent "agent1" memories
- `agent2.sqlite` - Agent "agent2" memories

Each agent has its own isolated memory database for privacy and organization.

**Schema**:
```sql
-- Main memories table
CREATE TABLE memories (
    id TEXT PRIMARY KEY,              -- mem_1771710685125870000
    content TEXT NOT NULL,            -- The actual memory content
    tags TEXT,                        -- JSON array: ["tag1", "tag2"]
    metadata TEXT,                    -- JSON object: {"key": "value"}
    created_at INTEGER NOT NULL,      -- Unix timestamp
    updated_at INTEGER NOT NULL       -- Unix timestamp
);

-- FTS5 virtual table for full-text search
CREATE VIRTUAL TABLE memories_fts USING fts5(
    id UNINDEXED,
    content,
    tags,
    content='memories',
    content_rowid='rowid'
);

-- Triggers keep FTS5 in sync automatically
```

---

## Build Requirements

**IMPORTANT**: SQLite FTS5 must be enabled during compilation.

### Makefile (Automated)

The `gateway/Makefile` includes the necessary CGO flags:

```makefile
build:
    @cd ../cmd/cli && CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build ...
```

Just run `make build` from the root directory.

### Manual Build

If building manually:

```bash
export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"
go build -o memdoor ./cmd/cli
```

### Verification

Test FTS5 support:

```bash
export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"
go run test_memory_tool.go
```

All tests should pass 

---

## Implementation Files

### Core Memory System
- `gateway/memory/types.go` - Interfaces and types
- `gateway/memory/bm25_store.go` - SQLite FTS5 implementation
- `gateway/memory/cgo_sqlite.go` - CGO build flags

### Tool Integration
- `tools/memory_tools.go` - Memory tool definition and handlers
- `main.go` - Tool registration (line 49)

### Testing
- `test_memory_tool.go` - End-to-end test suite

---

## OpenClaw Parity

###  Implemented (Phase 1 Complete)

-  BM25 keyword search with proper scoring (OpenClaw's fallback mode)
-  SQLite FTS5 backend with bm25() ranking function
-  Store, search, retrieve, list, delete actions
-  Tag-based organization
-  Metadata support (JSON)
-  Per-agent memory databases (`~/.memdoor/memory/{agentId}.sqlite`)
-  CLI commands for direct access
-  Agent-specific isolation (memories don't leak between agents)

---

## Troubleshooting

### Error: "no such module: fts5"

**Cause**: SQLite compiled without FTS5 support

**Solution**:
```bash
# Always use make build (includes CGO flags)
make build

# OR set environment variable manually:
export CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"
go build ./...
```

### Memory Database Location

**Per-Agent Databases**: `~/.memdoor/memory/{agentId}.sqlite`
- CLI usage: `~/.memdoor/memory/main.sqlite`
- Agent "myagent": `~/.memdoor/memory/myagent.sqlite`

To inspect a specific agent's memories:
```bash
# List all memory databases
ls -lh ~/.memdoor/memory/

# Inspect main agent's memories
sqlite3 ~/.memdoor/memory/main.sqlite
SELECT * FROM memories;
.schema memories

# Inspect specific agent's memories
sqlite3 ~/.memdoor/memory/myagent.sqlite
SELECT * FROM memories;
```

---

## Credits

**Pattern**: OpenClaw memory tool architecture
**Implementation**: Memdoor with interface-based design for future extensibility
**Search**: SQLite FTS5 with BM25 ranking (same as OpenClaw's fallback mode)
