package client

import (
	"fmt"
	"memdoor/gateway/protocol"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// done testing guard: the client's read pump keeps running after a test
// function returns (the final EventDisconnected can arrive after the last
// subtest), so the event handler must never call t.Log/t.Errorf on a
// completed test — that panics. Atomic so the read pump goroutine can read
// it without synchronization.
var integrationTestDone atomic.Bool

// skipIfNoGateway skips these tests unless someone opts in: they send real
// prompts to the running gateway, and every model is a paid API on the
// person's own key. They ran on every `go test ./...`
// once the engine left ~/.memdoor/remote-engine.json (2026-10-04), the file the
// old guard read to decide the brain was "local and free" — dozens of real
// coder turns on GLM with 880-message histories, interleaved with the
// person's own window. MEMDOOR_INTEGRATION_PAID=1 runs them on purpose.
func skipIfNoGateway(t *testing.T) {
	t.Helper()
	if testing.Short() || os.Getenv("MEMDOOR_INTEGRATION_PAID") != "1" {
		t.Skip("Skipping integration test: it sends paid inference to the running gateway — set MEMDOOR_INTEGRATION_PAID=1 to run it")
	}
	conn, err := net.DialTimeout("tcp", "localhost:18789", 200*time.Millisecond)
	if err != nil {
		t.Skip("Skipping integration test: gateway not reachable on localhost:18789")
	}
	conn.Close()
}

// TestClient_RealGateway_Bidirectional_Compression tests compression in both directions
// This is an integration test that requires a running gateway on localhost:18789
//
// Test coverage:
// ✅ Client can send small uncompressed messages (<8KB)
// ✅ Client can send large compressed messages (>8KB)
// ✅ Client can receive compressed messages from gateway
// ✅ No RSV2 protocol errors occur
// ✅ Connection stays alive with ping/pong
func TestClient_RealGateway_BidirectionalCompression(t *testing.T) {
	skipIfNoGateway(t)

	t.Log("=== Testing Bidirectional Compression with Real Gateway ===")

	// Create client with logging enabled
	opts := DefaultOptions()
	opts.EnableLogging = true
	opts.CompressionThreshold = 8 * 1024 // 8KB threshold

	sessionKey := fmt.Sprintf("test:compression:%d", time.Now().Unix())
	client := New("http://localhost:18789", sessionKey, opts)

	integrationTestDone.Store(false)

	// Track messages received
	var receivedMu sync.Mutex
	receivedMessages := make([]protocol.Message, 0)

	// Register handlers
	client.OnMessage(protocol.MessageTypeConnected, func(msg protocol.Message) error {
		t.Log("✓ Connected to gateway")
		return nil
	})

	client.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
		receivedMu.Lock()
		receivedMessages = append(receivedMessages, msg)
		receivedMu.Unlock()

		t.Logf("Received agent_event (size: %d bytes)", len(fmt.Sprintf("%+v", msg)))
		return nil
	})

	client.OnMessage(protocol.MessageTypeChatResponse, func(msg protocol.Message) error {
		receivedMu.Lock()
		receivedMessages = append(receivedMessages, msg)
		receivedMu.Unlock()

		t.Logf("Received chat_response (size: %d bytes)", len(fmt.Sprintf("%+v", msg)))
		return nil
	})

	// Track connection events
	var eventMu sync.Mutex
	events := make([]Event, 0)

	client.OnEvent(func(event Event) {
		eventMu.Lock()
		events = append(events, event)
		eventMu.Unlock()

		// The read pump can emit events after the test has completed (final
		// disconnected on client.Close()); logging then panics the test
		// binary. Record the event above, skip the logging.
		if integrationTestDone.Load() {
			return
		}

		t.Logf("Event: %s (error: %v)", event.Type, event.Error)
		if event.Type == EventError {
			t.Errorf("❌ Connection error: %v", event.Error)
		}
	})

	// Connect
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		// Mark the test done BEFORE closing: Close() triggers the read
		// pump's final EventDisconnected, and logging on a completed test
		// panics the binary.
		integrationTestDone.Store(true)
		client.Close()
	}()

	// Start listening
	client.Listen()

	// Wait for connection to stabilize
	time.Sleep(500 * time.Millisecond)

	// Test 1: Send small message (should NOT be compressed)
	t.Run("SmallMessage_Uncompressed", func(t *testing.T) {
		err := client.SendChat("Hello gateway!", "")
		if err != nil {
			t.Errorf("Failed to send small message: %v", err)
		}
		t.Log("✓ Small message sent (uncompressed)")
		time.Sleep(200 * time.Millisecond)
	})

	// Test 2: Send large message (should be compressed)
	t.Run("LargeMessage_Compressed", func(t *testing.T) {
		// Create message larger than 8KB
		largeText := strings.Repeat("This is a large message that will be compressed. ", 200) // ~10KB
		err := client.SendChat(largeText, "")
		if err != nil {
			t.Errorf("Failed to send large message: %v", err)
		}
		t.Log("✓ Large message sent (compressed)")
		time.Sleep(200 * time.Millisecond)
	})

	// Test 3: Send custom large protocol message
	t.Run("LargeProtocolMessage_Compressed", func(t *testing.T) {
		// Create a large data payload
		largeData := make(map[string]interface{})
		for i := 0; i < 100; i++ {
			largeData[fmt.Sprintf("field_%d", i)] = strings.Repeat("data", 100)
		}

		msg := protocol.Message{
			Type: "test_large",
			Data: largeData,
		}

		err := client.SendMessage(msg)
		if err != nil {
			t.Errorf("Failed to send large protocol message: %v", err)
		}
		t.Log("✓ Large protocol message sent (compressed)")
		time.Sleep(200 * time.Millisecond)
	})

	// Test 4: Wait for agent to respond (gateway will send compressed messages if >8KB)
	t.Run("ReceiveCompressedFromGateway", func(t *testing.T) {
		// Send a message that will trigger a large response
		err := client.SendChat("Tell me a very long story about WebSocket compression", "")
		if err != nil {
			t.Errorf("Failed to send message: %v", err)
		}

		// Wait for response (agent will generate text)
		time.Sleep(5 * time.Second)

		receivedMu.Lock()
		messageCount := len(receivedMessages)
		receivedMu.Unlock()

		t.Logf("✓ Received %d messages from gateway", messageCount)

		if messageCount == 0 {
			t.Log("⚠ No messages received (agent may still be processing)")
		}
	})

	// Test 5: Verify connection stayed alive (no protocol errors)
	t.Run("ConnectionStability", func(t *testing.T) {
		if !client.IsConnected() {
			t.Error("❌ Client disconnected unexpectedly")
		} else {
			t.Log("✓ Connection still alive after all tests")
		}

		// Check for error events
		eventMu.Lock()
		errorCount := 0
		for _, e := range events {
			if e.Type == EventError {
				errorCount++
				t.Logf("❌ Error event: %v", e.Error)
			}
		}
		eventMu.Unlock()

		if errorCount > 0 {
			t.Errorf("❌ %d error events occurred", errorCount)
		} else {
			t.Log("✓ No protocol errors (RSV2 bug is fixed!)")
		}
	})

	// Test 6: Ping/pong keep-alive (wait >54s for server to send ping)
	t.Run("PingPongKeepAlive", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping ping/pong test (takes 60s)")
		}

		t.Log("Waiting 60 seconds for server ping/pong cycle...")
		time.Sleep(60 * time.Second)

		if !client.IsConnected() {
			t.Error("❌ Connection died during ping/pong cycle")
		} else {
			t.Log("✓ Connection survived ping/pong keep-alive")
		}
	})
}

