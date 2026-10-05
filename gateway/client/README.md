# the maintainer WebSocket Client Library

Common WebSocket client library for connecting to the the maintainer gateway.

## Overview

This package provides a **thread-safe, RFC-compliant WebSocket client** that handles:
- ✅ Per-message compression (RFC 7692) - receives compressed messages from gateway
- ✅ Ping/pong keep-alive (RFC 6455) - responds to server pings automatically
- ✅ Protocol error prevention - explicitly disables write compression to avoid RSV2 bugs
- ✅ Type-safe message handlers using `protocol.Message`
- ✅ Connection lifecycle events (connected, disconnected, error)
- ✅ Structured logging (optional)

**Pattern**: Shared client library for TUI, CLI, and future clients

---

## Quick Start

```go
package main

import (
    "chat/gateway/client"
    "chat/gateway/protocol"
    "fmt"
)

func main() {
    // Create client with defaults
    c := client.New("http://localhost:18789", "my-session", client.DefaultOptions())

    // Register message handlers
    c.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
        fmt.Printf("Agent event: %+v\n", msg.Data)
        return nil
    })

    // Register event handler
    c.OnEvent(func(event client.Event) {
        fmt.Printf("Connection event: %s\n", event.Type)
    })

    // Connect and start listening
    if err := c.Connect(); err != nil {
        panic(err)
    }
    defer c.Close()

    c.Listen() // Start background read loop

    // Send chat message
    c.SendChat("Hello, the maintainer!")

    // Keep running
    select {}
}
```

---

## Architecture

### Thread Safety

The client is **fully thread-safe** and uses:
- **`writeMu`** (Mutex) - Protects WebSocket writes (RFC 6455 requirement)
- **`mu`** (RWMutex) - Protects connection state and closed flag
- **`handlerMu`** (RWMutex) - Protects message handler registration

**Concurrency guarantees**:
- Multiple goroutines can call `SendMessage()` / `SendChat()` concurrently
- Handlers are called sequentially in the read loop (no concurrent handler execution)
- `Close()` is idempotent and safe to call multiple times
- `OnMessage()` can register handlers while client is running

### Compression Handling (RFC 7692)

**Critical fix for RSV2 protocol errors:**

```go
// In SendMessage():
c.conn.EnableWriteCompression(false) // ✅ Prevents RSV2 bugs
err := c.conn.WriteJSON(msg)
```

**Why this works:**
1. **Connection-level**: `EnableCompression: true` in dialer → Client CAN receive compressed messages
2. **Per-message level**: `EnableWriteCompression(false)` before send → Client does NOT send compressed messages
3. **Result**: Gateway can send 8KB+ compressed messages, client receives them correctly, but client sends uncompressed (avoiding RSV2 bugs)

**Compression flow:**
```
Gateway → Client:  Compressed (if >8KB) ✅ Works
Client → Gateway:  Uncompressed         ✅ Works
```

### Ping/Pong Keep-Alive (RFC 6455)

**Server behavior** (gateway sends ping every 54s, expects pong within 120s):
```go
// Gateway sends ping
ticker := time.NewTicker(54 * time.Second)
conn.SetReadDeadline(time.Now().Add(120 * time.Second))
```

**Client behavior** (automatically responds to server pings):
```go
conn.SetPingHandler(func(appData string) error {
    // CRITICAL: Gorilla WebSocket does NOT auto-respond on client side
    return conn.WriteControl(websocket.PongMessage, []byte(appData), deadline)
})
```

**Why this is needed:**
- Gorilla WebSocket auto-responds to pings **ONLY on server side**
- Client **MUST manually implement ping handler**
- Without this, connection times out after 120s

---

## API Reference

### Creating a Client

```go
func New(gatewayURL string, sessionKey string, options Options) *Client
```

**Parameters:**
- `gatewayURL` - Gateway HTTP/WebSocket URL (e.g., `"http://localhost:18789"` or `"ws://localhost:18789"`)
- `sessionKey` - Session identifier (e.g., `"main:default:my-session"`)
- `options` - Client configuration (see Options below)

**Example:**
```go
opts := client.Options{
    EnableLogging: true,  // Enable for CLI, false for TUI
    ReadLimit:     20 * 1024 * 1024,  // 20MB limit
}
c := client.New("http://localhost:18789", "main:default:cli", opts)
```

### Options

```go
type Options struct {
    ReadBufferSize  int           // Buffer for reading (default: 1MB)
    WriteBufferSize int           // Buffer for writing (default: 1MB)
    ReadLimit       int64         // Max message size (default: 10MB)
    EnableLogging   bool          // Structured logging (default: false)
    PingInterval    time.Duration // Client ping interval (default: 0 = server controls)
    PongTimeout     time.Duration // Pong timeout (default: 10s)
}
```

