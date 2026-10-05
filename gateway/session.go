package gateway

import (
	"fmt"
	"sync"
	"time"

	"memdoor/tools"
)

// Session represents a conversation session
type Session struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // "main", "group", "channel"
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata"`
	Messages  []SessionMessage       `json:"messages"`
	mu        sync.RWMutex
}

// SessionMessage represents a message in a session
type SessionMessage struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionManager manages all active sessions
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new session with a random ID
func (sm *SessionManager) CreateSession(sessionType string) (*Session, error) {
	if sm == nil {
		return nil, fmt.Errorf("session manager not initialized")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &Session{
		ID:        generateID("session"),
		Type:      sessionType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
		Messages:  make([]SessionMessage, 0),
	}

	sm.sessions[session.ID] = session
	return session, nil
}

// GetOrCreateSession gets an existing session or creates one with a specific ID
// Pattern: OpenClaw's stable session management - allows reconnection to same session
func (sm *SessionManager) GetOrCreateSession(id, sessionType string) (*Session, error) {
	// Guard against nil SessionManager (can happen in tests or incomplete initialization)
	if sm == nil {
		return nil, fmt.Errorf("session manager not initialized")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session exists
	if session, exists := sm.sessions[id]; exists {
		return session, nil
	}

	// Create new session with specific ID
	session := &Session{
		ID:        id, // Use provided ID, not generated
		Type:      sessionType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
		Messages:  make([]SessionMessage, 0),
	}

	sm.sessions[id] = session
	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id string) (*Session, error) {
	if sm == nil {
		return nil, fmt.Errorf("session manager not initialized")
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(id string) error {
	if sm == nil {
		return fmt.Errorf("session manager not initialized")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	delete(sm.sessions, id)
	return nil
}

// ListSessions returns all sessions
func (sm *SessionManager) ListSessions() []*Session {
	if sm == nil {
		return []*Session{}
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// Count returns the number of active sessions
func (sm *SessionManager) Count() int {
	if sm == nil {
		return 0
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// maxInMemorySessionMessages bounds the in-RAM message slice so a long-lived,
// shared channel session doesn't grow without limit in a long-running gateway.
// Mirrors the persistence-layer keep count; the durable trim lives in
// SessionPersistence.compactSessionFile.
const maxInMemorySessionMessages = sessionKeepMessages

// AddMessage adds a message to a session, capping the in-memory history at
// maxInMemorySessionMessages (oldest dropped). Sessions are shared across all
// agents in a channel, so this keeps the in-RAM copy bounded regardless of how
// busy the channel gets.
func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := SessionMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	s.Messages = append(s.Messages, msg)
	if len(s.Messages) > maxInMemorySessionMessages {
		s.Messages = s.Messages[len(s.Messages)-maxInMemorySessionMessages:]
	}
	s.UpdatedAt = time.Now()
}

// GetMessages returns all messages in a session
func (s *Session) GetMessages() []SessionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	messages := make([]SessionMessage, len(s.Messages))
	copy(messages, s.Messages)
	return messages
}

// Clear clears all messages in a session
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = make([]SessionMessage, 0)
	s.UpdatedAt = time.Now()
}

// SetMetadata sets a metadata value
func (s *Session) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Metadata[key] = value
	s.UpdatedAt = time.Now()
}

// GetMetadata gets a metadata value (deprecated: use GetMetadataValue)
func (s *Session) GetMetadata() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent concurrent modification
	meta := make(map[string]interface{}, len(s.Metadata))
	for k, v := range s.Metadata {
		meta[k] = v
	}
	return meta
}

// GetMetadataValue gets a specific metadata value by key
func (s *Session) GetMetadataValue(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.Metadata[key]
	return val, ok
}

// GetKey returns the session ID
// Pattern: Implements tools.Session interface for sessions_spawn tool
func (s *Session) GetKey() string {
	return s.ID
}

// SetSystemPrompt sets the system prompt for this session
// Pattern: Implements tools.Session interface for subagent spawning
func (s *Session) SetSystemPrompt(prompt string) error {
	s.SetMetadata("system_prompt", prompt)
	return nil
}

// SessionManagerAdapter wraps SessionManager to implement tools.SessionGetter
// Pattern: Adapter pattern for interface compatibility
type SessionManagerAdapter struct {
	manager *SessionManager
}

// NewSessionManagerAdapter creates a new adapter wrapper
func NewSessionManagerAdapter(manager *SessionManager) *SessionManagerAdapter {
	return &SessionManagerAdapter{manager: manager}
}

// GetOrCreateSession implements tools.SessionGetter interface
// Returns Session as tools.Session interface (gateway.Session implements this)
func (sma *SessionManagerAdapter) GetOrCreateSession(id, sessionType string) (tools.Session, error) {
	return sma.manager.GetOrCreateSession(id, sessionType)
}
