package presence

import (
	"sync"
	"time"

	"memdoor/gateway/broadcast"
	"memdoor/pkg/shared"
)

const (
	StatusOnline  = "online"
	StatusOffline = "offline"
)

// Broadcaster interface for broadcasting presence events
type Broadcaster interface {
	BroadcastToAll(event broadcast.Event)
}

// Tracker manages real-time presence (online/offline status) for actors.
// It tracks active WebSocket connections and broadcasts presence changes.
type Tracker struct {
	// actorID -> set of active client IDs
	actorConnections map[string]map[string]bool

	// actorID -> last seen timestamp
	lastSeen map[string]time.Time

	// Broadcaster for sending presence events
	broadcaster Broadcaster

	mu sync.RWMutex
}

// NewTracker creates a new presence tracker.
func NewTracker(broadcaster Broadcaster) *Tracker {
	return &Tracker{
		actorConnections: make(map[string]map[string]bool),
		lastSeen:         make(map[string]time.Time),
		broadcaster:      broadcaster,
	}
}

// Connect registers a new WebSocket connection for an actor.
// If this is the actor's first connection, broadcasts online status.
func (t *Tracker) Connect(actorID shared.ActorID, clientID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	actorKey := actorID.String()

	// Check if actor was offline before this connection
	wasOffline := len(t.actorConnections[actorKey]) == 0

	// Initialize connection set if needed
	if t.actorConnections[actorKey] == nil {
		t.actorConnections[actorKey] = make(map[string]bool)
	}

	// Add this client connection
	t.actorConnections[actorKey][clientID] = true

	// Update last seen
	t.lastSeen[actorKey] = time.Now()

	// Broadcast online event if actor was offline
	if wasOffline {
		t.broadcastPresence(actorID, StatusOnline)
	}
}

// Disconnect removes a WebSocket connection for an actor.
// If this was the actor's last connection, broadcasts offline status.
func (t *Tracker) Disconnect(actorID shared.ActorID, clientID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	actorKey := actorID.String()

	// Remove this client connection
	if t.actorConnections[actorKey] != nil {
		delete(t.actorConnections[actorKey], clientID)

		// Update last seen
		t.lastSeen[actorKey] = time.Now()

		// If no more connections, actor is offline
		if len(t.actorConnections[actorKey]) == 0 {
			delete(t.actorConnections, actorKey)
			t.broadcastPresence(actorID, StatusOffline)
		}
	}
}

// GetStatus returns the current status of an actor.
func (t *Tracker) GetStatus(actorID shared.ActorID) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	actorKey := actorID.String()
	if len(t.actorConnections[actorKey]) > 0 {
		return StatusOnline
	}
	return StatusOffline
}

// GetOnlineActors returns a list of all currently online actor IDs.
func (t *Tracker) GetOnlineActors() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	actors := make([]string, 0, len(t.actorConnections))
	for actorKey := range t.actorConnections {
		actors = append(actors, actorKey)
	}
	return actors
}

// GetLastSeen returns the last seen timestamp for an actor.
// Returns zero time if actor has never connected.
func (t *Tracker) GetLastSeen(actorID shared.ActorID) time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	actorKey := actorID.String()
	return t.lastSeen[actorKey]
}

// GetConnectionCount returns the number of active connections for an actor.
func (t *Tracker) GetConnectionCount(actorID shared.ActorID) int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	actorKey := actorID.String()
	return len(t.actorConnections[actorKey])
}

// GetPresenceMap returns a map of actor IDs to their current status.
// Optionally filters to only include the provided actor IDs.
func (t *Tracker) GetPresenceMap(actorIDs []shared.ActorID) map[string]string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]string)

	if len(actorIDs) == 0 {
		// Return all actors
		for actorKey := range t.actorConnections {
			result[actorKey] = StatusOnline
		}
	} else {
		// Return only requested actors
		for _, actorID := range actorIDs {
			actorKey := actorID.String()
			if len(t.actorConnections[actorKey]) > 0 {
				result[actorKey] = StatusOnline
			} else {
				result[actorKey] = StatusOffline
			}
		}
	}

	return result
}

// Cleanup removes stale entries (optional periodic cleanup).
// Not strictly necessary since entries are cleaned up on Disconnect.
func (t *Tracker) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Remove actors with no connections
	for actorKey, connections := range t.actorConnections {
		if len(connections) == 0 {
			delete(t.actorConnections, actorKey)
		}
	}
}

// broadcastPresence sends a presence update event to all connected clients.
// Must be called with lock held.
func (t *Tracker) broadcastPresence(actorID shared.ActorID, status string) {
	if t.broadcaster == nil {
		return
	}

	event := broadcast.Event{
		Type: "presence.updated",
		Data: map[string]interface{}{
			"actor_id":  actorID.String(),
			"status":    status,
			"timestamp": time.Now().Unix(),
		},
	}

	// Broadcast to all connected clients
	t.broadcaster.BroadcastToAll(event)
}
