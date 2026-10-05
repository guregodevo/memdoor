package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"memdoor/gateway/dto"
	"memdoor/gateway/websocket"
	"memdoor/pkg/channel"
	"memdoor/pkg/message"
	"memdoor/pkg/shared"
)

func (cs *ChatServer) handleMessagesWithID(w http.ResponseWriter, r *http.Request) {
	// Check if this is a mark-as-read request (/api/messages/:id/read)
	if strings.HasSuffix(r.URL.Path, "/read") {
		cs.handleMarkMessageRead(w, r)
		return
	}

	// Check if this is a reactions request (/api/messages/:id/reactions)
	if strings.Contains(r.URL.Path, "/reactions") {
		cs.handleReactions(w, r)
		return
	}

	// Check if this is a thread replies request (/api/messages/:id/replies)
	if strings.HasSuffix(r.URL.Path, "/replies") {
		// Route based on HTTP method
		switch r.Method {
		case http.MethodGet:
			cs.handleGetThreadReplies(w, r)
		case http.MethodPost:
			cs.handleCreateThreadReply(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// GET /api/messages/:id - Fetch a single message by ID
	if r.Method == http.MethodGet {
		cs.handleGetSingleMessage(w, r)
		return
	}

	// PUT /api/messages/:id - Edit a message
	if r.Method == http.MethodPut {
		cs.handleEditMessage(w, r)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleGetSingleMessage handles GET /api/messages/:id
func (cs *ChatServer) handleGetSingleMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// Extract message ID from URL: /api/messages/:id
	path := r.URL.Path
	messageIDStr := strings.TrimPrefix(path, "/api/messages/")

	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	// Fetch the message from the database
	msg, err := cs.messageService.GetRepository().FindByID(r.Context(), message.MessageID(messageID))
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	// AUTHORIZATION: Verify user can read the message's channel
	if cs.authzService != nil {
		canRead, err := cs.authzService.CanReadChannel(r.Context(), actorID, channel.ChannelID(msg.ChannelID))
		if err != nil {
			http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
			return
		}
		if !canRead {
			http.Error(w, `{"error": "Permission denied"}`, http.StatusForbidden)
			return
		}
	}

	messageDTO := cs.enrichAndConvert(r.Context(), msg, nil)

	json.NewEncoder(w).Encode(messageDTO)
}

// handleEditMessage handles PUT /api/messages/:id
func (cs *ChatServer) handleEditMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// Extract message ID from URL: /api/messages/:id
	path := r.URL.Path
	messageIDStr := strings.TrimPrefix(path, "/api/messages/")
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid message ID"}`, http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		http.Error(w, `{"error": "Text cannot be empty"}`, http.StatusBadRequest)
		return
	}

	// Edit message via service (handles authorization + persistence + broadcast)
	editedMsg, err := cs.messageService.EditMessage(r.Context(), message.EditMessageRequest{
		MessageID: message.MessageID(messageID),
		AuthorID:  actorID,
		NewText:   req.Text,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error": "Message not found"}`, http.StatusNotFound)
		} else if strings.Contains(err.Error(), "only the message author") {
			http.Error(w, `{"error": "Permission denied"}`, http.StatusForbidden)
		} else {
			getChatLogger().Error("Failed to edit message", slog.String("error", err.Error()))
			http.Error(w, `{"error": "Failed to edit message"}`, http.StatusInternalServerError)
		}
		return
	}

	messageDTO := cs.enrichAndConvert(r.Context(), editedMsg, nil)

	// Broadcast message.updated to WebSocket clients
	cs.hub.BroadcastToChannel(editedMsg.ChannelID, websocket.WSMessage{
		Type: websocket.MessageUpdated,
		Data: mustMarshal(messageDTO),
	})

	json.NewEncoder(w).Encode(messageDTO)
}

func (cs *ChatServer) handleMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		var req struct {
			ChannelID      string   `json:"channel_id"`
			Text           string   `json:"text"`                      // Simplified: just text in request body
			SenderID       string   `json:"sender_id,omitempty"`       // Override sender for internal service calls
			Workspace      string   `json:"workspace"`                 // Workspace slug (required)
			PermissionMode string   `json:"permission_mode,omitempty"` // Claude-style mode: default|acceptEdits|plan
			Workdir        string   `json:"workdir,omitempty"`         // Client's launch dir — the coder works on THIS project (Claude-CLI semantics)
			AttachmentIDs  []string `json:"attachment_ids,omitempty"`
			Content        struct {
				Text     string   `json:"text"`
				Mentions []string `json:"mentions"`
			} `json:"content"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// A workspace is required — fail fast, it is the scope.
		workspace := strings.TrimSpace(req.Workspace)
		if workspace == "" {
			http.Error(w, `{"error":"workspace is required"}`, http.StatusBadRequest)
			return
		}
		execCtx := shared.GetExecutionContext(r.Context())
		if execCtx != nil {
			execCtx.WorkspaceSlug = workspace
		} else {
			// Guest users may not have an execution context yet — create one
			execCtx = &shared.ExecutionContext{WorkspaceSlug: workspace}
			ctx := shared.WithExecutionContext(r.Context(), execCtx)
			r = r.WithContext(ctx)
		}

		// Get author_id from authenticated user context (security: prevent impersonation)
		authorID, ok := requireAuth(w, r)
		if !ok {
			return
		}

		// Allow system:internal to specify sender_id (trusted agent tools)
		isInternalService := authorID == "system:internal"
		if isInternalService && req.SenderID != "" {
			authorID = shared.ActorID(req.SenderID)
		}

		// AUTHORIZATION CHECK: Verify user can write to this channel (defense-in-depth)
		// Note: MessageService also checks membership, but gateway check provides faster fail
		// Skip for system:internal (trusted agent tools — even when sender_id overrides authorID)
		if cs.authzService != nil && !isInternalService && authorID != "system:internal" {
			canWrite, err := cs.authzService.CanWriteToChannel(r.Context(), authorID, channel.ChannelID(req.ChannelID))
			if err != nil {
				getChatLogger().Error("Failed to check write permission",
					slog.String("actor_id", authorID.String()),
					slog.String("channel_id", req.ChannelID),
					slog.String("error", err.Error()))
				http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
				return
			}
			if !canWrite {
				http.Error(w, `{"error": "Permission denied: not a member of this channel"}`, http.StatusForbidden)
				return
			}
		}

		// ✅ CORRECT: Use MessageService (DDD layer)
		if cs.messageService == nil {
			http.Error(w, "Message service not available", http.StatusInternalServerError)
			return
		}

		// Support both old format (content.text) and new format (text)
		messageText := req.Content.Text
		if messageText == "" {
			messageText = req.Text
		}

		// Call MessageService (validates, resolves mentions, persists, triggers agents)
		// Mention resolution and DM auto-mention are domain logic handled by the service
		msg, err := cs.messageService.PostMessage(r.Context(), message.PostMessageRequest{
			ChannelID:           req.ChannelID,
			AuthorID:            authorID,
			SkipMembershipCheck: isInternalService,
			PermissionMode:      req.PermissionMode,
			Workdir:             req.Workdir,
			Content: message.MessageContent{
				Text: messageText,
			},
		})

		if err != nil {
			getChatLogger().Info("Failed to post message", slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf("Failed to post message: %v", err), http.StatusBadRequest)
			return
		}

		// Attach files to message if any
		var fileDTOs []dto.FileRefDTO
		if len(req.AttachmentIDs) > 0 && cs.fileService != nil {
			if err := cs.fileService.AttachToMessage(r.Context(), req.AttachmentIDs, int64(msg.ID)); err != nil {
				getChatLogger().Warn("Failed to attach files to message",
					slog.String("error", err.Error()))
			}
			// Load attached files for DTO
			attachments, err := cs.fileService.GetAttachmentsBatch(r.Context(), []int64{int64(msg.ID)})
			if err == nil {
				for _, f := range attachments[int64(msg.ID)] {
					fileDTOs = append(fileDTOs, dto.FileRefDTO{
						ID:        f.ID,
						Filename:  f.Filename,
						MimeType:  f.MimeType,
						SizeBytes: f.SizeBytes,
						URL:       fmt.Sprintf("/api/files/%s", f.ID),
					})
				}
			}
		}

		// Broadcast saved message (with real ID from database)
		// Note: This is redundant with CreateBroadcaster, but kept for backward compatibility
		messageDTO := cs.enrichAndConvert(r.Context(), msg, nil, fileDTOs...)

		cs.hub.BroadcastToChannel(msg.ChannelID, websocket.WSMessage{
			Type: websocket.MessageCreated,
			Data: mustMarshal(messageDTO),
		})

		// MessageService handles agent execution internally via Published Interface
		// No need for handleAgentMentions() - that bypasses DDD!

		// Return DTO in REST response
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(messageDTO)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMarkChannelRead marks all messages in a channel as read
func (cs *ChatServer) handleMarkChannelRead(w http.ResponseWriter, r *http.Request, channelID string) {
	w.Header().Set("Content-Type", "application/json")

	if _, ok := requireAuth(w, r); !ok {
		return
	}

	if cs.messageService == nil {
		http.Error(w, `{"error": "Message service not available"}`, http.StatusInternalServerError)
		return
	}

	messageRepo := cs.messageService.GetRepository()
	if err := messageRepo.MarkChannelAsRead(r.Context(), channelID); err != nil {
		getChatLogger().Info("Failed to mark channel as read", slog.String("error", err.Error()))
		http.Error(w, `{"error": "Failed to mark channel as read"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (cs *ChatServer) handleMarkMessageRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}
	_ = actorID // authenticated user context established

	// Extract message ID from URL: /api/messages/:id/read
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/messages/"), "/")
	if len(parts) < 2 || parts[1] != "read" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	messageIDStr := parts[0]
	messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	// Use message repository to mark as read
	if cs.messageService == nil {
		http.Error(w, "Message service not available", http.StatusInternalServerError)
		return
	}

	messageRepo := cs.messageService.GetRepository()
	err = messageRepo.MarkAsRead(r.Context(), message.MessageID(messageID))
	if err != nil {
		getChatLogger().Info("Failed to mark message as read", slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to mark message as read: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"message_id": messageID,
	})
}

// handleUnreadMentionCounts returns unread @mention counts per channel for the authenticated user
// GET /api/messages/unread-mentions
func (cs *ChatServer) handleUnreadMentionCounts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user from context
	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// Get message repository from service
	if cs.messageService == nil {
		http.Error(w, "Message service not available", http.StatusInternalServerError)
		return
	}

	messageRepo := cs.messageService.GetRepository()
	counts, err := messageRepo.CountUnreadMentionsByChannel(r.Context(), actorID)
	if err != nil {
		getChatLogger().Error("Failed to get unread mention counts",
			slog.String("actor_id", actorID.String()),
			slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to get mention counts: %v", err), http.StatusInternalServerError)
		return
	}

	// Return counts as JSON map: { "channel-uuid-1": 3, "channel-uuid-2": 1 }
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(counts)
}

// handleListUnreadMentions returns all unread messages that mention the authenticated user
// GET /api/messages/threads
func (cs *ChatServer) handleListUnreadMentions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user from context
	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	getChatLogger().Info("GET /api/messages/threads called",
		slog.String("actor_id", actorID.String()))

	// Parse limit from query params (default 100)
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get message repository from service
	if cs.messageService == nil {
		http.Error(w, "Message service not available", http.StatusInternalServerError)
		return
	}

	messageRepo := cs.messageService.GetRepository()
	mentions, err := messageRepo.ListUnreadMentions(r.Context(), actorID, limit)
	if err != nil {
		getChatLogger().Error("Failed to list unread mentions",
			slog.String("actor_id", actorID.String()),
			slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to list mentions: %v", err), http.StatusInternalServerError)
		return
	}

	// Also fetch unread thread replies (replies to messages the user authored)
	threadReplies, err := messageRepo.ListUnreadThreadReplies(r.Context(), actorID, limit)
	if err != nil {
		getChatLogger().Warn("Failed to list unread thread replies",
			slog.String("actor_id", actorID.String()),
			slog.String("error", err.Error()))
		// Non-fatal: continue with mentions only
	}

	// Merge and deduplicate
	seen := make(map[string]bool, len(mentions))
	messages := make([]*message.Message, 0, len(mentions)+len(threadReplies))
	for _, msg := range mentions {
		seen[msg.ID.String()] = true
		messages = append(messages, msg)
	}
	for _, msg := range threadReplies {
		if !seen[msg.ID.String()] {
			messages = append(messages, msg)
		}
	}

	getChatLogger().Info("Returning unread threads",
		slog.String("actor_id", actorID.String()),
		slog.Int("mentions", len(mentions)),
		slog.Int("thread_replies", len(threadReplies)),
		slog.Int("total", len(messages)),
		slog.Int("limit", limit))

	messageDTOs := cs.enrichAndConvertBatch(r.Context(), messages, nil)

	response := map[string]interface{}{
		"messages": messageDTOs,
		"count":    len(messageDTOs),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
