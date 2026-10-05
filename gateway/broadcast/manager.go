package broadcast

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"

	"memdoor/gateway/logs"
)

// Event represents a broadcast event
// Pattern: OpenClaw's event broadcasting
type Event struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// Subscriber represents a client that can receive broadcast events
type Subscriber interface {
	Send(data []byte) error
	GetID() string
}

// SubscriptionManager manages client subscriptions to events
// Pattern: OpenClaw src/gateway/server-node-subscriptions.ts
type SubscriptionManager struct {
	// Map of runID -> set of subscriber IDs
	runSubscribers map[string]map[string]Subscriber

	// Map of sessionKey -> set of subscriber IDs
	sessionSubscribers map[string]map[string]Subscriber

	// All subscribers
	allSubscribers map[string]Subscriber

	mu  sync.RWMutex
	log *logs.EventLogger
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(verbose bool) *SubscriptionManager {
	return &SubscriptionManager{
		runSubscribers:     make(map[string]map[string]Subscriber),
		sessionSubscribers: make(map[string]map[string]Subscriber),
		allSubscribers:     make(map[string]Subscriber),
		log:                logs.New("Broadcast"),
	}
}

// AddSubscriber adds a subscriber to the manager
func (sm *SubscriptionManager) AddSubscriber(sub Subscriber) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.allSubscribers[sub.GetID()] = sub

	sm.log.Debug("Added subscriber", slog.String("subscriber_id", sub.GetID()))
}

// RemoveSubscriber removes a subscriber from all subscriptions
func (sm *SubscriptionManager) RemoveSubscriber(subscriberID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Remove from all subscriptions
	delete(sm.allSubscribers, subscriberID)

	// Remove from run subscriptions
	for runID, subs := range sm.runSubscribers {
		delete(subs, subscriberID)
		if len(subs) == 0 {
			delete(sm.runSubscribers, runID)
		}
	}

	// Remove from session subscriptions
	for sessionKey, subs := range sm.sessionSubscribers {
		delete(subs, subscriberID)
		if len(subs) == 0 {
			delete(sm.sessionSubscribers, sessionKey)
		}
	}

	sm.log.Debug("Removed subscriber", slog.String("subscriber_id", subscriberID))
}

// SubscribeToRun subscribes a client to events for a specific run
func (sm *SubscriptionManager) SubscribeToRun(subscriberID, runID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sub, ok := sm.allSubscribers[subscriberID]
	if !ok {
		return
	}

	subs, ok := sm.runSubscribers[runID]
	if !ok {
		subs = make(map[string]Subscriber)
		sm.runSubscribers[runID] = subs
	}

	subs[subscriberID] = sub

	sm.log.Debug("Subscriber subscribed to run",
		slog.String("subscriber_id", subscriberID),
		slog.String("run_id", runID))
}

// UnsubscribeFromRun unsubscribes a client from run events
func (sm *SubscriptionManager) UnsubscribeFromRun(subscriberID, runID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs, ok := sm.runSubscribers[runID]
	if !ok {
		return
	}

	delete(subs, subscriberID)
	if len(subs) == 0 {
		delete(sm.runSubscribers, runID)
	}

	sm.log.Debug("Subscriber unsubscribed from run",
		slog.String("subscriber_id", subscriberID),
		slog.String("run_id", runID))
}

// SubscribeToSession subscribes a client to events for a specific session
func (sm *SubscriptionManager) SubscribeToSession(subscriberID, sessionKey string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sub, ok := sm.allSubscribers[subscriberID]
	if !ok {
		return
	}

	subs, ok := sm.sessionSubscribers[sessionKey]
	if !ok {
		subs = make(map[string]Subscriber)
		sm.sessionSubscribers[sessionKey] = subs
	}

	subs[subscriberID] = sub

	sm.log.Debug("Subscriber subscribed to session",
		slog.String("subscriber_id", subscriberID),
		slog.String("session_key", sessionKey))
}

