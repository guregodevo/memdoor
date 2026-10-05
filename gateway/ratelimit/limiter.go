package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a token bucket rate limiter
// Pattern: Per-client rate limiting to prevent API abuse
type Limiter struct {
	rate       int           // tokens per interval
	interval   time.Duration // interval duration
	tokens     int           // current tokens
	lastRefill time.Time     // last refill time
	mu         sync.Mutex    // protects tokens and lastRefill
}

// NewLimiter creates a new rate limiter
// rate: number of requests allowed per interval
// interval: time window for rate limiting (e.g., 1 minute)
func NewLimiter(rate int, interval time.Duration) *Limiter {
	return &Limiter{
		rate:       rate,
		interval:   interval,
		tokens:     rate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed
// Returns true if request is allowed, false if rate limit exceeded
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(l.lastRefill)

	if elapsed >= l.interval {
		// Full refill
		l.tokens = l.rate
		l.lastRefill = now
	}

	// Check if we have tokens available
	if l.tokens > 0 {
		l.tokens--
		return true
	}

	return false
}

// Reset resets the limiter to full capacity
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.tokens = l.rate
	l.lastRefill = time.Now()
}

// TokensRemaining returns the number of tokens remaining
func (l *Limiter) TokensRemaining() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.tokens
}

// PerClientLimiter manages rate limiters for multiple clients
type PerClientLimiter struct {
	limiters map[string]*Limiter
	rate     int
	interval time.Duration
	mu       sync.RWMutex
}

// NewPerClientLimiter creates a new per-client rate limiter
func NewPerClientLimiter(rate int, interval time.Duration) *PerClientLimiter {
	return &PerClientLimiter{
		limiters: make(map[string]*Limiter),
		rate:     rate,
		interval: interval,
	}
}

// Allow checks if a request from the given client should be allowed
func (p *PerClientLimiter) Allow(clientID string) bool {
	p.mu.RLock()
	limiter, exists := p.limiters[clientID]
	p.mu.RUnlock()

	if !exists {
		p.mu.Lock()
		// Double-check after acquiring write lock
		limiter, exists = p.limiters[clientID]
		if !exists {
			limiter = NewLimiter(p.rate, p.interval)
			p.limiters[clientID] = limiter
		}
		p.mu.Unlock()
	}

	return limiter.Allow()
}

// Remove removes a client's rate limiter (cleanup on disconnect)
func (p *PerClientLimiter) Remove(clientID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.limiters, clientID)
}

// Reset resets a client's rate limiter
func (p *PerClientLimiter) Reset(clientID string) {
	p.mu.RLock()
	limiter, exists := p.limiters[clientID]
	p.mu.RUnlock()

	if exists {
		limiter.Reset()
	}
}

// Count returns the number of active client limiters
func (p *PerClientLimiter) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.limiters)
}
