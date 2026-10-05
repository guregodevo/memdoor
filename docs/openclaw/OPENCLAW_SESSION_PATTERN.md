# OpenClaw Session Management Pattern

**Discovery Date**: 2026-02-17

## 🔍 The Problem

In our current implementation, each WebSocket connection creates a **new session**:
```go
// server.go:127
session, err := s.sessions.CreateSession("main")  // Always creates NEW session
```

This means:
- Connection 1: `session_901ca3402d3518fb`
- Connection 2: `session_75aa8bf621a4e4be` (DIFFERENT!)
- Conversation history is saved  but not loaded for new connections 

##  OpenClaw's Solution

### Stable Session Keys

OpenClaw uses **`resolveMainSessionKey()`** which returns a STABLE session identifier:

```typescript
// src/config/sessions/main-session.ts:11-24
export function resolveMainSessionKey(cfg?: {
  session?: { scope?: SessionScope; mainKey?: string };
  agents?: { list?: Array<{ id?: string; default?: boolean }> };
}): string {
  if (cfg?.session?.scope === "global") {
    return "global";  // Shared across all agents
  }
  const agents = cfg?.agents?.list ?? [];
  const defaultAgentId =
    agents.find((agent) => agent?.default)?.id ?? agents[0]?.id ?? DEFAULT_AGENT_ID;
  const agentId = normalizeAgentId(defaultAgentId);
  const mainKey = normalizeMainKey(cfg?.session?.mainKey);
  return buildAgentMainSessionKey({ agentId, mainKey });
}
```

### Key Insights

1. **Configuration-Based**: Session key is derived from config, not randomly generated
2. **Stable "main" Session**: The "main" session always has the same ID
3. **Scope Options**:
   - `global`: Shared session across all agents
   - `agent-specific`: Each agent has its own "main" session
4. **Alias Support**: "main" can be aliased to different underlying sessions

### Usage Pattern

```typescript
// src/gateway/boot.ts:76
const sessionKey = resolveMainSessionKey(params.cfg);
await agentCommand({
  message,
  sessionKey,  // This is the SAME key every time
  deliver: false,
});
```

##  How We Should Adapt This

### Option 1: Simple "main" Session (Recommended)

Use a **fixed session ID** for the main session:

```go
// In server.go
const MAIN_SESSION_ID = "session_main"

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // ...

    // Get or create main session (always the same ID)
    session, err := s.sessions.GetOrCreateSession(MAIN_SESSION_ID, "main")
    if err != nil {
        log.Printf("Failed to get/create session: %v", err)
        conn.Close()
        return
    }
    client.Session = session

    // ...
}
```

### Option 2: Session Query Parameter

Allow clients to specify session via URL:
```
ws://localhost:18789/ws?session=session_main
ws://localhost:18789/ws?session=session_abc123
```

```go
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Parse session from query param
    sessionID := r.URL.Query().Get("session")
    if sessionID == "" {
        sessionID = "session_main"  // Default to main
    }

    // Get or create
    session, err := s.sessions.GetOrCreateSession(sessionID, "main")
    // ...
}
```

### Option 3: Client-Sent Session ID

Client sends session ID in first message:
```json
{
  "type": "connect",
  "session_id": "session_main"
}
```

##  Comparison

| Approach | OpenClaw | Memdoor Current | Proposed |
|----------|----------|-------------------|----------|
| Session Creation | Config-based stable key | Random per connection | Fixed "main" or URL param |
| Reconnection | Automatic (same key) | Creates new session | Get-or-create pattern |
| Persistence | JSONL files | JSONL files  | JSONL files  |
| Multi-session | Agent-scoped | Not supported | URL param support |

##  Implementation Plan

### Step 1: Add GetOrCreateSession

```go
// In session.go
func (sm *SessionManager) GetOrCreateSession(id, sessionType string) (*Session, error) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    // Check if session exists
    if session, exists := sm.sessions[id]; exists {
        return session, nil
    }

    // Create new session with specific ID
    session := &Session{
        ID:        id,  // Use provided ID, not generated
        Type:      sessionType,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        Metadata:  make(map[string]interface{}),
        Messages:  []SessionMessage{},
    }

    sm.sessions[id] = session
    return session, nil
}
```

### Step 2: Update handleWebSocket

```go
// In server.go
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := s.upgrader.Upgrade(w, r, nil)
    if err != nil {
        if s.verbose {
            log.Printf("WebSocket upgrade failed: %v", err)
        }
        return
    }

    // Get session ID from query param or use default
    sessionID := r.URL.Query().Get("session")
    if sessionID == "" {
        sessionID = "session_main"  // Stable default session
    }

    // Get or create session (reuse existing if available)
    session, err := s.sessions.GetOrCreateSession(sessionID, "main")
    if err != nil {
        if s.verbose {
            log.Printf("Failed to get/create session: %v", err)
        }
        conn.Close()
        return
    }

    // Rest of the code remains the same...
}
```

### Step 3: Update WebSocket Clients

```javascript
// In WebSocket client
const sessionId = 'session_main';  // Or load from localStorage
const ws = new WebSocket(`ws://localhost:18789/ws?session=${sessionId}`);
```

##  Expected Behavior After Fix

```
Connection 1:
  URL: ws://localhost:18789/ws?session=session_main
  → Gets/creates: session_main
  → Saves: "My name is Alice"

Connection 2:
  URL: ws://localhost:18789/ws?session=session_main
  → Gets EXISTING: session_main
  → Loads: Previous conversation with Alice
  → AI remembers: "Your name is Alice!"
```

## 📝 Summary

**OpenClaw's Pattern:**
-  Configuration-based stable session keys
-  "main" session is always the same ID
-  Conversation history persists across connections
-  Multiple session scopes (global, agent-specific)

**Our Current State:**
-  Conversation persistence (JSONL files)
-  Message save/load working
-  Random session IDs per connection
-  No way to reconnect to existing session

**Next Step:**
1. Implement `GetOrCreateSession(id, type)` in session manager
2. Use fixed "session_main" as default
3. Optionally support URL query param for multi-session
4. Test conversation persistence across reconnections

---

**Pattern**: Stable session identifiers + get-or-create
**Benefit**: Conversations persist across WebSocket reconnections
**Complexity**: Low (just change session creation logic)
**OpenClaw Source**: `src/config/sessions/main-session.ts`