// BroadcastToRun broadcasts an event to all subscribers of a run
// Pattern: OpenClaw's broadcastToConnIds
func (sm *SubscriptionManager) BroadcastToRun(runID string, event Event) {
	sm.mu.RLock()
	subs, ok := sm.runSubscribers[runID]
	sm.mu.RUnlock()

	if !ok || len(subs) == 0 {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		sm.log.WithError(err).Error("Failed to marshal event",
			slog.String("event_type", event.Type),
			slog.String("run_id", runID))
		return
	}

	sent := 0
	for _, sub := range subs {
		if err := sub.Send(data); err != nil {
			sm.log.Debug("Failed to send to subscriber",
				slog.String("subscriber_id", sub.GetID()),
				slog.String("error", err.Error()))
		} else {
			sent++
		}
	}

	sm.log.Debug("Event sent to run subscribers",
		slog.String("event_type", event.Type),
		slog.Int("sent", sent),
		slog.Int("total", len(subs)),
		slog.String("run_id", runID))
}

// BroadcastToSession broadcasts an event to all subscribers of a session
func (sm *SubscriptionManager) BroadcastToSession(sessionKey string, event Event) {
	sm.mu.RLock()
	subs, ok := sm.sessionSubscribers[sessionKey]
	sm.mu.RUnlock()

	if !ok || len(subs) == 0 {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		sm.log.WithError(err).Error("Failed to marshal event",
			slog.String("event_type", event.Type),
			slog.String("session_key", sessionKey))
		return
	}

	sent := 0
	for _, sub := range subs {
		if err := sub.Send(data); err != nil {
			sm.log.Debug("Failed to send to subscriber",
				slog.String("subscriber_id", sub.GetID()),
				slog.String("error", err.Error()))
		} else {
			sent++
		}
	}

	sm.log.Debug("Event sent to session subscribers",
		slog.String("event_type", event.Type),
		slog.Int("sent", sent),
		slog.Int("total", len(subs)),
		slog.String("session_key", sessionKey))
}

// BroadcastToAll broadcasts an event to all connected subscribers
// Pattern: OpenClaw's broadcast()
func (sm *SubscriptionManager) BroadcastToAll(event Event) {
	sm.mu.RLock()
	subs := make([]Subscriber, 0, len(sm.allSubscribers))
	for _, sub := range sm.allSubscribers {
		subs = append(subs, sub)
	}
	sm.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		sm.log.WithError(err).Error("Failed to marshal event",
			slog.String("event_type", event.Type))
		return
	}

	sent := 0
	for _, sub := range subs {
		if err := sub.Send(data); err != nil {
			sm.log.Debug("Failed to send to subscriber",
				slog.String("subscriber_id", sub.GetID()),
				slog.String("error", err.Error()))
		} else {
			sent++
		}
	}

	sm.log.Debug("Event sent to all subscribers",
		slog.String("event_type", event.Type),
		slog.Int("sent", sent),
		slog.Int("total", len(subs)))
}

// BroadcastToWorkspace sends an event to every window of one workspace — the
// subscribers of its "workspace:<ws>:channel:…" sessions — once each, and to
// nobody else. A workflow run's events go here: a reopened window of the same
// workspace sees them, another user's window on a shared gateway does not
// (review, 2026-10-04: "every window" leaked runs across users).
func (sm *SubscriptionManager) BroadcastToWorkspace(workspace string, event Event) {
	if workspace == "" {
		return
	}
	prefix := "workspace:" + workspace + ":"
	sm.mu.RLock()
	seen := map[string]bool{}
	var subs []Subscriber
	for key, byID := range sm.sessionSubscribers {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		for id, sub := range byID {
			if !seen[id] {
				seen[id] = true
				subs = append(subs, sub)
			}
		}
	}
	sm.mu.RUnlock()
	if len(subs) == 0 {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	for _, sub := range subs {
		_ = sub.Send(data)
	}
}
