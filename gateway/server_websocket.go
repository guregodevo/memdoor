package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/pkg/shared"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memdoor/gateway/health"
	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
	"memdoor/pkg/sandbox"
	sharedctx "memdoor/pkg/shared/context"

	"github.com/gorilla/websocket"
)

// WebSocket connection handlers
// Pattern: OpenClaw WebSocket RPC protocol

const (
	// MaxWebSocketConnections limits concurrent WebSocket connections to prevent resource exhaustion
	// This prevents DoS attacks and ensures predictable resource usage under load
	MaxWebSocketConnections = 1000
)

// wsActorLocal is the actor of an unproxied loopback connection: the machine's
// own user, driving their own gateway.
const wsActorLocal = "local"

// wsAnonymousSession is the only conversation an anonymous listener is
// attached to — an empty one nothing runs in.
const wsAnonymousSession = "session_anonymous"

type wsTokenValidator interface {
	ValidateToken(token string) (userID string, err error)
}

// wsActor decides who a /ws connection acts as.
//
// A valid bearer token (Authorization header or ?token=) names its user.
// Without one, a connection that arrived on the loopback interface with no
// reverse-proxy forwarding headers is the machine's own user — the local TUI,
// which authenticates its REST calls but historically sent nothing on the
// socket. Everything else is anonymous: it may LISTEN to the broadcast stream
// and send nothing.
//
// Until 2026-08-27 there was no check at all. memdoor.ai's nginx proxies /ws,
// so wss://memdoor.ai/ws ran the coder — with a caller-chosen workdir and
// permission mode — for anyone who asked. nginx always sets X-Real-IP and
// X-Forwarded-For on that route; that is what keeps a remote caller from
// passing as local.
func wsActor(r *http.Request, validate wsTokenValidator) (string, error) {
	if token := wsBearerToken(r); token != "" && validate != nil {
		return validate.ValidateToken(token)
	}
	if wsUnproxiedLoopback(r) {
		return wsActorLocal, nil
	}
	return "", nil
}

func wsBearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func wsUnproxiedLoopback(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log := logs.New("WebSocket")

	var validator wsTokenValidator
	if s.authAdapter != nil {
		validator = s.authAdapter
	}
	actor, err := wsActor(r, validator)
	if err != nil {
		log.Warn("Connection rejected: invalid token", slog.String("remote", r.RemoteAddr))
		http.Error(w, "Invalid authentication token", http.StatusUnauthorized)
		return
	}

	// Check connection limit atomically (reserve a slot)
	s.clientsMu.Lock()
	connCount := len(s.clients)
	if connCount >= MaxWebSocketConnections {
		s.clientsMu.Unlock()
		log.Warn("Connection limit reached",
			slog.Int("current", connCount),
			slog.Int("max", MaxWebSocketConnections))
		http.Error(w, "Connection limit reached. Please try again later.", http.StatusServiceUnavailable)
		return
	}
	// Reserve a slot with a placeholder to prevent TOCTOU race
	reserveID := generateID("client")
	s.clients[reserveID] = nil
	s.clientsMu.Unlock()

	// Perform WebSocket upgrade and session creation outside the lock
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.clientsMu.Lock()
		delete(s.clients, reserveID)
		s.clientsMu.Unlock()
		log.WithError(err).Warn("WebSocket upgrade failed")
		return
	}

	// Set read limit to 10MB to handle large AI responses
	conn.SetReadLimit(10 * 1024 * 1024)

	// ?workspace=<slug> scopes this connection.
	scope := strings.TrimSpace(r.URL.Query().Get("workspace"))

	// Resolve the conversation session this connection subscribes to. Preferred
	// form: ?workspace=<slug>&channel=<id> — the server builds the session key
	// (the SAME key pkg/message's run emits on, both through pkg/shared), so a
	// client never reconstructs the session string itself. Legacy form: an
	// explicit ?session=<key>. Falls back to a default.
	sessionID := r.URL.Query().Get("session")
	if channelID := r.URL.Query().Get("channel"); channelID != "" {
		sessionID = shared.NewChannelSessionID(scope, channelID)
	}
	if sessionID == "" {
		sessionID = "session_main"
	}
	if actor == "" {
		// An anonymous listener gets no conversation — not the one it named,
		// not the default one. Broadcast events only.
		sessionID = wsAnonymousSession
	}

	session, err := s.sessions.GetOrCreateSession(sessionID, "main")
	if err != nil {
		s.clientsMu.Lock()
		delete(s.clients, reserveID)
		s.clientsMu.Unlock()
		log.WithError(err).Warn("Failed to get/create session")
		conn.Close()
		return
	}

	// Create client and replace the placeholder
	client := &Client{
		ID:      reserveID,
		Conn:    conn,
		Scope:   scope,
		Actor:   actor,
		send:    make(chan []byte, 1024),
		Session: session,
	}

	s.clientsMu.Lock()
	s.clients[reserveID] = client
	s.clientsMu.Unlock()

	// Track connection for health metrics
	health.IncrementActiveConnections()

	// Register with broadcaster for event streaming
	s.broadcaster.AddSubscriber(client)
	if actor != "" {
		s.broadcaster.SubscribeToSession(reserveID, session.ID)
	}

	log.Debug("New WebSocket client connected",
		slog.String("client_id", reserveID),
		slog.String("session", session.ID))

	// Send welcome message
	welcome := Message{
		Type:      "connected",
		SessionID: session.ID,
		Data: map[string]interface{}{
			"client_id": reserveID,
			"message":   "Connected to Memdoor Gateway",
		},
	}
	client.sendMessage(welcome)

	// What the window already holds before the first turn: the system prompt
	// and tool schemas every request carries, and the answering model's
	// window — so /context shows a real number instead of "0 / 200.0k", where
	// the 200K was the compaction ceiling, not the window. A no-op when no
	// turn has run yet and the baseline is not measured; the TUI then says
	// "no report yet" rather than inventing a window.
	if actor != "" && s.agent != nil {
		s.agent.EmitContextBaseline(session.ID)
	}

	// Start read/write goroutines
	go client.readPump(s)
	go client.writePump()
}

