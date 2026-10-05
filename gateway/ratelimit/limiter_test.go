package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	limiter := NewLimiter(3, 1*time.Second)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if limiter.Allow() {
		t.Error("4th request should be denied (rate limit exceeded)")
	}

	// Wait for refill
	time.Sleep(1100 * time.Millisecond)

	// Should allow requests again after refill
	if !limiter.Allow() {
		t.Error("Request after refill should be allowed")
	}
}

func TestLimiter_Reset(t *testing.T) {
	limiter := NewLimiter(2, 1*time.Second)

	// Exhaust tokens
	limiter.Allow()
	limiter.Allow()

	if limiter.Allow() {
		t.Error("Request should be denied (tokens exhausted)")
	}

	// Reset limiter
	limiter.Reset()

	// Should allow requests after reset
	if !limiter.Allow() {
		t.Error("Request after reset should be allowed")
	}
}

func TestLimiter_TokensRemaining(t *testing.T) {
	limiter := NewLimiter(5, 1*time.Second)

	if tokens := limiter.TokensRemaining(); tokens != 5 {
		t.Errorf("Initial tokens = %d, want 5", tokens)
	}

	limiter.Allow()
	limiter.Allow()

	if tokens := limiter.TokensRemaining(); tokens != 3 {
		t.Errorf("Remaining tokens = %d, want 3", tokens)
	}
}

func TestPerClientLimiter_Allow(t *testing.T) {
	limiter := NewPerClientLimiter(2, 1*time.Second)

	// Client 1: Allow 2 requests
	if !limiter.Allow("client1") {
		t.Error("Client1 request 1 should be allowed")
	}
	if !limiter.Allow("client1") {
		t.Error("Client1 request 2 should be allowed")
	}
	if limiter.Allow("client1") {
		t.Error("Client1 request 3 should be denied")
	}

	// Client 2: Should have separate limit
	if !limiter.Allow("client2") {
		t.Error("Client2 request 1 should be allowed (separate limit)")
	}
	if !limiter.Allow("client2") {
		t.Error("Client2 request 2 should be allowed")
	}
	if limiter.Allow("client2") {
		t.Error("Client2 request 3 should be denied")
	}
}

func TestPerClientLimiter_Remove(t *testing.T) {
	limiter := NewPerClientLimiter(2, 1*time.Second)

	limiter.Allow("client1")
	limiter.Allow("client2")

	if count := limiter.Count(); count != 2 {
		t.Errorf("Active limiters = %d, want 2", count)
	}

	limiter.Remove("client1")

	if count := limiter.Count(); count != 1 {
		t.Errorf("Active limiters after removal = %d, want 1", count)
	}
}

func TestPerClientLimiter_Concurrent(t *testing.T) {
	limiter := NewPerClientLimiter(10, 1*time.Second)

	var wg sync.WaitGroup
	var allowed, denied int
	var mu sync.Mutex

	// 10 concurrent clients, each making 15 requests
	for i := 0; i < 10; i++ {
		clientID := string(rune('A' + i))
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for j := 0; j < 15; j++ {
				if limiter.Allow(id) {
					mu.Lock()
					allowed++
					mu.Unlock()
				} else {
					mu.Lock()
					denied++
					mu.Unlock()
				}
			}
		}(clientID)
	}

	wg.Wait()

	// Each client has limit of 10, so:
	// - 10 clients * 10 allowed = 100 allowed
	// - 10 clients * 5 denied = 50 denied
	if allowed != 100 {
		t.Errorf("Allowed requests = %d, want 100", allowed)
	}
	if denied != 50 {
		t.Errorf("Denied requests = %d, want 50", denied)
	}

	if count := limiter.Count(); count != 10 {
		t.Errorf("Active limiters = %d, want 10", count)
	}
}

func TestPerClientLimiter_Reset(t *testing.T) {
	limiter := NewPerClientLimiter(2, 1*time.Second)

	// Exhaust client's tokens
	limiter.Allow("client1")
	limiter.Allow("client1")

	if limiter.Allow("client1") {
		t.Error("Request should be denied (tokens exhausted)")
	}

	// Reset client's limiter
	limiter.Reset("client1")

	// Should allow requests after reset
	if !limiter.Allow("client1") {
		t.Error("Request after reset should be allowed")
	}
}