**Defaults:**
```go
client.DefaultOptions() // Returns sensible defaults
```

### Connecting

```go
func (c *Client) Connect() error
```

Establishes WebSocket connection to gateway. Automatically:
- Converts `http://` → `ws://` and `https://` → `wss://`
- Adds `/ws` path
- Adds `?session=<sessionKey>` query parameter
- Sets up ping handler
- Emits `EventConnected` event

**Example:**
```go
if err := c.Connect(); err != nil {
    log.Fatalf("Failed to connect: %v", err)
}
defer c.Close()
```

### Listening for Messages

```go
func (c *Client) Listen()
```

Starts background goroutine to read messages from WebSocket.

**IMPORTANT**: Call this AFTER registering message handlers.

**Example:**
```go
c.OnMessage(protocol.MessageTypeAgentEvent, handleAgentEvent)
c.OnMessage(protocol.MessageTypeChatResponse, handleChatResponse)
c.Listen() // Start reading
```

### Registering Message Handlers

```go
func (c *Client) OnMessage(messageType string, handler MessageHandler)
```

Registers a handler for a specific message type.

**Handler signature:**
```go
type MessageHandler func(msg protocol.Message) error
```

**Message types** (from `protocol` package):
- `MessageTypeConnected` - Connection confirmation
- `MessageTypeAgentEvent` - Agent lifecycle/tool/assistant events
- `MessageTypeChatResponse` - Chat response from gateway
- `MessageTypeError` - Error message
- etc.

**Example:**
```go
c.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
    // Extract stream type
    stream, _ := msg.Data["stream"].(string)

    switch stream {
    case "lifecycle":
        // Handle lifecycle events
    case "tool":
        // Handle tool execution
    case "assistant":
        // Handle assistant responses
    }

    return nil
})
```

### Registering Event Handlers

```go
func (c *Client) OnEvent(handler EventHandler)
```

Registers a handler for connection lifecycle events.

**Handler signature:**
```go
type EventHandler func(event Event)
```

**Event types:**
- `EventConnected` - WebSocket connected
- `EventDisconnected` - WebSocket disconnected (normal or unexpected)
- `EventError` - Protocol error or unexpected close

**Example:**
```go
c.OnEvent(func(event client.Event) {
    switch event.Type {
    case client.EventConnected:
        fmt.Println("Connected to gateway")
    case client.EventDisconnected:
        fmt.Println("Disconnected from gateway")
    case client.EventError:
        fmt.Printf("Error: %v\n", event.Error)
    }
})
```

### Sending Messages

```go
func (c *Client) SendChat(text string) error
func (c *Client) SendMessage(msg protocol.Message) error
```

**SendChat** - Convenience method for sending chat messages:
```go
err := c.SendChat("Hello!")
```

**SendMessage** - Send any protocol message:
```go
msg := protocol.Message{
    Type: protocol.MessageTypeChat,
    Data: map[string]interface{}{
        "text": "Custom message",
    },
}
err := c.SendMessage(msg)
```

### Closing the Connection

```go
func (c *Client) Close() error
```

Closes the WebSocket connection. Idempotent - safe to call multiple times.

**Example:**
```go
defer c.Close()
```

### Checking Connection Status

```go
func (c *Client) IsConnected() bool
```

Returns `true` if client is connected, `false` otherwise.

**Example:**
```go
if c.IsConnected() {
    c.SendChat("I'm still connected!")
}
```

---

## Usage Examples

### TUI Client (No Logging)

```go
package main

import (
    "chat/gateway/client"
    "chat/gateway/protocol"
    tea "github.com/charmbracelet/bubbletea"
)

func main() {
    // Create client with logging disabled (TUI mode)
    opts := client.DefaultOptions()
    opts.EnableLogging = false  // Don't pollute TUI output

    c := client.New("http://localhost:18789", "main:default:tui", opts)

    // Register handlers that send Bubble Tea messages
    c.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
        program.Send(agentEventMsg{data: msg.Data})
        return nil
    })

    c.Connect()
    defer c.Close()
    c.Listen()

    // Run TUI
    program := tea.NewProgram(initialModel())
    program.Run()
}
```

### CLI Client (With Logging)

```go
package main

import (
    "chat/gateway/client"
    "chat/gateway/protocol"
    "fmt"
)

func main() {
    // Create client with logging enabled (CLI mode)
    opts := client.DefaultOptions()
    opts.EnableLogging = true  // Show connection logs

    c := client.New("http://localhost:18789", "main:default:cli", opts)

    // Register handlers that print to stdout
    c.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
        fmt.Printf("Agent: %+v\n", msg.Data)
        return nil
    })

    c.OnEvent(func(event client.Event) {
        fmt.Printf("Event: %s\n", event.Type)
    })

    if err := c.Connect(); err != nil {
        panic(err)
    }
    defer c.Close()

    c.Listen()

    // Send message
    c.SendChat("Hello from CLI!")

    // Keep running
    select {}
}
```