// readPump handles reading messages from the WebSocket
func (c *Client) readPump(s *Server) {
	log := logs.New("WebSocket")

	defer func() {
		s.removeClient(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	for {
		msgType, message, err := c.Conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log protocol errors with hex dump to diagnose corruption
				errMsg := err.Error()
				if strings.Contains(errMsg, "protocol error") || strings.Contains(errMsg, "RSV") || strings.Contains(errMsg, "bad opcode") {
					// Try to read a few raw bytes to see what's in the buffer (for debugging)
					log.WithError(err).Warn("WebSocket protocol error detected",
						slog.String("client_id", c.ID),
						slog.String("session", c.Session.ID),
						slog.String("error_type", "protocol_error"),
						slog.String("error_detail", errMsg))
				} else {
					log.WithError(err).Warn("WebSocket error",
						slog.String("client_id", c.ID),
						slog.String("session", c.Session.ID),
						slog.String("error_type", "unexpected_close"))
				}
			} else {
				// Log expected close errors at debug level
				log.Debug("WebSocket closed",
					slog.String("client_id", c.ID),
					slog.String("session", c.Session.ID),
					slog.String("error", err.Error()))
			}
			break
		}

		// Log frame type to help diagnose corruption issues (abbreviated)
		preview := string(message)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		log.Debug("Received WebSocket frame",
			slog.String("client_id", c.ID),
			slog.Int("frame_type", msgType),
			slog.Int("size", len(message)),
			slog.String("preview", preview))

		// Parse message
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.WithError(err).Warn("Failed to parse message")
			continue
		}

		// Handle message
		s.handleMessage(c, &msg)
	}
}

// writePump handles writing messages to the WebSocket
func (c *Client) writePump() {
	log := logs.New("WebSocket")
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
		log.Debug("WritePump exiting",
			slog.String("client_id", c.ID),
			slog.String("session", c.Session.ID))
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.writeMu.Lock()
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				log.Debug("Send channel closed, sending close message",
					slog.String("client_id", c.ID))
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				c.writeMu.Unlock()
				return
			}

			// Enable compression for large messages (>8KB) using Gorilla's per-message compression
			// Pattern: OpenClaw per-message compression control (RFC 7692)
			const compressionThreshold = 8 * 1024 // 8KB
			if len(message) > compressionThreshold {
				c.Conn.EnableWriteCompression(true)
				log.Debug("Sending compressed message",
					slog.String("client_id", c.ID),
					slog.Int("size", len(message)))
			} else {
				c.Conn.EnableWriteCompression(false)
				log.Debug("Sending message",
					slog.String("client_id", c.ID),
					slog.Int("size", len(message)))
			}

			err := c.Conn.WriteMessage(websocket.TextMessage, message)
			c.writeMu.Unlock()
			if err != nil {
				log.WithError(err).Debug("Write error in writePump",
					slog.String("client_id", c.ID))
				return
			}

		case <-ticker.C:
			c.writeMu.Lock()
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			c.Conn.EnableWriteCompression(false) // Never compress control frames (ping/pong/close)
			log.Debug("Sending ping",
				slog.String("client_id", c.ID))
			err := c.Conn.WriteMessage(websocket.PingMessage, nil)
			c.writeMu.Unlock()
			if err != nil {
				log.WithError(err).Debug("Ping failed",
					slog.String("client_id", c.ID))
				return
			}
		}
	}
}

