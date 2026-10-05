package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/protocol"

	"github.com/gorilla/websocket"
)

// Client is a common WebSocket client for connecting to the Memdoor gateway
// Thread-safe: All methods can be called concurrently
// Pattern: Shared client library for TUI, CLI, and future clients
//
// Usage:
//
//	client := client.New(gatewayURL, sessionKey, options)
//	client.Connect()
//	client.OnMessage(func(msg protocol.Message) { ... })
//	client.SendChat("hello", "")
//	client.Close()
type Client struct {
	// Connection
	conn       *websocket.Conn
	gatewayURL string
	sessionKey string

	// Concurrency control
	writeMu sync.Mutex   // Protects WebSocket writes (RFC 6455 §5.1: single concurrent writer)
	mu      sync.RWMutex // Protects closed flag and connection state
	closed  bool

	// Message handlers
	handlers     map[string]MessageHandler
	handlerMu    sync.RWMutex
	eventHandler EventHandler

	// Logging
	log *logs.EventLogger

	// Options
	options Options
}

// Options configures the WebSocket client
type Options struct {
	// ReadBufferSize is the buffer size for reading messages (default: 1MB)
	ReadBufferSize int

	// WriteBufferSize is the buffer size for writing messages (default: 1MB)
	WriteBufferSize int

	// ReadLimit is the maximum message size in bytes (default: 10MB)
	ReadLimit int64

	// EnableLogging enables structured logging (default: false for TUI, true for CLI)
	EnableLogging bool

	// PingInterval is how often to send ping frames (0 = server controls)
	// Client doesn't need to send pings - only respond to server pings
	PingInterval time.Duration

	// PongTimeout is how long to wait for pong response (only if PingInterval > 0)
	PongTimeout time.Duration

	// CompressionThreshold is the message size above which compression is enabled (default: 8KB)
	// Set to 0 to disable compression, -1 to always compress
	CompressionThreshold int

	// Workspace is the workspace slug sent as the ?workspace= handshake param,
	// so the gateway scopes this connection's agent runs. Empty lets the
	// gateway use its single workspace.
	Workspace string

	// Channel is the channel id this connection subscribes to. With Workspace,
	// the gateway builds the conversation session key server-side
	// (?workspace=&channel=), so the client never reconstructs it.
	Channel string

	// Token is the bearer token sent on the handshake. The gateway lets an
	// unproxied loopback connection act without one; anything reached through
	// a reverse proxy is an anonymous listener unless this names a user.
	Token string
}

// MessageHandler handles incoming messages by type
type MessageHandler func(msg protocol.Message) error

// EventHandler handles connection lifecycle events
type EventHandler func(event Event)

// Event represents a connection lifecycle event
type Event struct {
	Type      EventType
	Error     error
	Timestamp time.Time
}

// EventType categorizes connection events
type EventType string

const (
	EventConnected    EventType = "connected"
	EventDisconnected EventType = "disconnected"
	EventError        EventType = "error"
)

// DefaultOptions returns sensible defaults
func DefaultOptions() Options {
	return Options{
		ReadBufferSize:       1024 * 1024,      // 1MB
		WriteBufferSize:      1024 * 1024,      // 1MB
		ReadLimit:            10 * 1024 * 1024, // 10MB
		EnableLogging:        false,            // Disable by default (TUI mode)
		PingInterval:         0,                // Server controls pings
		PongTimeout:          10 * time.Second,
		CompressionThreshold: 8 * 1024, // 8KB - matches gateway default
	}
}

// New creates a new WebSocket client
func New(gatewayURL string, sessionKey string, options Options) *Client {
	// Apply defaults for zero values
	if options.ReadBufferSize == 0 {
		options.ReadBufferSize = DefaultOptions().ReadBufferSize
	}
	if options.WriteBufferSize == 0 {
		options.WriteBufferSize = DefaultOptions().WriteBufferSize
	}
	if options.ReadLimit == 0 {
		options.ReadLimit = DefaultOptions().ReadLimit
	}

	var log *logs.EventLogger
	if options.EnableLogging {
		log = logs.New("Client")
	}

	return &Client{
		gatewayURL: gatewayURL,
		sessionKey: sessionKey,
		options:    options,
		log:        log,
		handlers:   make(map[string]MessageHandler),
	}
}

