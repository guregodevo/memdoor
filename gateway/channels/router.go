package channels

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"memdoor/gateway/channels/adapters"
	"memdoor/gateway/logs"
)

// Router manages all channel adapters and routes messages between them and agents
// Pattern: OpenClaw multi-channel routing architecture
type Router struct {
	adapters map[string]adapters.ChannelAdapter // channel_id -> adapter
	bindings map[string]string                  // channel_user_id -> session_key
	mu       sync.RWMutex
	log      *logs.EventLogger
}

// NewRouter creates a new channel router
func NewRouter(verbose bool) *Router {
	return &Router{
		adapters: make(map[string]adapters.ChannelAdapter),
		bindings: make(map[string]string),
		log:      logs.New("Channels"),
	}
}

// RegisterAdapter adds a channel adapter to the router
func (r *Router) RegisterAdapter(adapter adapters.ChannelAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	channelID := adapter.GetID()
	if _, exists := r.adapters[channelID]; exists {
		return fmt.Errorf("channel adapter %q already registered", channelID)
	}

	r.adapters[channelID] = adapter

	r.log.Debug("Registered adapter",
		slog.String("channel_id", channelID),
		slog.String("type", adapter.GetType()))

	return nil
}

// UnregisterAdapter removes a channel adapter
func (r *Router) UnregisterAdapter(channelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	adapter, exists := r.adapters[channelID]
	if !exists {
		return fmt.Errorf("channel adapter %q not found", channelID)
	}

	// Stop the adapter if running
	if adapter.IsRunning() {
		if err := adapter.Stop(); err != nil {
			r.log.WithError(err).Warn("Failed to stop adapter",
				slog.String("channel_id", channelID))
		}
	}

	delete(r.adapters, channelID)

	r.log.Debug("Unregistered adapter", slog.String("channel_id", channelID))

	return nil
}

// GetAdapter returns a channel adapter by ID
func (r *Router) GetAdapter(channelID string) (adapters.ChannelAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, exists := r.adapters[channelID]
	if !exists {
		return nil, fmt.Errorf("channel adapter %q not found", channelID)
	}

	return adapter, nil
}

// ListAdapters returns all registered channel adapters
func (r *Router) ListAdapters() []adapters.ChannelAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]adapters.ChannelAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		list = append(list, adapter)
	}

	return list
}

// StartAll starts all registered channel adapters
func (r *Router) StartAll(ctx context.Context) error {
	r.mu.RLock()
	adapters := make([]adapters.ChannelAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		adapters = append(adapters, adapter)
	}
	r.mu.RUnlock()

	var errs []error
	for _, adapter := range adapters {
		if err := adapter.Start(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to start %s: %w", adapter.GetID(), err))
		} else {
			r.log.Debug("Started adapter", slog.String("channel_id", adapter.GetID()))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to start some adapters: %v", errs)
	}

	return nil
}

// StopAll stops all registered channel adapters
func (r *Router) StopAll() error {
	r.mu.RLock()
	adapters := make([]adapters.ChannelAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		adapters = append(adapters, adapter)
	}
	r.mu.RUnlock()

	var errs []error
	for _, adapter := range adapters {
		if adapter.IsRunning() {
			if err := adapter.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop %s: %w", adapter.GetID(), err))
			} else {
				r.log.Debug("Stopped adapter", slog.String("channel_id", adapter.GetID()))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to stop some adapters: %v", errs)
	}

	return nil
}

// BindUser maps a channel user to a session key
// Pattern: OpenClaw channel bindings (user@channel -> agent:id:session)
func (r *Router) BindUser(channelID, channelUserID, sessionKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create composite key: channel_id:channel_user_id
	compositeKey := fmt.Sprintf("%s:%s", channelID, channelUserID)
	r.bindings[compositeKey] = sessionKey

	r.log.Debug("Bound channel user to session",
		slog.String("composite_key", compositeKey),
		slog.String("session_key", sessionKey))

	return nil
}

// GetSessionKey returns the session key for a channel user
func (r *Router) GetSessionKey(channelID, channelUserID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	compositeKey := fmt.Sprintf("%s:%s", channelID, channelUserID)
	sessionKey, exists := r.bindings[compositeKey]
	if !exists {
		return "", fmt.Errorf("no binding found for %s", compositeKey)
	}

	return sessionKey, nil
}

// SendMessage sends a message to a specific channel user
func (r *Router) SendMessage(channelID, channelUserID, message string) error {
	adapter, err := r.GetAdapter(channelID)
	if err != nil {
		return err
	}

	return adapter.SendMessage(channelUserID, message)
}

// SendEvent sends an event to a specific channel user
func (r *Router) SendEvent(channelID, channelUserID string, event map[string]interface{}) error {
	adapter, err := r.GetAdapter(channelID)
	if err != nil {
		return err
	}

	return adapter.SendEvent(channelUserID, event)
}
