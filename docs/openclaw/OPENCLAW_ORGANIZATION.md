# OpenClaw Organization Pattern

**Date**: 2026-02-17

##  OpenClaw's Separation of Concerns

### Directory Structure

```
src/
├── gateway/                    # Gateway server
│   ├── server.ts              # Barrel file (exports)
│   ├── server.impl.ts         # Main server implementation
│   ├── server-*.ts            # Server component files
│   ├── session-utils.ts       # Session management
│   ├── protocol/              # Protocol definitions
│   │   ├── index.ts           # Message types, schemas
│   │   ├── schema/            # Schema definitions
│   │   └── client-info.ts     # Client information
│   ├── server/                # Server infrastructure
│   │   ├── ws-connection.ts   # WebSocket connection handling
│   │   ├── health-state.ts    # Health monitoring
│   │   ├── http-listen.ts     # HTTP server setup
│   │   ├── hooks.ts           # Hooks system
│   │   └── tls.ts             # TLS configuration
│   └── server-methods/        # RPC-style method handlers
│       ├── agent.ts           # Agent operations
│       ├── chat.ts            # Chat handling
│       ├── sessions.ts        # Session operations
│       ├── health.ts          # Health checks
│       ├── config.ts          # Configuration
│       └── ...                # Other methods
├── infra/                     # Cross-cutting infrastructure
│   ├── agent-events.ts        # Event system ⚡
│   ├── outbound/              # Delivery system
│   │   ├── agent-delivery.ts  # Agent delivery logic
│   │   └── targets.ts         # Target resolution
│   ├── net/                   # Networking utilities
│   └── ...
├── agents/                    # Agent-related code
│   ├── tools/                 # Tool implementations
│   ├── skills/                # Skills
│   └── schema/                # Agent schemas
├── config/                    # Configuration
│   ├── sessions/              # Session config
│   └── ...
└── utils/                     # Utility functions
```

## 📋 Key Patterns

### 1. **Gateway Organization**

**Main Files** (`gateway/`)
- `server.impl.ts` - Main server implementation
- `session-utils.ts` - Session management utilities
- `session-utils.fs.ts` - File-based session persistence

**Protocol** (`gateway/protocol/`)
- Message types and schemas
- Protocol definitions
- Client information structures

**Server Infrastructure** (`gateway/server/`)
- WebSocket connection management
- HTTP server setup
- Health monitoring
- TLS configuration
- Server-level utilities

**Server Methods** (`gateway/server-methods/`)
- Individual handler files for each operation type
- RPC-style method implementations
- Clear separation by functionality:
  - `chat.ts` - Chat operations
  - `agent.ts` - Agent operations
  - `sessions.ts` - Session operations
  - `health.ts` - Health checks
  - `config.ts` - Configuration management

### 2. **Infrastructure** (`infra/`)

Cross-cutting concerns that span multiple components:
- **Event System** (`agent-events.ts`)
  - Event emitter with monotonic sequence numbers
  - Listener registration/unregistration
  - Fire-and-forget event delivery

- **Outbound Delivery** (`outbound/`)
  - Agent delivery planning
  - Target resolution
  - Channel routing

- **Networking** (`net/`, `tls/`)
  - Network utilities
  - TLS configuration
  - Connection management

### 3. **Why This Organization?**

**Clear Boundaries:**
- Protocol definitions separate from implementation
- Infrastructure code reusable across components
- Handler logic isolated by operation type

**Testability:**
- Each handler can be tested independently
- Protocol can be tested separately
- Infrastructure has clear interfaces

**Scalability:**
- Easy to add new handlers (just add a file in server-methods/)
- Protocol changes isolated to protocol/
- Infrastructure changes don't affect business logic

**Discoverability:**
- New developers know where to find code
- Logical grouping by responsibility
- Clear naming conventions

## 🔄 Applying to Memdoor

### Current Structure (Before)

```
gateway/
├── cmd/main.go
├── server.go                   # Everything mixed together
├── agent_adapter.go
├── session.go
├── session_persistence.go
├── agent_events.go            # Should be in infra/
└── util.go
```

### New Structure (After)

```
gateway/
├── cmd/main.go                # Entry point
├── server.go                  # Main server (like server.impl.ts)
├── session.go                 # Session management (like session-utils.ts)
├── session_persistence.go     # Persistence (like session-utils.fs.ts)
├── agent_adapter.go           # Agent runtime
├── util.go                    # Utilities
├── protocol/                  # Protocol definitions
│   └── types.go               # Message types
├── server/                    # Server infrastructure
│   ├── ws_connection.go       # WebSocket handling
│   └── health.go              # Health checks
├── handlers/                  # Method handlers (like server-methods/)
│   ├── chat.go                # Chat handler
│   ├── ping.go                # Ping handler
│   └── session.go             # Session methods
└── infra/                     # Cross-cutting infrastructure
    └── events.go              # Event system

# Top-level (outside gateway/)
infra/                         # Could also be here for shared infra
└── events.go
```

## 📝 Migration Steps

1.  Create `protocol/` for message types
2.  Create `server/` for WebSocket connection handling
3.  Create `handlers/` for method handlers
4.  Move `agent_events.go` to `infra/`
5. ⏳ Extract chat handling to `handlers/chat.go`
6. ⏳ Extract ping handling to `handlers/ping.go`
7. ⏳ Update imports and package references
8. ⏳ Test everything still works

## 🎓 Key Takeaways

1. **Separate concerns by layer:**
   - Protocol (definitions)
   - Server (infrastructure)
   - Handlers (business logic)
   - Infrastructure (cross-cutting)

2. **Follow OpenClaw's naming:**
   - `server-methods/` → `handlers/` (in Go)
   - `server.impl.ts` → `server.go`
   - `session-utils.ts` → `session.go`

3. **Keep it simple:**
   - Don't over-engineer
   - Start with OpenClaw's proven patterns
   - Add structure as needed

---

**Pattern Source**: OpenClaw `src/gateway/` structure
**Applied**: Memdoor Gateway reorganization
**Date**: 2026-02-17