// sendMessage sends a message to the client
// Thread-safe: Can be called by multiple goroutines concurrently
// Uses RLock to allow concurrent sends from multiple sources
// IMPORTANT: Lock is held during channel send to prevent send-on-closed-channel race
func (c *Client) sendMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client closed")
	}

	// Keep lock held during send - prevents close() from running between check and send
	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("client send channel full")
	}
}

// handleMessage handles incoming messages
func (s *Server) handleMessage(c *Client, msg *Message) {
	log := logs.New("WebSocket")

	// Rate limiting: Check if client has exceeded rate limit
	// Ping messages are exempt from rate limiting
	if msg.Type != "ping" {
		if !s.rateLimiter.Allow(c.ID) {
			log.Warn("Rate limit exceeded",
				slog.String("client_id", c.ID),
				slog.String("message_type", msg.Type))
			c.sendMessage(Message{
				Type:  "error",
				Error: "Rate limit exceeded. Please wait before sending more requests.",
			})
			return
		}
	}

	// An anonymous listener may keep the connection alive and nothing else:
	// every other type reaches the queue, a running turn, or the coder.
	if c.Actor == "" && msg.Type != "ping" {
		log.Warn("Rejected message from anonymous client",
			slog.String("client_id", c.ID),
			slog.String("message_type", msg.Type))
		c.sendMessage(Message{Type: "error", Error: "authentication required"})
		return
	}

	switch msg.Type {
	case "ping":
		c.sendMessage(Message{Type: "pong"})

	case "agent":
		s.handleAgentRPC(c, msg)

	case "agent.wait":
		s.handleAgentWaitRPC(c, msg)

	case "cancel":
		// Client (Esc) asks to interrupt this session's running turn.
		if s.cancelRun(c.Session.ID) {
			log.Debug("Cancelled running turn", slog.String("session", c.Session.ID))
		}
		c.sendMessage(Message{Type: "cancelled", SessionID: c.Session.ID})

	case "answer":
		// Client's answer to an interactive ask_user_question — deliver it to the
		// blocked tool call so the agent run continues.
		qid, _ := msg.Data["question_id"].(string)
		ans, _ := msg.Data["answer"].(string)
		if qid != "" && s.agent.AnswerQuestion(qid, ans) {
			log.Debug("Delivered question answer", slog.String("question_id", qid))
		}

	case "chat":
		// Route to AI agent via queue system
		text, ok := msg.Data["text"].(string)
		if !ok {
			c.sendMessage(Message{
				Type:  "error",
				Error: "Missing 'text' field in message",
			})
			return
		}

		log.Debug("Chat message from client",
			slog.String("client_id", c.ID),
			slog.String("text", text))

		// Create response writer callback to send response back to this client
		responseWriter := func(response interface{}) error {
			// Send the agent response back to the WebSocket client
			if responseMap, ok := response.(map[string]interface{}); ok {
				return c.sendMessage(Message{
					Type:      "chat_response",
					SessionID: c.Session.ID,
					Data:      responseMap,
				})
			}
			return fmt.Errorf("invalid response type")
		}

		// Scope the run to the connection's workspace, so a tool that needs one
		// has it (without it they fail with "no workspace available"). An empty
		// slug falls back to the install's single workspace. The SandboxContext
		// rides job.Context and is propagated to the run by executeAgentJob.
		wsSlug := strings.TrimSpace(c.Scope)
		if wsSlug == "" {
			wsSlug = s.defaultWorkspaceSlug()
		}
		jobCtx := context.WithValue(context.Background(), sharedctx.SandboxContextKey,
			sandbox.SandboxContext{WorkspaceSlug: wsSlug})

		// Claude-style interaction mode for the turn (plan/acceptEdits/default),
		// sent by the TUI. Rides job.Context to the run so executeTool gates
		// mutating tools in plan mode (read-only).
		if pm, _ := msg.Data["permission_mode"].(string); pm != "" {
			jobCtx = context.WithValue(jobCtx, sharedctx.PermissionModeKey, pm)
		}

		// The client's launch directory (Claude-CLI semantics): the coder's
		// confinement roots at the project the TUI was started from. Absolute,
		// existing directories only — anything else falls back to the default.
		if wd, _ := msg.Data["workdir"].(string); wd != "" && filepath.IsAbs(wd) {
			if fi, err := os.Stat(wd); err == nil && fi.IsDir() {
				jobCtx = context.WithValue(jobCtx, sharedctx.WorkdirKey, wd)
			}
		}

		// A TUI chat turn runs the coding agent (the "coder" buddy) by default:
		// its palette + system prompt, gated by the permission mode above. The
		// mode changes permission, not which agent runs. Without this the WS
		// path would run the bare default profile (no coding tools, no
		// plan-mode instructions).
		//
		// A LEADING @name routes the turn to that agent instead: "@planner
		// outline this" must not run the CODER, which lacks that agent's tools
		// and prompt. The mention is what every user reaches for; honor it.
		// Unknown names fall through to the coder rather than failing a turn.
		agentName := "coder"
		if name, rest, ok := leadingMention(text); ok {
			if b, err := s.repoFactory.Buddies().GetByName(context.Background(), name); err == nil && b != nil {
				agentName, text = name, rest
			}
		}
		extraPrompt := ""
		if buddy, err := s.repoFactory.Buddies().GetByName(context.Background(), agentName); err == nil && buddy != nil {
			if len(buddy.Tools) > 0 {
				jobCtx = context.WithValue(jobCtx, sharedctx.BuddyToolsKey, append([]string(nil), buddy.Tools...))
			}
			jobCtx = context.WithValue(jobCtx, sharedctx.IsBuddyChatKey, true)
			if buddy.SystemPrompt != nil {
				extraPrompt = *buddy.SystemPrompt
			}
		}

		// Enqueue job for execution
		// Pattern: OpenClaw's enqueueCommandInLane()
		job := &queue.AgentJob{
			SessionKey:        c.Session.ID,
			Message:           text,
			GlobalLane:        queue.LaneMain,
			EnqueueTime:       time.Now(),
			Context:           jobCtx,
			ResponseWriter:    responseWriter,
			ExtraSystemPrompt: extraPrompt,
		}

		err := s.queueManager.EnqueueJob(job)
		if err != nil {
			log.WithError(err).Warn("Failed to enqueue job")
			c.sendMessage(Message{
				Type:  "error",
				Error: fmt.Sprintf("Failed to enqueue job: %s", err.Error()),
			})
			return
		}

		// Send acknowledgment that job was queued
		c.sendMessage(Message{
			Type:      "chat_response",
			SessionID: c.Session.ID,
			Data: map[string]interface{}{
				"status":  "queued",
				"message": "Your request has been queued for processing",
			},
		})

	default:
		c.sendMessage(Message{
			Type:  "error",
			Error: fmt.Sprintf("Unknown message type: %s", msg.Type),
		})
	}
}

