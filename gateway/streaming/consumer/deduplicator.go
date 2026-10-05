package consumer

import (
	"sync"
	"time"
)

// Deduplicator tracks finalized runs and prevents redundant processing
// Pattern: OpenClaw's multi-level deduplication (finalized runs, session runs, change detection)
type Deduplicator struct {
	finalizedRuns map[string]time.Time // runID → finalized timestamp
	sessionRuns   map[string][]string  // sessionID → runIDs
	mu            sync.RWMutex
	ttl           time.Duration // How long to remember finalized runs
	verbose       bool
}

// NewDeduplicator creates a new deduplicator
func NewDeduplicator(ttl time.Duration, verbose bool) *Deduplicator {
	if ttl <= 0 {
		ttl = 10 * time.Minute // Default: 10 minutes (OpenClaw uses 10 min)
	}

	return &Deduplicator{
		finalizedRuns: make(map[string]time.Time),
		sessionRuns:   make(map[string][]string),
		ttl:           ttl,
		verbose:       verbose,
	}
}

// IsFinalized checks if a run has already been finalized
func (d *Deduplicator) IsFinalized(runID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, exists := d.finalizedRuns[runID]
	return exists
}

// MarkFinalized marks a run as finalized
func (d *Deduplicator) MarkFinalized(runID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.finalizedRuns[runID] = time.Now()
}

// AddSessionRun associates a run with a session
func (d *Deduplicator) AddSessionRun(sessionID, runID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	runs, exists := d.sessionRuns[sessionID]
	if !exists {
		runs = []string{}
	}

	// Check if runID already in list
	for _, existingRunID := range runs {
		if existingRunID == runID {
			return // Already exists
		}
	}

	runs = append(runs, runID)
	d.sessionRuns[sessionID] = runs
}

// GetSessionRuns returns all run IDs for a session
func (d *Deduplicator) GetSessionRuns(sessionID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	runs, exists := d.sessionRuns[sessionID]
	if !exists {
		return []string{}
	}

	// Return a copy to prevent concurrent modification
	runsCopy := make([]string, len(runs))
	copy(runsCopy, runs)
	return runsCopy
}

// ClearSessionRuns removes all runs for a session
func (d *Deduplicator) ClearSessionRuns(sessionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.sessionRuns, sessionID)
}

// Prune removes old finalized runs and cleans up session mappings
// Returns number of entries pruned
func (d *Deduplicator) Prune() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	pruned := 0

	// Prune old finalized runs
	for runID, finalizedTime := range d.finalizedRuns {
		if now.Sub(finalizedTime) > d.ttl {
			delete(d.finalizedRuns, runID)
			pruned++

			// Also remove from session mappings
			d.removeRunFromSessions(runID)
		}
	}

	return pruned
}

// removeRunFromSessions removes a run ID from all session mappings
// NOTE: Must be called with lock held
func (d *Deduplicator) removeRunFromSessions(runID string) {
	for sessionID, runs := range d.sessionRuns {
		filtered := []string{}
		for _, r := range runs {
			if r != runID {
				filtered = append(filtered, r)
			}
		}

		if len(filtered) == 0 {
			delete(d.sessionRuns, sessionID)
		} else {
			d.sessionRuns[sessionID] = filtered
		}
	}
}

// GetStats returns statistics about deduplicator state
func (d *Deduplicator) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	totalSessionRuns := 0
	for _, runs := range d.sessionRuns {
		totalSessionRuns += len(runs)
	}

	return map[string]interface{}{
		"finalized_runs":     len(d.finalizedRuns),
		"active_sessions":    len(d.sessionRuns),
		"total_session_runs": totalSessionRuns,
		"ttl_minutes":        d.ttl.Minutes(),
	}
}

// IsRunInSession checks if a run belongs to a specific session
func (d *Deduplicator) IsRunInSession(sessionID, runID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	runs, exists := d.sessionRuns[sessionID]
	if !exists {
		return false
	}

	for _, r := range runs {
		if r == runID {
			return true
		}
	}

	return false
}

// ClearAllSessions removes all session mappings
func (d *Deduplicator) ClearAllSessions() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.sessionRuns = make(map[string][]string)
}

// ClearAllFinalized removes all finalized run records
func (d *Deduplicator) ClearAllFinalized() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.finalizedRuns = make(map[string]time.Time)
}
