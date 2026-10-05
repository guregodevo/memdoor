package websocket

import (
	"log"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// TokenValidator is an interface for validating authentication tokens
type TokenValidator interface {
	ValidateToken(token string) (userID string, err error)
}

// sameOriginOnly is the fallback CheckOrigin when the caller injects none: it
// accepts a request with no Origin header (CLI/TUI clients) and otherwise
// requires the Origin's host to equal the request Host — the browser
// same-origin policy for WebSockets. This is what gorilla does by default, made
// explicit so a nil checker is never a mistake.
func sameOriginOnly(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if i := len(origin); i > 0 {
		if u, err := url.Parse(origin); err == nil {
			return u.Host == r.Host
		}
	}
	return false
}

// HandleWebSocketWithAuth upgrades HTTP connection to WebSocket with authentication.
// The validator is mandatory: there is no unauthenticated mode. The former
// "legacy" branch that trusted a ?user=<id> query parameter is gone with it.
//
// checkOrigin gates cross-origin upgrades (CSWSH protection). The caller passes
// the gateway's own origin policy; a nil checker falls back to same-origin only.
// The previous "return true" allowed ANY site to open an authenticated socket.
func HandleWebSocketWithAuth(hub *Hub, tokenValidator TokenValidator, checkOrigin func(*http.Request) bool) http.HandlerFunc {
	if tokenValidator == nil {
		panic("websocket: HandleWebSocketWithAuth requires a TokenValidator")
	}
	if checkOrigin == nil {
		checkOrigin = sameOriginOnly
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkOrigin,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Authenticate the WebSocket connection
		// Accept token from Authorization header or query parameter
		token := ""

		// Try Authorization header first (preferred)
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}

		// Fallback to query parameter (for browser WebSocket clients)
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			log.Printf("[WebSocket] Connection rejected: no token provided")
			return
		}

		userID, err := tokenValidator.ValidateToken(token)
		if err != nil {
			http.Error(w, "Invalid authentication token", http.StatusUnauthorized)
			log.Printf("[WebSocket] Connection rejected: invalid token - %v", err)
			return
		}

		// Get channel from query parameters
		channelID := r.URL.Query().Get("channel")

		// Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[WebSocket] Upgrade failed: %v", err)
			return
		}

		// Create new client
		client := &Client{
			ID:        uuid.New().String(),
			Conn:      conn,
			Send:      make(chan WSMessage, 256),
			Hub:       hub,
			ChannelID: channelID,
			UserID:    userID,
		}

		// Register client
		hub.register <- client

		log.Printf("[WebSocket] New connection: client=%s, channel=%s, user=%s", client.ID, channelID, userID)

		// Start read/write pumps
		go client.WritePump()
		go client.ReadPump()
	}
}
