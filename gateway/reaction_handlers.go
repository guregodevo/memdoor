package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"memdoor/gateway/websocket"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
)

// handleReactions handles POST (add) and DELETE (remove) for message reactions
func (cs *ChatServer) handleReactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract message ID from URL: /api/messages/:id/reactions
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/messages/"), "/")
	if len(parts) < 2 || parts[1] != "reactions" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	messageIDStr := parts[0]
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	// AUTHORIZATION: Require authentication and use authenticated actor
	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodPost:
		// Add reaction
		var req struct {
			Emoji string `json:"emoji"` // e.g., "👍"
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Use authenticated actor ID instead of trusting request body
		react, err := reaction.NewReaction(message.MessageID(messageID), actorID, req.Emoji)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid reaction: %v", err), http.StatusBadRequest)
			return
		}

		reactionID, err := cs.reactionRepo.Add(r.Context(), react)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				http.Error(w, "Reaction already exists", http.StatusConflict)
			} else {
				http.Error(w, fmt.Sprintf("Failed to add reaction: %v", err), http.StatusInternalServerError)
			}
			return
		}

		// Broadcast reaction event via WebSocket
		cs.hub.BroadcastToChannel(fmt.Sprintf("channel-%d", messageID), websocket.WSMessage{
			Type: "reaction_added",
			Data: mustMarshal(map[string]interface{}{
				"message_id":  messageID,
				"reaction_id": reactionID,
				"user_id":     string(actorID),
				"emoji":       req.Emoji,
			}),
		})

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         reactionID,
			"message_id": messageID,
			"user_id":    string(actorID),
			"emoji":      req.Emoji,
		})

	case http.MethodDelete:
		// Remove reaction — only allow removing own reactions
		emoji := r.URL.Query().Get("emoji")

		if emoji == "" {
			http.Error(w, "Missing emoji query parameter", http.StatusBadRequest)
			return
		}

		err := cs.reactionRepo.Remove(r.Context(), message.MessageID(messageID), string(actorID), emoji)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to remove reaction: %v", err), http.StatusInternalServerError)
			return
		}

		// Broadcast reaction event via WebSocket
		cs.hub.BroadcastToChannel(fmt.Sprintf("channel-%d", messageID), websocket.WSMessage{
			Type: "reaction_removed",
			Data: mustMarshal(map[string]interface{}{
				"message_id": messageID,
				"user_id":    string(actorID),
				"emoji":      emoji,
			}),
		})

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
