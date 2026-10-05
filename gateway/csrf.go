package gateway

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// CSRFToken represents a CSRF token with expiration
type CSRFToken struct {
	Token     string
	ExpiresAt time.Time
}

// CSRFManager manages CSRF tokens for web clients
// Note: This is for browser-based web UI, not for API tokens (which use Bearer auth)
type CSRFManager struct {
	tokens map[string]*CSRFToken // sessionID -> CSRFToken
	mu     sync.RWMutex
}

// NewCSRFManager creates a new CSRF token manager
func NewCSRFManager() *CSRFManager {
	return &CSRFManager{
		tokens: make(map[string]*CSRFToken),
	}
}

// GenerateToken creates a new CSRF token for a session
func (m *CSRFManager) GenerateToken(sessionID string) (string, error) {
	// Generate 32 random bytes
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(24 * time.Hour)

	m.mu.Lock()
	m.tokens[sessionID] = &CSRFToken{
		Token:     token,
		ExpiresAt: expiresAt,
	}
	m.mu.Unlock()

	return token, nil
}

// ValidateToken checks if a CSRF token is valid for a session
func (m *CSRFManager) ValidateToken(sessionID, token string) bool {
	if sessionID == "" || token == "" {
		return false
	}

	m.mu.RLock()
	csrfToken, exists := m.tokens[sessionID]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	// Check expiration
	if time.Now().After(csrfToken.ExpiresAt) {
		// Clean up expired token
		m.mu.Lock()
		delete(m.tokens, sessionID)
		m.mu.Unlock()
		return false
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(csrfToken.Token), []byte(token)) == 1
}

// DeleteToken removes a CSRF token (e.g., on logout)
func (m *CSRFManager) DeleteToken(sessionID string) {
	m.mu.Lock()
	delete(m.tokens, sessionID)
	m.mu.Unlock()
}

// CleanupExpired removes expired CSRF tokens (should be called periodically)
func (m *CSRFManager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for sessionID, token := range m.tokens {
		if now.After(token.ExpiresAt) {
			delete(m.tokens, sessionID)
		}
	}
}