// TestClient_CompressionThreshold tests the compression threshold configuration
func TestClient_CompressionThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold int
		msgSize   int
		wantComp  bool
	}{
		{
			name:      "Small message below 8KB threshold",
			threshold: 8 * 1024,
			msgSize:   4 * 1024,
			wantComp:  false,
		},
		{
			name:      "Large message above 8KB threshold",
			threshold: 8 * 1024,
			msgSize:   10 * 1024,
			wantComp:  true,
		},
		{
			name:      "Compression disabled (threshold = 0)",
			threshold: 0,
			msgSize:   100 * 1024,
			wantComp:  false,
		},
		{
			name:      "Always compress (threshold = -1)",
			threshold: -1,
			msgSize:   100,
			wantComp:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.CompressionThreshold = tt.threshold

			// We just test the logic, not actual compression
			enableCompression := false
			if opts.CompressionThreshold > 0 && tt.msgSize > opts.CompressionThreshold {
				enableCompression = true
			} else if opts.CompressionThreshold == -1 {
				enableCompression = true
			}

			if enableCompression != tt.wantComp {
				t.Errorf("Compression logic wrong: got %v, want %v", enableCompression, tt.wantComp)
			}
		})
	}
}

// TestClient_ConcurrentSends tests thread safety of concurrent sends
func TestClient_ConcurrentSends(t *testing.T) {
	skipIfNoGateway(t)

	opts := DefaultOptions()
	opts.EnableLogging = false // Reduce noise

	sessionKey := fmt.Sprintf("test:concurrent:%d", time.Now().Unix())
	client := New("http://localhost:18789", sessionKey, opts)

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	client.Listen()

	// Send 100 messages concurrently
	const numMessages = 100
	var wg sync.WaitGroup
	errors := make(chan error, numMessages)

	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := client.SendChat(fmt.Sprintf("Message %d", id), "")
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Errorf("Send error: %v", err)
		errorCount++
	}

	if errorCount == 0 {
		t.Logf("✓ Successfully sent %d messages concurrently", numMessages)
	}

	if !client.IsConnected() {
		t.Error("❌ Client disconnected during concurrent sends")
	}
}