// removeClient removes a client from the server
// Thread-safe: Uses encapsulated client.close() method
func (s *Server) removeClient(c *Client) {
	log := logs.New("WebSocket")

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if _, ok := s.clients[c.ID]; ok {
		delete(s.clients, c.ID)
		c.close() // Use encapsulated close method - thread-safe

		// Track disconnection for health metrics
		health.DecrementActiveConnections()

		// Unregister from broadcaster
		s.broadcaster.RemoveSubscriber(c.ID)

		// Remove client's rate limiter to free memory
		s.rateLimiter.Remove(c.ID)

		log.Debug("Client disconnected", slog.String("client_id", c.ID))
	}
}

// defaultWorkspaceSlug returns the install's primary workspace slug (first by
// rowid), used when a WS connection didn't name a workspace. The
// single-workspace local norm hits this; multi-workspace clients pass ?workspace=.
func (s *Server) defaultWorkspaceSlug() string {
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok || db == nil {
		return ""
	}
	var slug string
	_ = db.QueryRow(`SELECT slug FROM workspaces ORDER BY rowid LIMIT 1`).Scan(&slug)
	return slug
}

// leadingMention splits a turn that opens with "@name " into the agent name
// and the remaining text. Only a LEADING mention routes: "@planner outline
// this" picks the planner, while "email me @ 5pm" or a mention further in the
// sentence is ordinary text. Returns ok=false when there is no leading
// mention. The caller decides whether the name is a real agent.
func leadingMention(text string) (name, rest string, ok bool) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "@") || len(t) < 2 {
		return "", "", false
	}
	i := strings.IndexFunc(t[1:], func(r rune) bool {
		return !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	if i <= 0 { // "@" alone, or a mention with no name
		return "", "", false
	}
	return strings.ToLower(t[1 : i+1]), strings.TrimSpace(t[i+1:]), true
}
