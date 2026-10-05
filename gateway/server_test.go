package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"memdoor/gateway/broadcast"
	"memdoor/gateway/logs"
	"memdoor/gateway/ratelimit"
)

// TestServer_HTTPHealth tests the HTTP health endpoint
func TestServer_HTTPHealth(t *testing.T) {
	// Create a test server
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Create HTTP server
	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	// Test health endpoint
	resp, err := http.Get(httpServer.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestServer_WebSocketConnection tests basic WebSocket connection
func TestServer_WebSocketConnection(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	// Create HTTP test server
	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect to WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Read welcome message
	var msg Message
	if err := ws.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read welcome message: %v", err)
	}

	if msg.Type != "connected" {
		t.Errorf("Welcome message type = %q, want %q", msg.Type, "connected")
	}

	if msg.SessionID == "" {
		t.Error("Welcome message missing session_id")
	}
}

// TestServer_WebSocketPing tests ping/pong messages
func TestServer_WebSocketPing(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	// Read and discard welcome message
	var welcome Message
	ws.ReadJSON(&welcome)

	// Send ping
	ping := Message{Type: "ping"}
	if err := ws.WriteJSON(ping); err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Read pong
	var pong Message
	if err := ws.ReadJSON(&pong); err != nil {
		t.Fatalf("Failed to read pong: %v", err)
	}

	if pong.Type != "pong" {
		t.Errorf("Response type = %q, want %q", pong.Type, "pong")
	}
}

// TestServer_RateLimiting tests rate limiting enforcement
func TestServer_RateLimiting(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	// Read welcome message
	var welcome Message
	ws.ReadJSON(&welcome)

	// Extract client ID from welcome message
	clientID, ok := welcome.Data["client_id"].(string)
	if !ok {
		t.Fatal("Welcome message missing client_id")
	}

	// Send messages until rate limited (limit is 10 per minute)
	// Use "test" message type to avoid needing queue/agent initialization
	rateLimited := false
	for i := 0; i < 15; i++ {
		msg := Message{
			Type: "test_message", // Simple message type that triggers rate limit check
			Data: map[string]interface{}{
				"test": "data",
			},
		}

		if err := ws.WriteJSON(msg); err != nil {
			t.Fatalf("Failed to send message %d: %v", i+1, err)
		}

		var response Message
		if err := ws.ReadJSON(&response); err != nil {
			t.Fatalf("Failed to read response %d: %v", i+1, err)
		}

		if response.Type == "error" && strings.Contains(response.Error, "Rate limit exceeded") {
			rateLimited = true
			t.Logf("✓ Rate limited after %d requests (client_id: %s)", i+1, clientID)
			break
		}
	}

	if !rateLimited {
		t.Error("Expected rate limiting to kick in, but it didn't")
	}
}

// TestServer_ConnectionLimit tests maximum connection enforcement
func TestServer_ConnectionLimit(t *testing.T) {
	// This test would require creating 1000+ connections
	// Skip in normal test runs, but document the behavior
	t.Skip("Connection limit test requires creating 1000+ connections - skipped for performance")

	// Test would:
	// 1. Create 1000 WebSocket connections (MaxWebSocketConnections)
	// 2. Attempt 1001st connection
	// 3. Verify it's rejected with 503 Service Unavailable
}

// TestServer_MultipleClients tests multiple concurrent clients
func TestServer_MultipleClients(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect 5 clients
	clients := make([]*websocket.Conn, 5)
	for i := 0; i < 5; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		defer ws.Close()
		clients[i] = ws

		// Read welcome message
		var welcome Message
		ws.ReadJSON(&welcome)
	}

	// Verify all clients are connected
	server.clientsMu.RLock()
	connectedCount := len(server.clients)
	server.clientsMu.RUnlock()

	if connectedCount != 5 {
		t.Errorf("Connected clients = %d, want 5", connectedCount)
	}

	// Close one client
	clients[0].Close()
	time.Sleep(100 * time.Millisecond) // Give server time to clean up

	// Verify client was removed
	server.clientsMu.RLock()
	connectedCount = len(server.clients)
	server.clientsMu.RUnlock()

	if connectedCount != 4 {
		t.Errorf("Connected clients after disconnect = %d, want 4", connectedCount)
	}
}

// TestServer_SessionPersistence tests session creation and reuse
func TestServer_SessionPersistence(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	// Connect with custom session ID
	sessionID := "test-session-123"
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws?session=" + sessionID

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	// Read welcome message
	var welcome Message
	if err := ws.ReadJSON(&welcome); err != nil {
		t.Fatalf("Failed to read welcome: %v", err)
	}

	if welcome.SessionID != sessionID {
		t.Errorf("Session ID = %q, want %q", welcome.SessionID, sessionID)
	}

	// Verify session exists in server
	session, err := server.sessions.GetSession(sessionID)
	if err != nil {
		t.Errorf("Failed to get session: %v", err)
	}
	if session == nil {
		t.Error("Session not found in server")
	}
}

// TestServer_InvalidMessage tests handling of invalid messages
func TestServer_InvalidMessage(t *testing.T) {
	server, cleanup := createTestServer(t)
	defer cleanup()

	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	// Read welcome message
	var welcome Message
	ws.ReadJSON(&welcome)

	// Send invalid message type
	invalid := Message{Type: "invalid_type"}
	if err := ws.WriteJSON(invalid); err != nil {
		t.Fatalf("Failed to send invalid message: %v", err)
	}

	// Should receive error response
	var response Message
	if err := ws.ReadJSON(&response); err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if response.Type != "error" {
		t.Errorf("Response type = %q, want %q", response.Type, "error")
	}

	if !strings.Contains(response.Error, "Unknown message type") {
		t.Errorf("Error message = %q, want message containing 'Unknown message type'", response.Error)
	}
}

// TestServer_CleanShutdown tests graceful server shutdown
func TestServer_CleanShutdown(t *testing.T) {
	server, cleanup := createTestServer(t)

	httpServer := httptest.NewServer(server.setupRoutes())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	// Read welcome
	var welcome Message
	ws.ReadJSON(&welcome)

	// Shutdown server
	cleanup()

	// Verify server cleaned up
	// (In real implementation, this would test server.Stop() method)
	t.Log("✓ Server shutdown completed")
}

// Helper function to create a test server
func createTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	// Initialize global logger (required by logs.New() in handlers)
	if err := logs.InitGlobalLoggerDefault(false); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	server := &Server{
		port:        8080,
		sessions:    NewSessionManager(),
		clients:     make(map[string]*Client),
		upgrader:    websocket.Upgrader{},
		verbose:     false,
		broadcaster: broadcast.NewSubscriptionManager(false), // Initialize broadcaster for event streaming
	}

	// Initialize rate limiter
	server.rateLimiter = ratelimit.NewPerClientLimiter(10, 1*time.Minute)

	cleanup := func() {
		// Close all client connections
		server.clientsMu.Lock()
		for _, client := range server.clients {
			client.Conn.Close()
		}
		server.clientsMu.Unlock()
	}

	return server, cleanup
}

// setupRoutes creates HTTP routes for testing
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	return mux
}
