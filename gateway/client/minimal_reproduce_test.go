package client

import (
	"memdoor/gateway/protocol"
	"net"
	"testing"
	"time"
)

// TestMinimal_SendMultipleMessages tests sending multiple messages like TUI does
// This should reproduce the RSV2/RSV3/opcode 7 error
func TestMinimal_SendMultipleMessages(t *testing.T) {
	// Same guard as the rest of this package: these send real prompts, and a
	// model costs the person's key. See skipIfNoGateway.
	skipIfNoGateway(t)
	conn, err := net.DialTimeout("tcp", "localhost:18789", 200*time.Millisecond)
	if err != nil {
		t.Skip("Skipping integration test: gateway not reachable on localhost:18789")
	}
	conn.Close()

	gatewayURL := "http://localhost:18789"
	sessionKey := "test-minimal-reproduce"

	// Create client with TUI-like settings
	opts := DefaultOptions()
	opts.EnableLogging = true
	opts.CompressionThreshold = 8 * 1024

	client := New(gatewayURL, sessionKey, opts)

	// Connect
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Listen for messages
	messageCount := 0
	client.OnMessage(protocol.MessageTypeAgentEvent, func(msg protocol.Message) error {
		messageCount++
		t.Logf("Received message #%d: %s", messageCount, msg.Type)
		return nil
	})

	client.Listen()

	// Wait for connection to be fully established
	time.Sleep(500 * time.Millisecond)

	// Send first chat message (should work)
	t.Log("Sending first message...")
	if err := client.SendChat("test message 1", ""); err != nil {
		t.Fatalf("Failed to send first message: %v", err)
	}

	// Wait for response
	time.Sleep(5 * time.Second)

	// Send second chat message (this is where RSV2 error occurs in TUI)
	t.Log("Sending second message...")
	if err := client.SendChat("test message 2", ""); err != nil {
		t.Fatalf("Failed to send second message: %v", err)
	}

	// Wait for response
	time.Sleep(5 * time.Second)

	// Send third message
	t.Log("Sending third message...")
	if err := client.SendChat("test message 3", ""); err != nil {
		t.Fatalf("Failed to send third message: %v", err)
	}

	// Wait for final responses
	time.Sleep(5 * time.Second)

	t.Logf("Successfully sent 3 messages, received %d messages", messageCount)

	// Check if still connected
	if !client.IsConnected() {
		t.Error("Client disconnected unexpectedly")
	}
}
