package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"memdoor/pkg/shared"

	"github.com/gorilla/websocket"
)

// Message types sent over WebSocket
type MessageType string

const (
	MessageCreated       MessageType = "message.created"
	MessageUpdated       MessageType = "message.updated"
	MessageDeleted       MessageType = "message.deleted"
	ExecutionStarted     MessageType = "execution.started"
	ExecutionEvent       MessageType = "execution.event"
	ExecutionCompleted   MessageType = "execution.completed"
	ExecutionTodoUpdated MessageType = "execution.todo.updated" // Agent todo list updates
	TypingStarted        MessageType = "typing.started"
	TypingStopped        MessageType = "typing.stopped"
	PresenceUpdated      MessageType = "presence.updated"
	MentionCreated       MessageType = "mention.created"
)

// WSMessage represents a message sent over WebSocket
type WSMessage struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID        string
	Conn      *websocket.Conn
	Send      chan WSMessage
	Hub       *Hub
	ChannelID string // Current channel the client is viewing
	UserID    string // User or agent ID
}

// UserMessage represents a message for a specific user
type UserMessage struct {
	UserID  string
	Message WSMessage
}

// Hub manages all WebSocket clients and broadcasts messages
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Clients grouped by channel
	channelClients map[string]map[*Client]bool

	// Clients grouped by user ID
	userClients map[string]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast message to all clients
	broadcast chan WSMessage

	// Broadcast message to specific channel
	channelBroadcast chan ChannelMessage

	// Broadcast message to specific user (all their connections)
	userBroadcast chan UserMessage

	// Presence tracker for online/offline status
	presenceTracker PresenceTracker

	mu sync.RWMutex
}

// ChannelMessage represents a message for a specific channel
type ChannelMessage struct {
	ChannelID string
	Message   WSMessage
}

// PresenceTracker is a duck-typed interface for tracking online/offline status
// Pattern: Dependency inversion - Hub depends on interface, not concrete implementation
type PresenceTracker interface {
	Connect(actorID shared.ActorID, clientID string)
	Disconnect(actorID shared.ActorID, clientID string)
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:          make(map[*Client]bool),
		channelClients:   make(map[string]map[*Client]bool),
		userClients:      make(map[string]map[*Client]bool),
		register:         make(chan *Client),
		unregister:       make(chan *Client),
		broadcast:        make(chan WSMessage, 256),
		channelBroadcast: make(chan ChannelMessage, 256),
		userBroadcast:    make(chan UserMessage, 256),
	}
}

// SetPresenceTracker sets the presence tracker for the hub
func (h *Hub) SetPresenceTracker(tracker PresenceTracker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.presenceTracker = tracker
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true

			// Add to channel clients
			if client.ChannelID != "" {
				if h.channelClients[client.ChannelID] == nil {
					h.channelClients[client.ChannelID] = make(map[*Client]bool)
				}
				h.channelClients[client.ChannelID][client] = true
			}

			// Add to user clients
			if client.UserID != "" {
				if h.userClients[client.UserID] == nil {
					h.userClients[client.UserID] = make(map[*Client]bool)
				}
				h.userClients[client.UserID][client] = true
			}

			// Track presence if user ID is set
			tracker := h.presenceTracker
			h.mu.Unlock()

			if tracker != nil && client.UserID != "" {
				actorID := shared.ActorID(client.UserID)
				tracker.Connect(actorID, client.ID)
			}

			log.Printf("[Hub] Client registered: %s (channel: %s, user: %s)", client.ID, client.ChannelID, client.UserID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)

				// Remove from channel clients
				if client.ChannelID != "" {
					delete(h.channelClients[client.ChannelID], client)
					if len(h.channelClients[client.ChannelID]) == 0 {
						delete(h.channelClients, client.ChannelID)
					}
				}

				// Remove from user clients
				if client.UserID != "" {
					delete(h.userClients[client.UserID], client)
					if len(h.userClients[client.UserID]) == 0 {
						delete(h.userClients, client.UserID)
					}
				}

				close(client.Send)

				// Track presence disconnect if user ID is set
				tracker := h.presenceTracker
				h.mu.Unlock()

				if tracker != nil && client.UserID != "" {
					actorID := shared.ActorID(client.UserID)
					tracker.Disconnect(actorID, client.ID)
				}
			} else {
				h.mu.Unlock()
			}
			log.Printf("[Hub] Client unregistered: %s", client.ID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					// Client's send buffer is full, disconnect
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()

		case channelMsg := <-h.channelBroadcast:
			h.mu.RLock()
			if clients, ok := h.channelClients[channelMsg.ChannelID]; ok {
				for client := range clients {
					select {
					case client.Send <- channelMsg.Message:
					default:
						// Client's send buffer is full, disconnect
						close(client.Send)
						delete(h.clients, client)
						delete(h.channelClients[channelMsg.ChannelID], client)
					}
				}
			}
			h.mu.RUnlock()

		case userMsg := <-h.userBroadcast:
			h.mu.RLock()
			if clients, ok := h.userClients[userMsg.UserID]; ok {
				for client := range clients {
					select {
					case client.Send <- userMsg.Message:
					default:
						close(client.Send)
						delete(h.clients, client)
						delete(h.userClients[userMsg.UserID], client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToAll sends a message to all connected clients
func (h *Hub) BroadcastToAll(message WSMessage) {
	h.broadcast <- message
}

// BroadcastToChannel sends a message to all clients in a specific channel
func (h *Hub) BroadcastToChannel(channelID string, message WSMessage) {
	h.channelBroadcast <- ChannelMessage{
		ChannelID: channelID,
		Message:   message,
	}
}

// BroadcastToUser sends a message to all connections of a specific user
func (h *Hub) BroadcastToUser(userID string, message WSMessage) {
	h.userBroadcast <- UserMessage{
		UserID:  userID,
		Message: message,
	}
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetChannelClientCount returns the number of clients in a specific channel
func (h *Hub) GetChannelClientCount(channelID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.channelClients[channelID]; ok {
		return len(clients)
	}
	return 0
}

// Constants for WebSocket configuration
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB
)

// ReadPump reads messages from the WebSocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Client] Error: %v", err)
			}
			break
		}

		// Parse incoming message
		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("[Client] Failed to parse message: %v", err)
			continue
		}

		// Handle incoming messages (e.g., typing indicators, presence updates)
		log.Printf("[Client] Received message: type=%s", wsMsg.Type)

		// Broadcast typing indicators to channel
		if wsMsg.Type == TypingStarted || wsMsg.Type == TypingStopped {
			if c.ChannelID != "" {
				c.Hub.BroadcastToChannel(c.ChannelID, wsMsg)
			}
		}
	}
}

// WritePump writes messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write message as JSON
			if err := c.Conn.WriteJSON(message); err != nil {
				log.Printf("[Client] Write error: %v", err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
