package health

import (
	"sync"
	"time"
)

// Cache manages health snapshot caching
type Cache struct {
	mu       sync.RWMutex
	snapshot *HealthSummary
	cachedAt time.Time
}

// NewCache creates a new health cache
func NewCache() *Cache {
	return &Cache{}
}

// Get retrieves cached snapshot if valid
func (c *Cache) Get(maxAge time.Duration) *HealthSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.snapshot == nil {
		return nil
	}

	if time.Since(c.cachedAt) > maxAge {
		return nil
	}

	// Mark as cached
	cached := *c.snapshot
	cached.Cached = true
	return &cached
}

// Set stores a new snapshot in cache
func (c *Cache) Set(snapshot *HealthSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.snapshot = snapshot
	c.cachedAt = time.Now()
}

// Clear removes cached snapshot
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.snapshot = nil
	c.cachedAt = time.Time{}
}

// Age returns the age of the cached snapshot
func (c *Cache) Age() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.snapshot == nil {
		return 0
	}

	return time.Since(c.cachedAt)
}