// Connect establishes a WebSocket connection to the gateway
func (c *Client) Connect() error {
	// Parse gateway URL and add WebSocket path
	u, err := url.Parse(c.gatewayURL)
	if err != nil {
		return fmt.Errorf("invalid gateway URL: %w", err)
	}

	// Convert HTTP(S) to WS(S)
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// Already WebSocket
	default:
		return fmt.Errorf("unsupported scheme: %s (use http, https, ws, or wss)", u.Scheme)
	}

	// Add /ws path and session query parameter
	u.Path = "/ws"
	q := u.Query()
	q.Set("session", c.sessionKey)
	if w := strings.TrimSpace(c.options.Workspace); w != "" {
		q.Set("workspace", w)
	}
	if ch := strings.TrimSpace(c.options.Channel); ch != "" {
		q.Set("channel", ch)
	}
	u.RawQuery = q.Encode()

	// Configure dialer - DO NOT enable compression on client side
	// Pattern: Avoid Gorilla WebSocket compression bugs (RSV2/RSV3 protocol errors)
	// - Client does NOT negotiate compression during handshake
	// - Client can still decompress messages from server (Gorilla handles this automatically)
	// - Client NEVER sends compressed messages (no RSV2/RSV3 bugs)
	dialer := &websocket.Dialer{
		ReadBufferSize:    c.options.ReadBufferSize,
		WriteBufferSize:   c.options.WriteBufferSize,
		EnableCompression: false, // CRITICAL: Do NOT negotiate compression to avoid protocol errors
	}

	if c.log != nil {
		c.log.Debug("Connecting to gateway", slog.String("url", u.String()))
	}

	// Dial WebSocket. The token rides the header, not the URL, so it stays out
	// of proxy access logs.
	var header http.Header
	if t := strings.TrimSpace(c.options.Token); t != "" {
		header = http.Header{"Authorization": []string{"Bearer " + t}}
	}
	conn, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		if c.log != nil {
			c.log.WithError(err).Error("Failed to connect")
		}
		return fmt.Errorf("failed to connect: %w", err)
	}

	if resp != nil && c.log != nil {
		c.log.Debug("Handshake complete", slog.Int("status", resp.StatusCode))
	}

	// Set read limit
	conn.SetReadLimit(c.options.ReadLimit)

	// Set up ping handler - client MUST respond to server pings (RFC 6455)
	// Gorilla WebSocket does NOT auto-respond to pings on client side
	conn.SetPingHandler(func(appData string) error {
		if c.log != nil {
			c.log.Debug("Received ping from server, sending pong")
		}

		// Send pong response
		// CRITICAL: Control frames are NEVER compressed per RFC 7692
		c.writeMu.Lock()
		err := conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
		c.writeMu.Unlock()

		if err != nil && c.log != nil {
			c.log.WithError(err).Warn("Failed to send pong")
		}

		return err
	})

	c.conn = conn

	if c.log != nil {
		c.log.Info("WebSocket connected", slog.String("session", c.sessionKey))
	}

	// Emit connected event
	c.emit(Event{Type: EventConnected, Timestamp: time.Now()})

	return nil
}

// Listen starts the message read loop in a background goroutine
// Thread-safe: Can only be called once after Connect()
func (c *Client) Listen() {
	go c.readPump()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		if c.log != nil {
			c.log.Debug("Read pump exiting")
		}

		// Emit disconnected (thread-safe), then hand off to the reconnect loop —
		// a gateway restart must not leave a silently dead client.
		c.emit(Event{Type: EventDisconnected, Timestamp: time.Now()})
		if !c.isClosed() {
			go c.reconnectLoop()
		}
	}()

	for {
		frameType, message, err := c.conn.ReadMessage()
		if err != nil {
			// Check if it's a protocol error or expected close
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				if c.log != nil {
					c.log.WithError(err).Warn("WebSocket unexpected close")
				}

				c.emit(Event{Type: EventError, Error: err, Timestamp: time.Now()})
			} else {
				if c.log != nil {
					c.log.WithError(err).Debug("WebSocket closed normally")
				}
			}
			return
		}

		if c.log != nil {
			c.log.Debug("Received frame",
				slog.Int("type", frameType),
				slog.Int("size", len(message)))
		}

		// Parse message
		var msg protocol.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			if c.log != nil {
				c.log.WithError(err).Warn("Failed to parse message")
			}
			continue
		}

		// Dispatch to handler
		c.dispatchMessage(msg)
	}
}

// dispatchMessage routes messages to registered handlers
func (c *Client) dispatchMessage(msg protocol.Message) {
	c.handlerMu.RLock()
	handler, exists := c.handlers[msg.Type]
	c.handlerMu.RUnlock()

	if exists {
		if err := handler(msg); err != nil && c.log != nil {
			c.log.WithError(err).Warn("Handler error", slog.String("type", msg.Type))
		}
	} else if c.log != nil {
		c.log.Debug("No handler for message type", slog.String("type", msg.Type))
	}
}