### Handling All Message Types

```go
c.OnMessage(protocol.MessageTypeConnected, func(msg protocol.Message) error {
    fmt.Println("Connected to gateway")
    return nil
})

c.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
    stream, _ := msg.Data["stream"].(string)
    data, _ := msg.Data["data"].(map[string]interface{})
    event, _ := data["event"].(string)

    fmt.Printf("[%s] %s\n", stream, event)
    return nil
})

c.OnMessage(protocol.MessageTypeChatResponse, func(msg protocol.Message) error {
    text, _ := msg.Data["text"].(string)
    fmt.Printf("Response: %s\n", text)
    return nil
})

c.OnMessage(protocol.MessageTypeError, func(msg protocol.Message) error {
    fmt.Printf("Error: %s\n", msg.Error)
    return nil
})
```

---

## Testing

See `websocket_test.go` for comprehensive integration tests.

**Run tests:**
```bash
go test -v chat/gateway/client
```

**Test coverage:**
- ✅ Connection establishment
- ✅ Ping/pong keep-alive
- ✅ Compression handling (receive compressed, send uncompressed)
- ✅ Message routing
- ✅ Event emission
- ✅ Concurrent sends
- ✅ Graceful disconnection

---

## Troubleshooting

### RSV2 Protocol Errors

**Symptom**: `websocket: close 1002 (protocol error): RSV2 set`

**Cause**: Client is sending compressed messages with incorrect reserved bits.

**Fix**: The common client library ALREADY fixes this by calling `EnableWriteCompression(false)` before every send.

**If using custom WebSocket code**: Add this before `WriteJSON()`:
```go
conn.EnableWriteCompression(false)  // Disable compression for client->server
err := conn.WriteJSON(msg)
```

### Connection Timeout After 120s

**Symptom**: Connection closes after ~2 minutes of inactivity

**Cause**: Client is not responding to server pings.

**Fix**: The common client library ALREADY fixes this with `SetPingHandler()`.

**If using custom WebSocket code**: Add ping handler after dial:
```go
conn.SetPingHandler(func(appData string) error {
    return conn.WriteControl(websocket.PongMessage, []byte(appData), deadline)
})
```

### Cannot Receive Large Messages (>8KB)

**Symptom**: Protocol errors or truncated messages when receiving large tool outputs

**Cause**: Compression disabled at connection level.

**Fix**: The common client library ALREADY fixes this with `EnableCompression: true` in dialer.

**If using custom WebSocket code**: Enable compression negotiation:
```go
dialer := &websocket.Dialer{
    EnableCompression: true,  // Allow receiving compressed messages
}
```

---

## Migration Guide

### From TUI Custom Client

**Before** (`cmd/tui/ui/websocket.go`):
```go
type WebSocketClient struct {
    conn *websocket.Conn
    // ... custom implementation
}

func (c *WebSocketClient) Connect() error {
    dialer := &websocket.Dialer{...}
    conn, _, err := dialer.Dial(url, nil)
    // ... custom logic
}
```

**After** (using common client):
```go
import "chat/gateway/client"

opts := client.DefaultOptions()
opts.EnableLogging = false  // TUI mode

c := client.New(gatewayURL, sessionKey, opts)
c.OnMessage(protocol.MessageTypeAgentEvent, handleAgentEvent)
c.Connect()
c.Listen()
```

**Benefits**:
- ✅ RSV2 bug automatically fixed
- ✅ Ping/pong handling automatic
- ✅ Compression configured correctly
- ✅ Less code to maintain
- ✅ Consistent behavior across all clients

---

## RFC Compliance

This client implements:

- **RFC 6455** - WebSocket Protocol
  - Ping/pong keep-alive (§5.5.2, §5.5.3)
  - Control frame handling (§5.5)
  - Reserved bits (RSV1, RSV2, RSV3)

- **RFC 7692** - WebSocket Per-Message Compression
  - Compression negotiation (§4)
  - Per-message deflate (§7)
  - Control frames never compressed (§6)

- **RFC 1951** - DEFLATE compression algorithm

---

## Pattern: OpenClaw Compatibility

This client follows OpenClaw's WebSocket patterns:
- Protocol message types match OpenClaw's TypeScript definitions
- Compression threshold (8KB) matches OpenClaw server
- Event-driven architecture compatible with OpenClaw clients

**Cross-compatibility**: Go clients using this library can communicate with OpenClaw TypeScript servers and vice versa.

---

## Credits

**Pattern**: OpenClaw WebSocket client architecture
**RFC Compliance**: Gorilla WebSocket + custom extensions
**Author**: the maintainer Team
**License**: MIT

🦞 Built with Claude Code
