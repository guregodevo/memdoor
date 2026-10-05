# 🦞 the maintainer Gateway

**Status**: ✅ Phase 1 Complete - WebSocket Gateway MVP

A WebSocket-based gateway server for the maintainer, enabling real-time multi-session communication with AI agents.

## ⚠️ IMPORTANT - Development Guidelines

**ALWAYS use `make` commands for building and running:**

```bash
# ✅ CORRECT - Use make commands
cd gateway
make build              # Build the gateway
make run-verbose        # Run with logging

# ❌ WRONG - Don't use go commands directly
go build -o ...         # DON'T do this
go run cmd/main.go      # DON'T do this
```

**Why?** The Makefile ensures:
- Correct binary names (`greg-gateway`)
- Proper build flags and paths
- Consistent development workflow
- Easy onboarding for new developers

See `make help` for all available commands.

## 🎯 What We Built (Week 1)

### Core Components

1. **WebSocket Gateway Server** (`gateway/server.go`)
   - WebSocket endpoint at `ws://localhost:18789/ws`
   - HTTP health check at `/health`
   - Session listing at `/sessions`
   - API message endpoint at `/api/message`

2. **Session Management** (`gateway/session.go`)
   - Multi-session support (main, group, channel)
   - Message history per session
   - Metadata storage
   - Thread-safe operations

3. **Client Management**
   - WebSocket connection handling
   - Automatic ping/pong keepalive
   - Graceful connection cleanup
   - Message queueing

## 🚀 Quick Start

### Start the Gateway

**From project root:**
```bash
make gateway            # Run the gateway
make gateway-verbose    # Run with verbose logging
make build-gateway      # Build the binary
```

**From gateway folder:**
```bash
cd gateway
make run                # Run the gateway
make run-verbose        # Run with verbose logging
make build              # Build the binary
./greg-gateway     # Run the built binary
```

The gateway will start on port 18789 by default:
- **WebSocket**: `ws://localhost:18789/ws`
- **Health Check**: `http://localhost:18789/health`
- **Sessions**: `http://localhost:18789/sessions`

### Test with curl

```bash
# Check health
curl http://localhost:18789/health

# List sessions
curl http://localhost:18789/sessions

# Send message via API (TODO: not yet wired to agent)
curl -X POST http://localhost:18789/api/message \
  -H "Content-Type: application/json" \
  -d '{"session_id": "session_xxx", "message": "Hello"}'
```

### Test with WebSocket client (wscat)

```bash
# Install wscat if needed
npm install -g wscat

# Connect to gateway
wscat -c ws://localhost:18789/ws

# Send ping
> {"type": "ping"}

# Send chat message
> {"type": "chat", "data": {"text": "Hello gateway!"}}
```

## 📦 Project Structure

```
memdoor/
├── gateway/                  # All gateway code lives here (self-contained!)
│   ├── cmd/
│   │   └── main.go           # Gateway CLI entry point
│   ├── server.go             # WebSocket server and HTTP endpoints
│   ├── session.go            # Session management
│   ├── util.go               # Helper functions
│   ├── README.md             # This file
│   └── Makefile              # Gateway-specific build commands
└── Makefile                  # Root Makefile (delegates to gateway/Makefile)
```

## 🔌 WebSocket Protocol

### Client → Server Messages

**Ping**
```json
{
  "type": "ping"
}
```

**Chat Message**
```json
{
  "type": "chat",
  "data": {
    "text": "Your message here"
  }
}
```

### Server → Client Messages

**Connection Established**
```json
{
  "type": "connected",
  "session_id": "session_abc123...",
  "data": {
    "client_id": "client_def456...",
    "message": "Connected to the maintainer Gateway"
  }
}
```

**Pong Response**
```json
{
  "type": "pong"
}
```

**Chat Response**
```json
{
  "type": "chat_response",
  "session_id": "session_abc123...",
  "data": {
    "text": "Response text"
  }
}
```

**Error**
```json
{
  "type": "error",
  "error": "Error message"
}
```

## 🎯 What Works Now

✅ **WebSocket server** running on port 18789
✅ **Session management** with automatic session creation
✅ **Client connection handling** with keepalive
✅ **Message routing** (basic echo functionality)
✅ **Health check** endpoint
✅ **Session listing** endpoint
✅ **Graceful shutdown** handling
✅ **Test client** (HTML-based)

## 📋 Next Steps (Week 2)

### Priority 1: Agent Integration
- [ ] Wire gateway to Taskforce agent
- [ ] Route chat messages to agent
- [ ] Stream agent responses back to client
- [ ] Handle tool calls and results

### Priority 2: Configuration System
- [ ] Load config from YAML/JSON file
- [ ] Support for multiple ports
- [ ] CORS configuration
- [ ] TLS/SSL support

### Priority 3: Enhanced Features
- [ ] Session persistence (Redis)
- [ ] Message history pagination
- [ ] Typing indicators
- [ ] Presence management

## 🏗️ Architecture

```
┌─────────────┐
│   Browser   │
│   (WebUI)   │
└──────┬──────┘
       │ WebSocket
       │
┌──────▼───────────────────────────┐
│     Gateway Server               │
│                                  │
│  ┌────────────────────────────┐ │
│  │  WebSocket Handler         │ │
│  │  - Connection mgmt         │ │
│  │  - Message routing         │ │
│  └────────────┬───────────────┘ │
│               │                  │
│  ┌────────────▼───────────────┐ │
│  │  Session Manager           │ │
│  │  - Multi-session support   │ │
│  │  - Message history         │ │
│  └────────────┬───────────────┘ │
│               │                  │
│  ┌────────────▼───────────────┐ │
│  │  Agent Runtime (TODO)      │ │
│  │  - Taskforce API calls     │ │
│  │  - Tool execution          │ │
│  └────────────────────────────┘ │
└──────────────────────────────────┘
```

## 🛠️ Development

### Run tests
```bash
make test
```

### Format code
```bash
make fmt
```

### Check code
```bash
make check
```

### Clean build artifacts
```bash
make clean
```

## 🔍 Troubleshooting

### Port already in use
```bash
# Find process using port 18789
lsof -i :18789

# Kill the process
kill -9 <PID>
```

### WebSocket connection refused
- Make sure gateway is running: `make gateway`
- Check firewall settings
- Verify port 18789 is accessible

### Can't see messages in test client
- Open browser developer console (F12)
- Check for JavaScript errors
- Verify WebSocket connection is established (should show "Connected")

## 📊 Monitoring

### Health Check
```bash
curl http://localhost:18789/health
```

Expected response:
```json
{
  "status": "healthy",
  "clients": 2,
  "sessions": 1
}
```

### List Active Sessions
```bash
curl http://localhost:18789/sessions
```

Example response:
```json
{
  "sessions": [
    {
      "id": "session_abc123...",
      "type": "main",
      "created_at": "2026-02-17T20:00:00Z",
      "updated_at": "2026-02-17T20:05:00Z",
      "metadata": {},
      "messages": [...]
    }
  ]
}
```

## 🎉 Achievement Unlocked!

We successfully completed **Phase 1, Week 1** of the the maintainer roadmap:

✅ Implemented WebSocket gateway server
✅ Created basic session management
✅ Added message routing and handling
✅ Built test client for verification

**What's next**: Week 2 will focus on integrating the Taskforce agent so we can have real AI-powered conversations through the gateway!

---

**Built with**: Go 1.24, gorilla/websocket
**License**: Same as Memdoor
**Author**: @guregodevo
**Date**: 2026-02-17