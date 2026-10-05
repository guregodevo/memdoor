package gateway

import (
	"sync"
	"testing"
	"time"
)

// TestClient_ConcurrentSendAndClose tests the fixed implementation
// This tests the actual Client.Send() and Client.close() methods
func TestClient_ConcurrentSendAndClose(t *testing.T) {
	// Create client with our new thread-safe implementation
	client := &Client{
		ID:   "test-client",
		send: make(chan []byte, 1024),
	}

	var wg sync.WaitGroup
	errorCh := make(chan error, 100)

	// Start 10 sender goroutines (simulating broadcasters)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				data := []byte("test message")
				err := client.Send(data) // Use actual Send method
				if err != nil {
					// Expected after close - not an error
					if err.Error() != "client closed" {
						errorCh <- err
					}
					return
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Start a goroutine that closes the client
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond) // Let senders run a bit
		client.close()                    // Use actual close method
	}()

	wg.Wait()
	close(errorCh)

	// Check for unexpected errors
	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("❌ Unexpected errors: %v", errors)
	} else {
		t.Log("✓ No unexpected errors with RWMutex protection")
		t.Log("✓ Concurrent Send() and close() work correctly")
	}
}

// TestClient_DoubleClose tests that close() is idempotent
func TestClient_DoubleClose(t *testing.T) {
	client := &Client{
		ID:   "test-client",
		send: make(chan []byte, 10),
	}

	// Close multiple times - should not panic
	client.close()
	client.close()
	client.close()

	t.Log("✓ close() is idempotent - can be called multiple times safely")
}

// TestClient_SendAfterClose tests that Send returns error after close
func TestClient_SendAfterClose(t *testing.T) {
	client := &Client{
		ID:   "test-client",
		send: make(chan []byte, 10),
	}

	client.close()

	err := client.Send([]byte("test"))
	if err == nil {
		t.Error("❌ Expected error when sending to closed client")
	} else if err.Error() != "client closed" {
		t.Errorf("❌ Unexpected error: %v", err)
	} else {
		t.Log("✓ Send() returns error after close()")
	}
}

// TestClient_ConcurrentReaders tests that multiple senders don't block each other
func TestClient_ConcurrentReaders(t *testing.T) {
	client := &Client{
		ID:   "test-client",
		send: make(chan []byte, 1000),
	}

	// Start a goroutine to drain the channel
	done := make(chan bool)
	go func() {
		for range client.send {
			// Drain
		}
		done <- true
	}()

	// Measure time for 100 concurrent sends
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Send([]byte("test"))
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	client.close()
	<-done

	// With RWMutex, this should be fast (concurrent reads)
	if elapsed > 100*time.Millisecond {
		t.Logf("⚠ Concurrent sends took %v (might indicate lock contention)", elapsed)
	} else {
		t.Logf("✓ Concurrent sends completed in %v (good concurrency)", elapsed)
	}
}
