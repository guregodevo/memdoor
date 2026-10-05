package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocket_ConcurrentCloseRace tests the race condition where both
// readPump and writePump call Close() concurrently
func TestWebSocket_ConcurrentCloseRace(t *testing.T) {
	// Create a minimal server
	s := &Server{
		clients:   make(map[string]*Client),
		clientsMu: sync.RWMutex{},
		upgrader: websocket.Upgrader{
			ReadBufferSize:    1024,
			WriteBufferSize:   1024,
			EnableCompression: false,
			CheckOrigin:       func(r *http.Request) bool { return true },
		},
	}

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(s.handleWebSocket))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Run multiple iterations to increase chance of hitting the race
	for i := 0; i < 100; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Dial failed: %v", err)
		}

		// Immediately close to trigger concurrent close in both pumps
		time.Sleep(1 * time.Millisecond) // Let pumps start
		conn.Close()

		// Brief pause between iterations
		time.Sleep(5 * time.Millisecond)
	}

	// Give time for all cleanups
	time.Sleep(100 * time.Millisecond)

	// Verify no clients remain
	s.clientsMu.RLock()
	remaining := len(s.clients)
	s.clientsMu.RUnlock()

	if remaining != 0 {
		t.Errorf("Expected 0 clients after cleanup, got %d", remaining)
	}
}

// TestWebSocket_SendOnClosedChannelRace tests the race where broadcaster
// sends to a client's channel while removeClient is closing it
func TestWebSocket_SendOnClosedChannelRace(t *testing.T) {
	// This test requires a full server setup with broadcaster
	// Skipping for now - would need integration test setup
	t.Skip("Requires full server integration test setup")

	// The race scenario:
	// 1. Client connects, gets added to broadcaster
	// 2. Connection error occurs, readPump calls removeClient
	// 3. removeClient closes c.send channel
	// 4. Broadcaster goroutine tries to send event → PANIC
}

// TestWebSocket_HighThroughputStressTest sends rapid messages to stress test
// the write pump and detect any concurrency issues
func TestWebSocket_HighThroughputStressTest(t *testing.T) {
	t.Skip("Requires full server setup - use minimal tests instead")
}

// TestWebSocket_DisconnectDuringBroadcast tests the specific race:
// Client disconnects while broadcaster is sending events
func TestWebSocket_DisconnectDuringBroadcast(t *testing.T) {
	t.Skip("Requires full server setup - use TestMinimal_SendOnClosedChannel instead")
}

// TestWebSocket_PongHandlerDeadlineRace tests concurrent SetReadDeadline calls
func TestWebSocket_PongHandlerDeadlineRace(t *testing.T) {
	// This test would need to:
	// 1. Set up client with pong handler
	// 2. Send pings rapidly to trigger pong handler
	// 3. Simultaneously have readPump reading messages
	// 4. Check for race conditions in SetReadDeadline calls

	t.Skip("Requires deeper integration - pong handler is internal to gorilla/websocket")

	// The race is:
	// - readPump: c.Conn.SetReadDeadline(...) at line 99 (initial)
	// - readPump: c.Conn.SetReadDeadline(...) in pong handler at line 101 (callback)
	// These could potentially race if pong arrives while setting initial deadline
}

// Note: Full integration tests with broadcaster require proper server setup
// Use minimal_race_reproducer_test.go for targeted race condition testing
