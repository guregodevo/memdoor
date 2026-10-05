package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"memdoor/gateway/dto"
	"memdoor/gateway/websocket"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/shared"
)

// handleGetThreadReplies handles GET /api/messages/:id/replies
func (cs *ChatServer) handleGetThreadReplies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if MessageService is available
	if cs.messageService == nil {
		http.Error(w, "Message service not available", http.StatusInternalServerError)
		return
	}

	// Extract message ID from URL: /api/messages/:id/replies
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/messages/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	// Parse message ID
	messageIDStr := parts[0]
	messageIDInt, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid message ID: %v", err), http.StatusBadRequest)
		return
	}
	messageID := message.MessageID(messageIDInt)

	// Fetch thread replies from repository
	replies, err := cs.messageService.GetRepository().ListReplies(r.Context(), messageID)
	if err != nil {
		getChatLogger().Info("Failed to fetch thread replies",
			slog.String("message_id", messageIDStr),
			slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to fetch thread replies: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract message IDs for bulk reaction loading
	replyIDs := make([]message.MessageID, len(replies))
	for i, reply := range replies {
		replyIDs[i] = reply.ID
	}

	// Bulk-load reactions for all replies
	reactionsByMessage := make(map[message.MessageID][]*reaction.Reaction)
	if len(replyIDs) > 0 && cs.reactionRepo != nil {
		var err error
		reactionsByMessage, err = cs.reactionRepo.ListByMessages(r.Context(), replyIDs)
		if err != nil {
			getChatLogger().Info("Failed to load reactions for thread replies", slog.String("error", err.Error()))
			// Continue without reactions - don't fail the whole request
		}
	}

	replyDTOs := cs.enrichAndConvertBatch(r.Context(), replies, reactionsByMessage)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"replies": replyDTOs,
	})
}

// handleCreateThreadReply handles POST /api/messages/:id/replies
func (cs *ChatServer) handleCreateThreadReply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if MessageService is available
	if cs.messageService == nil {
		http.Error(w, "Message service not available", http.StatusInternalServerError)
		return
	}

	// Extract parent message ID from URL: /api/messages/:id/replies
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/messages/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	// Parse parent message ID
	parentIDStr := parts[0]
	parentIDInt, err := strconv.ParseInt(parentIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid message ID: %v", err), http.StatusBadRequest)
		return
	}
	parentID := message.MessageID(parentIDInt)

	// Parse request body
	var req struct {
		Text    string `json:"text"` // Simplified: just text in request body
		Content struct {
			Text     string   `json:"text"`
			Mentions []string `json:"mentions"`
		} `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get author_id from authenticated user context (security: prevent impersonation)
	authorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// Support both old format (content.text) and new format (text)
	messageText := req.Content.Text
	if messageText == "" {
		messageText = req.Text
	}

	if messageText == "" {
		http.Error(w, "Message text is required", http.StatusBadRequest)
		return
	}

	// Fetch parent message to get channel_id
	parentMessage, err := cs.messageService.GetRepository().FindByID(r.Context(), parentID)
	if err != nil {
		getChatLogger().Info("Failed to fetch parent message",
			slog.String("parent_id", parentIDStr),
			slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Parent message not found: %v", err), http.StatusNotFound)
		return
	}

	// Convert mentions to domain types
	mentions := make([]message.Mention, 0, len(req.Content.Mentions))
	for _, m := range req.Content.Mentions {
		actorID := shared.ActorID(m)
		// Extract name from ActorID (e.g., "agent:writer" -> "writer")
		name := string(actorID)
		if idx := strings.Index(name, ":"); idx > 0 && idx < len(name)-1 {
			name = name[idx+1:]
		}
		mentions = append(mentions, message.Mention{
			ActorID: actorID,
			Name:    name,
		})
	}

	// Create reply message using message service (this triggers agent mentions for thread auto-participation)
	content := message.MessageContent{
		Text:     messageText,
		Mentions: mentions,
	}

	// Unfurl deep links in reply content (same as regular messages)
	unfurledContent, err := cs.messageService.UnfurlDeepLinks(r.Context(), content)
	if err != nil {
		getChatLogger().Warn("Failed to unfurl deep links in reply", slog.String("error", err.Error()))
		unfurledContent = content // Use original content if unfurling fails
	}

	// PRESENTATION LAYER: Enrich link preview names BEFORE saving to database
	// This ensures the enriched names are persisted
	if len(unfurledContent.LinkPreviews) > 0 {
		unfurledContent.LinkPreviews = dto.EnrichLinkPreviewNames(
			r.Context(),
			unfurledContent.LinkPreviews,
			cs.nameEnricher,
			cs.channelRepo,
		)
	}

	// Use PostMessage with parent_id to create a thread reply AND trigger agent mentions
	// This enables thread auto-participation (lead agent receives all thread messages)
	replyMsg, err := message.NewReply(parentMessage.ChannelID, authorID, unfurledContent, parentID)
	if err != nil {
		getChatLogger().Info("Failed to create reply message", slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to create reply: %v", err), http.StatusBadRequest)
		return
	}

	// Save to database (this triggers handleAgentMentions for thread auto-participation)
	replyID, err := cs.messageService.GetRepository().Save(r.Context(), replyMsg)
	if err != nil {
		getChatLogger().Info("Failed to save reply", slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to save reply: %v", err), http.StatusInternalServerError)
		return
	}

	// Update message with assigned ID
	replyMsg.ID = replyID

	// IMPORTANT: Trigger agent mentions for thread auto-participation
	// Pattern: If this is a thread reply, auto-deliver to thread lead (no @mention needed)
	// Duck typing: messageService implements AgentMentionHandler interface
	if replyMsg.HasMentions() || replyMsg.ParentID != nil {
		// Extract ExecutionContext and attach to background context (detached from HTTP lifecycle)
		asyncCtx := context.Background()
		if execCtx := shared.GetExecutionContext(r.Context()); execCtx != nil {
			asyncCtx = shared.WithExecutionContext(asyncCtx, execCtx)
		} else {
			execCtx := shared.FromContextValues(r.Context())
			if execCtx != nil {
				asyncCtx = shared.WithExecutionContext(asyncCtx, execCtx)
			}
		}

		// Use duck-typed interface - cast concrete type to interface
		var handler AgentMentionHandler = cs.messageService
		go handler.HandleAgentMentions(asyncCtx, replyMsg)
	}

	// Broadcast reply message via WebSocket (so it appears in real-time)
	messageDTO := cs.enrichAndConvert(r.Context(), replyMsg, nil)

	cs.hub.BroadcastToChannel(replyMsg.ChannelID, websocket.WSMessage{
		Type: websocket.MessageCreated,
		Data: mustMarshal(messageDTO),
	})

	getChatLogger().Info("Created thread reply",
		slog.Int64("reply_id", int64(replyID)),
		slog.Int64("parent_id", int64(parentID)))

	// Return DTO in REST response
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(messageDTO)
}