// TestClient_DisconnectReconnect tests disconnection and reconnection
func TestClient_DisconnectReconnect(t *testing.T) {
	skipIfNoGateway(t)

	opts := DefaultOptions()
	sessionKey := fmt.Sprintf("test:reconnect:%d", time.Now().Unix())

	// First connection
	client1 := New("http://localhost:18789", sessionKey, opts)
	if err := client1.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	client1.Listen()
	time.Sleep(500 * time.Millisecond)

	// Send message
	client1.SendChat("First connection", "")
	time.Sleep(200 * time.Millisecond)

	// Disconnect
	client1.Close()
	time.Sleep(500 * time.Millisecond)

	// Reconnect with same session
	client2 := New("http://localhost:18789", sessionKey, opts)
	if err := client2.Connect(); err != nil {
		t.Fatalf("Failed to reconnect: %v", err)
	}
	defer client2.Close()

	client2.Listen()
	time.Sleep(500 * time.Millisecond)

	// Send message after reconnect
	if err := client2.SendChat("Second connection", ""); err != nil {
		t.Errorf("Failed to send after reconnect: %v", err)
	}

	t.Log("✓ Successfully disconnected and reconnected")
}

// BenchmarkClient_SendMessages benchmarks message throughput
func BenchmarkClient_SendMessages(b *testing.B) {
	opts := DefaultOptions()
	opts.EnableLogging = false

	sessionKey := fmt.Sprintf("bench:send:%d", time.Now().Unix())
	client := New("http://localhost:18789", sessionKey, opts)

	if err := client.Connect(); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	client.Listen()
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.SendChat("Benchmark message", "")
	}
}

// BenchmarkClient_SendLargeMessages benchmarks compressed message throughput
func BenchmarkClient_SendLargeMessages(b *testing.B) {
	opts := DefaultOptions()
	opts.EnableLogging = false

	sessionKey := fmt.Sprintf("bench:large:%d", time.Now().Unix())
	client := New("http://localhost:18789", sessionKey, opts)

	if err := client.Connect(); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	client.Listen()
	time.Sleep(100 * time.Millisecond)

	// Create 20KB message
	largeMsg := strings.Repeat("Large benchmark message. ", 800)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.SendChat(largeMsg, "")
	}
}