// OnMessage registers a handler for a specific message type
// Thread-safe: Can be called while client is running
func (c *Client) OnMessage(messageType string, handler MessageHandler) {
	c.handlerMu.Lock()
	c.handlers[messageType] = handler
	c.handlerMu.Unlock()
}

// OnEvent registers a handler for connection lifecycle events
func (c *Client) OnEvent(handler EventHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.eventHandler = handler
}

// emit delivers a lifecycle event to the registered handler, thread-safely: the
// handler is read under handlerMu because the read pump, the reconnect loop and
// the main goroutine all emit concurrently with OnEvent registration.
func (c *Client) emit(ev Event) {
	c.handlerMu.RLock()
	h := c.eventHandler
	c.handlerMu.RUnlock()
	if h != nil {
		h(ev)
	}
}

// SendAnswer replies to an interactive ask_user_question (by question id), so the
// blocked tool call on the server unblocks and the agent run continues.
func (c *Client) SendAnswer(questionID, answer string) error {
	return c.SendMessage(protocol.Message{
		Type: protocol.MessageTypeAnswer,
		Data: map[string]interface{}{"question_id": questionID, "answer": answer},
	})
}

// SendChat sends a chat message to the gateway. permissionMode is the Claude-style
// interaction mode for the turn ("default"|"acceptEdits"|"plan"); empty is omitted
// and treated as default server-side.
// Thread-safe: Can be called by multiple goroutines concurrently
func (c *Client) SendChat(text, permissionMode string) error {
	data := map[string]interface{}{"text": text}
	if permissionMode != "" {
		data["permission_mode"] = permissionMode
	}
	// Claude-CLI semantics: the coder works on the project the client was
	// LAUNCHED from. Send our cwd; the gateway roots the coder's file/bash
	// confinement there (falling back to its own default when absent).
	if wd, err := os.Getwd(); err == nil {
		data["workdir"] = wd
	}
	msg := protocol.Message{
		Type: protocol.MessageTypeChat,
		Data: data,
	}
	return c.SendMessage(msg)
}

// SendMessage sends a protocol message to the gateway
// Thread-safe: Can be called by multiple goroutines concurrently
//
// Compression behavior:
// - Messages larger than CompressionThreshold are compressed (default: 8KB)
// - Smaller messages are sent uncompressed (avoids overhead)
// - Control frames (ping/pong/close) are NEVER compressed per RFC 7692
//
// Pattern: Matches gateway's per-message compression strategy
func (c *Client) SendMessage(msg protocol.Message) error {
	// First check: connection exists
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Acquire RLock first - check closed flag
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	// Now acquire write mutex (while still holding RLock)
	// This ensures Close() can't run between our check and the actual write
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Marshal to JSON first (for logging size)
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if c.log != nil {
		c.log.Debug("Sending message",
			slog.String("type", msg.Type),
			slog.Int("size", len(data)))
	}

	// Send message uncompressed (compression was not negotiated at connection level)
	// This avoids Gorilla WebSocket RSV2/RSV3 protocol errors
	err = c.conn.WriteJSON(msg)
	if err != nil && c.log != nil {
		c.log.WithError(err).Error("Failed to send message", slog.String("type", msg.Type))
	}

	return err
}

// Close closes the WebSocket connection
// Thread-safe: Idempotent - safe to call multiple times
// isClosed reports whether Close() was called deliberately (no reconnect then).
func (c *Client) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// reconnectLoop re-establishes a dropped connection with exponential backoff
// (1s -> 15s cap) until it succeeds or the client is deliberately closed. A
// gateway restart otherwise leaves the client silently dead: the TUI looks
// alive, input goes nowhere, and the run's emitted events are lost. On success
// Connect() emits EventConnected (the UI shows "reconnected") and a fresh read
// pump starts. The conn swap happens inside Connect under the same fields the
// send path nil-checks, and event emission is thread-safe via emit().
func (c *Client) reconnectLoop() {
	backoff := time.Second
	for {
		if c.isClosed() {
			return
		}
		time.Sleep(backoff)
		if c.isClosed() {
			return
		}
		if err := c.Connect(); err == nil {
			c.Listen()
			return
		}
		if backoff < 15*time.Second {
			backoff *= 2
			if backoff > 15*time.Second {
				backoff = 15 * time.Second
			}
		}
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil // Already closed
	}

	c.closed = true

	if c.conn != nil {
		if c.log != nil {
			c.log.Debug("Closing WebSocket connection")
		}
		return c.conn.Close()
	}

	return nil
}

// IsConnected returns true if the client is currently connected
// Thread-safe: Can be called concurrently
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.closed && c.conn != nil
}
