package gateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"memdoor/gateway/dto"
	"memdoor/pkg/authorization"
	"memdoor/pkg/channel"
	"memdoor/pkg/domain"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/shared"

	"github.com/google/uuid"
)

func (cs *ChatServer) handleRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Fetch active channels from repository
		if cs.channelRepo == nil {
			http.Error(w, "Channel repository not available", http.StatusInternalServerError)
			return
		}

		allChannels, err := cs.channelRepo.ListActive(r.Context(), shared.AllPages())
		if err != nil {
			getChatLogger().Info("Failed to list channels", slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf("Failed to list channels: %v", err), http.StatusInternalServerError)
			return
		}

		// AUTHORIZATION: Filter channels user can access
		// Only return public channels OR private channels where user is a member
		// system:internal sees all channels (trusted agent tools)
		actorID := authorization.GetActorID(r.Context())
		getChatLogger().Info("GET /api/channels request",
			slog.String("actor_id", actorID.String()),
			slog.Int("total_channels", len(allChannels)))

		var accessibleChannels []*domain.Channel
		if actorID == "system:internal" {
			accessibleChannels = allChannels
		} else {
			accessibleChannels = make([]*domain.Channel, 0, len(allChannels))
			for _, ch := range allChannels {
				canRead, err := cs.authzService.CanReadChannel(r.Context(), actorID, channel.ChannelID(ch.ID.String()))
				if err != nil {
					getChatLogger().Warn("Failed to check channel access",
						slog.String("channel_id", ch.ID.String()),
						slog.String("error", err.Error()))
					continue
				}
				if canRead {
					accessibleChannels = append(accessibleChannels, ch)
				}
			}
		}

		getChatLogger().Info("Filtered accessible channels",
			slog.String("actor_id", actorID.String()),
			slog.Int("accessible_channels", len(accessibleChannels)),
			slog.Int("total_channels", len(allChannels)))

		// PRESENTATION LAYER: Convert domain Channels to DTOs
		channelDTOs := dto.ToChannelDTOBatch(accessibleChannels)

		// Populate members for each channel (needed for autocomplete filtering)
		membershipRepo := cs.channelService.GetMembershipRepository()
		if membershipRepo != nil {
			for i, ch := range accessibleChannels {
				memberships, err := membershipRepo.ListByChannel(r.Context(), channel.ChannelID(ch.ID.String()), shared.AllPages())
				if err != nil {
					getChatLogger().Warn("Failed to fetch members for channel",
						slog.String("channel_id", ch.ID.String()),
						slog.String("error", err.Error()))
					continue // Skip this channel's members, but don't fail the whole request
				}

				// Convert memberships to ActorID strings
				members := make([]string, len(memberships))
				for j, membership := range memberships {
					members[j] = membership.ID.ActorID.String()
				}
				channelDTOs[i].Members = members
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"channels": channelDTOs,
		})

	case http.MethodPost:
		// Create new channel (API endpoint: POST /api/channels)
		// AUTHORIZATION: Require workspace admin (same pattern as channel delete)
		createActorID := authorization.GetActorID(r.Context())
		if createActorID == "" {
			http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
			return
		}
		if cs.authzService != nil && createActorID != "system:internal" {
			isAdmin, err := cs.authzService.IsWorkspaceAdmin(r.Context(), createActorID)
			if err != nil {
				http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
				return
			}
			if !isAdmin {
				http.Error(w, `{"error": "Permission denied: only admins can create channels"}`, http.StatusForbidden)
				return
			}
		}

		if cs.channelRepo == nil {
			http.Error(w, "Channel repository not available", http.StatusInternalServerError)
			return
		}

		var req struct {
			Name        string  `json:"name"`
			Type        string  `json:"type"`
			Description *string `json:"description"`
			OwnerID     string  `json:"owner_id,omitempty"` // Owner to auto-add as admin (for internal service calls)
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate input
		if req.Name == "" {
			http.Error(w, "Channel name is required", http.StatusBadRequest)
			return
		}

		// Convert type to IsPrivate boolean
		isPrivate := req.Type == "private"

		// Create domain model (domain.Channel represents a channel: #general, #test2)
		now := time.Now()
		domainChannel := &domain.Channel{
			ID:          uuid.New(),
			Name:        req.Name,
			Description: req.Description,
			IsPrivate:   isPrivate,
			WorkspaceScoped: domain.WorkspaceScoped{
				WorkspaceID: uuid.Nil, // SQLite doesn't use workspaces
			},
			Timestamps: domain.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
			Archivable: domain.Archivable{
				ArchivedAt: nil,
			},
		}

		// Save to database
		if err := cs.channelRepo.Create(r.Context(), domainChannel); err != nil {
			getChatLogger().Info("Failed to create channel", slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf("Failed to create channel: %v", err), http.StatusInternalServerError)
			return
		}

		// Automatically add the owner as an admin member
		// For system:internal calls, use owner_id from request body
		// For regular users, use the authenticated user
		currentUserID := authorization.GetActorID(r.Context())
		ownerID := currentUserID
		if currentUserID == "system:internal" && req.OwnerID != "" {
			ownerID = shared.ActorID(req.OwnerID)
		}
		if ownerID != "" && ownerID != "system:internal" {
			addMemberReq := channel.AddMemberRequest{
				ChannelID:   channel.ChannelID(domainChannel.ID.String()),
				ActorID:     ownerID,
				RequesterID: nil, // System operation - bypass authorization
				Role:        channel.RoleAdmin,
			}

			_, err := cs.channelService.AddMember(r.Context(), addMemberReq)
			if err != nil {
				getChatLogger().Info("Warning: Failed to add owner as member", slog.String("error", err.Error()))
			} else {
				getChatLogger().Info("Added owner as admin to channel",
					slog.String("owner_id", ownerID.String()),
					slog.String("channel_id", domainChannel.ID.String()))
			}
		}

		// PRESENTATION LAYER: Convert to DTO
		channelDTO := dto.ToChannelDTO(domainChannel)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(channelDTO)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (cs *ChatServer) handleRoom(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract channel ID and sub-path from URL
	// /api/channels/{id}/members or /api/channels/{id} or /api/channels/dm
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/channels/"), "/")

	// Handle POST /api/channels/dm (Direct Message creation)
	if len(parts) == 1 && parts[0] == "dm" && r.Method == http.MethodPost {
		cs.handleCreateDM(w, r)
		return
	}

	// Handle DELETE /api/channels/:id
	if len(parts) == 1 && r.Method == http.MethodDelete {
		channelID := parts[0]
		cs.handleDeleteRoom(w, r, channelID)
		return
	}

	if len(parts) < 2 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
		return
	}

	channelID := parts[0]
	subPath := parts[1]

	// Handle /api/channels/:id/members
	if subPath == "members" && r.Method == http.MethodGet {
		cs.handleGetRoomMembers(w, r, channelID)
		return
	}

	// Handle /api/channels/:id/messages
	if subPath == "messages" && r.Method == http.MethodGet {
		cs.handleGetRoomMessages(w, r, channelID)
		return
	}

	// Handle POST /api/channels/:id/join - Join a public channel
	if subPath == "join" && r.Method == http.MethodPost {
		cs.handleJoinRoom(w, r, channelID)
		return
	}

	// Handle POST /api/channels/:id/leave - Leave a channel
	if subPath == "leave" && r.Method == http.MethodPost {
		cs.handleLeaveChannel(w, r, channelID)
		return
	}

	// Handle GET /api/channels/:id/membership - Check if user is a member
	if subPath == "membership" && r.Method == http.MethodGet {
		cs.handleCheckMembership(w, r, channelID)
		return
	}

	// Handle POST /api/channels/:id/read - Mark all messages in channel as read
	if subPath == "read" && r.Method == http.MethodPost {
		cs.handleMarkChannelRead(w, r, channelID)
		return
	}

	// Default response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

// handleRoomMembers handles POST /api/channels/members for adding members
func (cs *ChatServer) handleRoomMembers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		var req struct {
			ChannelID string `json:"channel_id"`
			ActorID   string `json:"actor_id"`
			Role      string `json:"role"` // "member" or "admin"
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Convert to domain types
		channelID := channel.ChannelID(req.ChannelID)
		actorID := shared.ActorID(req.ActorID)

		// Parse role
		var role channel.MembershipRole
		switch req.Role {
		case "admin":
			role = channel.RoleAdmin
		case "member":
			role = channel.RoleMember
		default:
			http.Error(w, "Invalid role (must be 'member' or 'admin')", http.StatusBadRequest)
			return
		}

		// Get authenticated user (requester) for authorization check
		requesterID := authorization.GetActorID(r.Context())

		// Call service to add member (with authorization)
		addMemberReq := channel.AddMemberRequest{
			ChannelID: channelID,
			ActorID:   actorID,
			Role:      role,
		}
		// System internal operations skip authorization (trusted agent tools)
		if requesterID != "system:internal" {
			addMemberReq.RequesterID = &requesterID // Authorization required
		}

		membership, err := cs.channelService.AddMember(r.Context(), addMemberReq)
		if err != nil {
			getChatLogger().Info("Failed to add member", slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf("Failed to add member: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success response
		response := map[string]interface{}{
			"channel_id": membership.ID.ChannelID.String(),
			"actor_id":   membership.ID.ActorID.String(),
			"role":       membership.Role.String(),
			"joined_at":  membership.JoinedAt.Format(time.RFC3339),
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRoomMembers handles GET /api/channels/:id/members
func (cs *ChatServer) handleGetRoomMembers(w http.ResponseWriter, r *http.Request, channelID string) {
	w.Header().Set("Content-Type", "application/json")

	// AUTHORIZATION CHECK: Only members can see channel members (protects DM privacy)
	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}
	if cs.authzService != nil {
		canRead, err := cs.authzService.CanReadChannel(r.Context(), actorID, channel.ChannelID(channelID))
		if err != nil {
			http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
			return
		}
		if !canRead {
			http.Error(w, `{"error": "Permission denied: not a member of this channel"}`, http.StatusForbidden)
			return
		}
	}

	// Get membership repository via ChannelService
	membershipRepo := cs.channelService.GetMembershipRepository()
	if membershipRepo == nil {
		http.Error(w, "Membership repository not available", http.StatusInternalServerError)
		return
	}

	// Fetch members for this channel
	memberships, err := membershipRepo.ListByChannel(r.Context(), channel.ChannelID(channelID), shared.AllPages())
	if err != nil {
		getChatLogger().Info("Failed to fetch members for channel",
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to fetch members: %v", err), http.StatusInternalServerError)
		return
	}

	// Filter out agent members for non-admin users
	isAdmin := false
	if cs.authzService != nil && actorID != "" {
		isAdmin, _ = cs.authzService.IsWorkspaceAdmin(r.Context(), actorID)
	}
	if !isAdmin {
		filtered := make([]*channel.ChannelMembership, 0, len(memberships))
		for _, m := range memberships {
			if !strings.HasPrefix(m.ID.ActorID.String(), "agent:") {
				filtered = append(filtered, m)
			}
		}
		memberships = filtered
	}

	// PRESENTATION LAYER: Batch load display names for UI enrichment
	displayNames := make(map[string]string) // actorID -> displayName
	if cs.nameEnricher != nil {
		actorIDs := make([]shared.ActorID, 0, len(memberships))
		for _, membership := range memberships {
			actorIDs = append(actorIDs, membership.ID.ActorID)
		}
		displayNames = cs.nameEnricher.LoadBatch(r.Context(), actorIDs)
	}

	// PRESENTATION LAYER: Batch load agent data (avatar_emoji, icon, avatar_url, status) for all agent members
	type agentData struct {
		avatarEmoji string
		icon        string
		avatarURL   string
		status      string
	}
	agentInfo := make(map[string]agentData) // map[actorID]data (with "agent:" prefix)
	if cs.buddyRepo != nil {
		// Collect all agent names
		var agentNames []string
		for _, membership := range memberships {
			actorID := membership.ID.ActorID.String()
			if strings.HasPrefix(actorID, "agent:") {
				agentName := strings.TrimPrefix(actorID, "agent:")
				agentNames = append(agentNames, agentName)
			}
		}

		// BATCH LOAD: Single query for all agents (performance optimization)
		if len(agentNames) > 0 {
			buddies, err := cs.buddyRepo.GetByNames(r.Context(), agentNames)
			if err != nil {
				getChatLogger().Info("Failed to batch load agents", slog.String("error", err.Error()))
			} else {
				// Map buddies by name for quick lookup
				for _, buddy := range buddies {
					status := "offline"
					if buddy.IsActive {
						status = "online"
					}
					agentInfo["agent:"+buddy.Name] = agentData{
						avatarEmoji: buddy.AvatarEmoji,
						avatarURL:   buddy.AvatarURL,
						icon:        buddy.Icon,
						status:      status,
					}
				}
			}
		}
	}

	// PRESENTATION LAYER: Prepare enrichment data maps
	avatarEmojis := make(map[string]string)
	avatarURLs := make(map[string]string)
	icons := make(map[string]string)
	statuses := make(map[string]string)

	// Get real-time presence status from presence tracker
	if cs.presenceTracker != nil {
		actorIDs := make([]shared.ActorID, 0, len(memberships))
		for _, membership := range memberships {
			actorIDs = append(actorIDs, membership.ID.ActorID)
		}
		presenceMap := cs.presenceTracker.GetPresenceMap(actorIDs)
		for actorIDStr, status := range presenceMap {
			statuses[actorIDStr] = status
		}
	}

	for _, membership := range memberships {
		actorID := membership.ID.ActorID.String()

		// Set default values based on actor type
		if strings.HasPrefix(actorID, "human:") {
			avatarEmojis[actorID] = ""
			icons[actorID] = ""
			// Status already set from presence tracker, or defaults to offline if not in map
			if _, ok := statuses[actorID]; !ok {
				statuses[actorID] = "offline"
			}
		} else if strings.HasPrefix(actorID, "agent:") {
			// Load agent avatar from batch-loaded data
			if data, ok := agentInfo[actorID]; ok {
				avatarEmojis[actorID] = data.avatarEmoji
				avatarURLs[actorID] = data.avatarURL
				icons[actorID] = data.icon
				// Status from presence tracker takes precedence over buddy.IsActive
				if _, ok := statuses[actorID]; !ok {
					statuses[actorID] = "offline"
				}
			} else {
				avatarEmojis[actorID] = "🤖" // Fallback emoji
				icons[actorID] = ""
				if _, ok := statuses[actorID]; !ok {
					statuses[actorID] = "offline"
				}
			}
		}
	}

	// PRESENTATION LAYER: Convert domain memberships to DTOs
	memberDTOs := dto.ToChannelMemberDTOBatch(memberships, displayNames, avatarEmojis, icons, statuses)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"members": memberDTOs,
	})
}

// handleGetRoomMessages handles GET /api/channels/:id/messages
func (cs *ChatServer) handleGetRoomMessages(w http.ResponseWriter, r *http.Request, channelID string) {
	w.Header().Set("Content-Type", "application/json")

	// Check if MessageService is available
	if cs.messageService == nil {
		http.Error(w, "Message service not available", http.StatusInternalServerError)
		return
	}

	// AUTHORIZATION CHECK: Verify user can read from this channel
	// Get authenticated user from context
	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// Check if user has read permission (is a member)
	if cs.authzService != nil {
		canRead, err := cs.authzService.CanReadChannel(r.Context(), actorID, channel.ChannelID(channelID))
		if err != nil {
			getChatLogger().Error("Failed to check read permission",
				slog.String("actor_id", actorID.String()),
				slog.String("channel_id", channelID),
				slog.String("error", err.Error()))
			http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
			return
		}
		if !canRead {
			http.Error(w, `{"error": "Permission denied: not a member of this channel"}`, http.StatusForbidden)
			return
		}
	}

	// Parse cursor pagination params
	var beforeID int64
	if beforeStr := r.URL.Query().Get("before"); beforeStr != "" {
		fmt.Sscanf(beforeStr, "%d", &beforeID)
	}
	var limit int
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	page := shared.NewCursorPage(beforeID, limit)

	// Fetch messages from MessageService (DDD layer)
	result, err := cs.messageService.ListMessages(r.Context(), channelID, page)
	if err != nil {
		getChatLogger().Info("Failed to fetch messages for channel",
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to fetch messages: %v", err), http.StatusInternalServerError)
		return
	}
	messages := result.Messages

	// Extract message IDs for bulk reaction loading
	messageIDs := make([]message.MessageID, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.ID
	}

	// Bulk-load reactions for all messages
	reactionsByMessage := make(map[message.MessageID][]*reaction.Reaction)
	if len(messageIDs) > 0 && cs.reactionRepo != nil {
		var err error
		reactionsByMessage, err = cs.reactionRepo.ListByMessages(r.Context(), messageIDs)
		if err != nil {
			getChatLogger().Info("Failed to load reactions", slog.String("error", err.Error()))
		}
	}

	// PRESENTATION LAYER: Batch-load file attachments
	fileAttachmentsByMsg := make(map[int64][]dto.FileRefDTO)
	if cs.fileService != nil && len(messages) > 0 {
		msgIDs := make([]int64, len(messages))
		for i, msg := range messages {
			msgIDs[i] = int64(msg.ID)
		}
		if attachments, err := cs.fileService.GetAttachmentsBatch(r.Context(), msgIDs); err == nil {
			for msgID, files := range attachments {
				for _, f := range files {
					fileAttachmentsByMsg[msgID] = append(fileAttachmentsByMsg[msgID], dto.FileRefDTO{
						ID:        f.ID,
						Filename:  f.Filename,
						MimeType:  f.MimeType,
						SizeBytes: f.SizeBytes,
						URL:       fmt.Sprintf("/api/files/%s", f.ID),
					})
				}
			}
		}
	}

	messageDTOs := cs.enrichAndConvertBatch(r.Context(), messages, reactionsByMessage)
	// Attach file refs to DTOs
	for i, msgDTO := range messageDTOs {
		msgID, _ := strconv.ParseInt(msgDTO.ID, 10, 64)
		if files, ok := fileAttachmentsByMsg[msgID]; ok {
			messageDTOs[i].Content.Attachments = files
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messageDTOs,
		"has_more": result.HasMore,
	})

	getChatLogger().Info("Returned messages for channel",
		slog.Int("count", len(messages)),
		slog.String("channel_id", channelID))
}

// handleJoinRoom handles POST /api/channels/:id/join
// Allows authenticated users to join public channels
func (cs *ChatServer) handleJoinRoom(w http.ResponseWriter, r *http.Request, channelIDStr string) {
	w.Header().Set("Content-Type", "application/json")

	// FAIL FAST: Check dependencies
	if cs.authzService == nil {
		http.Error(w, `{"error": "Authorization service not initialized"}`, http.StatusInternalServerError)
		return
	}
	if cs.channelRepo == nil {
		http.Error(w, `{"error": "Channel repository not initialized"}`, http.StatusInternalServerError)
		return
	}
	if cs.channelService == nil {
		http.Error(w, `{"error": "Channel service not initialized"}`, http.StatusInternalServerError)
		return
	}

	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// Get channel details
	channelID := channel.ChannelID(channelIDStr)
	channelUUID, err := uuid.Parse(channelIDStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid channel ID"}`, http.StatusBadRequest)
		return
	}

	ch, err := cs.channelRepo.GetByID(r.Context(), channelUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Channel not found: %s"}`, err.Error()), http.StatusNotFound)
		return
	}

	// Check if channel is public
	if ch.IsPrivate {
		http.Error(w, `{"error": "Cannot join private channel - invitation required"}`, http.StatusForbidden)
		return
	}

	// Check if already a member (requires write permission = actual membership)
	isMember, err := cs.authzService.CanWriteToChannel(r.Context(), actorID, channelID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to check membership: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	getChatLogger().Info("handleJoinRoom: Membership check complete",
		slog.String("channel_id", channelIDStr),
		slog.String("actor_id", actorID.String()),
		slog.Bool("is_member", isMember))

	if isMember {
		getChatLogger().Info("handleJoinRoom: User already a member",
			slog.String("channel_id", channelIDStr),
			slog.String("actor_id", actorID.String()))
		// Already a member - return success
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "already_member",
			"message": "You are already a member of this channel",
		})
		return
	}

	getChatLogger().Info("handleJoinRoom: Adding user as member",
		slog.String("channel_id", channelIDStr),
		slog.String("actor_id", actorID.String()))

	// Add user as member (self-join for public channels)
	// For public channels: RequesterID = actorID (self-join allowed)
	// For private channels: authorization check will verify invitation
	addMemberReq := channel.AddMemberRequest{
		ChannelID:   channelID,
		ActorID:     actorID,
		RequesterID: &actorID,           // Self-join (allowed for public channels, checked for private)
		Role:        channel.RoleMember, // Default to member role
	}

	getChatLogger().Info("handleJoinRoom: Calling channelService.AddMember",
		slog.String("channel_id", channelIDStr),
		slog.String("actor_id", actorID.String()),
		slog.String("role", channel.RoleMember.String()))

	membership, err := cs.channelService.AddMember(r.Context(), addMemberReq)
	if err != nil {
		getChatLogger().Error("handleJoinRoom: Failed to add member",
			slog.String("error", err.Error()),
			slog.String("actor_id", actorID.String()),
			slog.String("channel_id", channelIDStr))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to join channel: %s", err.Error()),
		})
		return
	}

	getChatLogger().Info("handleJoinRoom: User successfully joined channel",
		slog.String("actor_id", actorID.String()),
		slog.String("channel_id", channelIDStr),
		slog.String("role", membership.Role.String()))

	// Return success response
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "joined",
		"channel_id": membership.ID.ChannelID.String(),
		"actor_id":   membership.ID.ActorID.String(),
		"role":       membership.Role.String(),
		"joined_at":  membership.JoinedAt.Format(time.RFC3339),
	})
}

// handleLeaveChannel handles POST /api/channels/:id/leave
// Allows authenticated users to leave channels (self-removal)
func (cs *ChatServer) handleLeaveChannel(w http.ResponseWriter, r *http.Request, channelIDStr string) {
	w.Header().Set("Content-Type", "application/json")

	// FAIL FAST: Check dependencies
	if cs.channelService == nil {
		http.Error(w, `{"error": "Channel service not initialized"}`, http.StatusInternalServerError)
		return
	}

	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// Parse channel ID
	channelID := channel.ChannelID(channelIDStr)
	_, err := uuid.Parse(channelIDStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid channel ID"}`, http.StatusBadRequest)
		return
	}

	// Call ChannelService.RemoveMember with self-removal pattern
	// RequesterID = actorID (self-removal, automatically allowed)
	err = cs.channelService.RemoveMember(r.Context(), channelID, actorID, &actorID)
	if err != nil {
		getChatLogger().Error("handleLeaveChannel: Failed to remove member",
			slog.String("error", err.Error()),
			slog.String("actor_id", actorID.String()),
			slog.String("channel_id", channelIDStr))

		// Check for specific error messages
		if err.Error() == "cannot remove last admin from channel" {
			http.Error(w, `{"error": "Cannot leave channel: you are the last admin. Please promote another member to admin first."}`, http.StatusForbidden)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Failed to leave channel: %s", err.Error()),
		})
		return
	}

	getChatLogger().Info("handleLeaveChannel: User successfully left channel",
		slog.String("actor_id", actorID.String()),
		slog.String("channel_id", channelIDStr))

	// Return success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "left",
		"channel_id": channelIDStr,
		"actor_id":   actorID.String(),
	})
}

// handleCheckMembership handles GET /api/channels/:id/membership
// Returns whether the authenticated user is a member of the channel
func (cs *ChatServer) handleCheckMembership(w http.ResponseWriter, r *http.Request, channelIDStr string) {
	w.Header().Set("Content-Type", "application/json")

	// FAIL FAST: Check dependencies
	if cs.authzService == nil {
		http.Error(w, `{"error": "Authorization service not initialized"}`, http.StatusInternalServerError)
		return
	}
	if cs.channelRepo == nil {
		http.Error(w, `{"error": "Channel repository not initialized"}`, http.StatusInternalServerError)
		return
	}

	// Get authenticated user from context
	actorID := authorization.GetActorID(r.Context())

	if actorID == "" {
		// Not authenticated - not a member
		json.NewEncoder(w).Encode(map[string]interface{}{
			"is_member": false,
			"can_join":  false,
		})
		return
	}

	channelID := channel.ChannelID(channelIDStr)

	// Check if channel exists
	channelUUID, err := uuid.Parse(channelIDStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid channel ID"}`, http.StatusBadRequest)
		return
	}

	ch, err := cs.channelRepo.GetByID(r.Context(), channelUUID)
	if err != nil {
		http.Error(w, `{"error": "Channel not found"}`, http.StatusNotFound)
		return
	}

	// Check if user is a member (has write permission)
	isMember, err := cs.authzService.CanWriteToChannel(r.Context(), actorID, channelID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to check membership: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Check if user can join (channel is public and not a member)
	canJoin := !ch.IsPrivate && !isMember

	json.NewEncoder(w).Encode(map[string]interface{}{
		"is_member": isMember,
		"can_join":  canJoin,
		"is_public": !ch.IsPrivate,
	})
}

// handleDeleteRoom handles DELETE /api/channels/:id
func (cs *ChatServer) handleDeleteRoom(w http.ResponseWriter, r *http.Request, channelIDStr string) {
	w.Header().Set("Content-Type", "application/json")

	actorID, ok := requireAuth(w, r)
	if !ok {
		return
	}

	// AUTHORIZATION: Require workspace admin
	if cs.authzService != nil {
		isAdmin, err := cs.authzService.IsWorkspaceAdmin(r.Context(), actorID)
		if err != nil {
			http.Error(w, `{"error": "Failed to check permissions"}`, http.StatusInternalServerError)
			return
		}
		if !isAdmin {
			http.Error(w, `{"error": "Permission denied: workspace admin required"}`, http.StatusForbidden)
			return
		}
	}

	// Check if repository is available
	if cs.channelRepo == nil {
		http.Error(w, "Channel repository not available", http.StatusInternalServerError)
		return
	}

	// Parse channel ID
	channelID, err := uuid.Parse(channelIDStr)
	if err != nil {
		http.Error(w, "Invalid channel ID", http.StatusBadRequest)
		return
	}

	// Delete the channel
	if err := cs.channelRepo.Delete(r.Context(), channelID); err != nil {
		getChatLogger().Info("Failed to delete channel", slog.String("channel_id", channelID.String()), slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf("Failed to delete channel: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "deleted",
		"channel_id": channelIDStr,
	})

	getChatLogger().Info("Deleted channel", slog.String("channel_id", channelID.String()))
}

// handleCreateDM handles POST /api/channels/dm for creating or finding a DM channel
func (cs *ChatServer) handleCreateDM(w http.ResponseWriter, r *http.Request) {
	getChatLogger().Info("handleCreateDM called", slog.String("method", r.Method))
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		getChatLogger().Info("Wrong method", slog.String("method", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user from context
	currentUserID := authorization.GetActorID(r.Context())
	getChatLogger().Info("Auth check", slog.String("current_user_id", string(currentUserID)))
	if currentUserID == "" {
		getChatLogger().Info("❌ No authentication - returning 401")
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return
	}

	// Parse request
	var req struct {
		OtherUserID string `json:"other_user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.OtherUserID == "" {
		http.Error(w, `{"error": "other_user_id is required"}`, http.StatusBadRequest)
		return
	}

	otherUserID := shared.ActorID(req.OtherUserID)

	getChatLogger().Info("Creating DM",
		slog.String("current_user", string(currentUserID)),
		slog.String("other_user", string(otherUserID)))

	// Create DM name with sorted IDs for consistency (so "dm-A-B" == "dm-B-A")
	users := []string{string(currentUserID), string(otherUserID)}
	if users[0] > users[1] {
		users[0], users[1] = users[1], users[0]
	}
	dmName := fmt.Sprintf("dm-%s-%s", users[0], users[1])
	getChatLogger().Info("DM name", slog.String("dm_name", dmName))

	// Check if DM already exists by name
	existingDM, err := cs.channelRepo.GetByName(r.Context(), dmName)
	if err == nil && existingDM != nil {
		channelDTO := dto.ToChannelDTO(existingDM)
		json.NewEncoder(w).Encode(channelDTO)
		return
	}

	// Create new DM channel
	now := time.Now()
	dmChannel := &domain.Channel{
		ID:          uuid.New(),
		Name:        dmName,
		Description: nil, // No description for DMs
		IsPrivate:   true,
		WorkspaceScoped: domain.WorkspaceScoped{
			WorkspaceID: uuid.Nil,
		},
		Timestamps: domain.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
		Archivable: domain.Archivable{
			ArchivedAt: nil,
		},
	}

	if err := cs.channelRepo.Create(r.Context(), dmChannel); err != nil {
		// Duplicate — race condition. Retry lookup.
		if existing, retryErr := cs.channelRepo.GetByName(r.Context(), dmName); retryErr == nil && existing != nil {
			channelDTO := dto.ToChannelDTO(existing)
			json.NewEncoder(w).Encode(channelDTO)
			return
		}
		http.Error(w, `{"error": "Failed to create DM"}`, http.StatusInternalServerError)
		return
	}

	getChatLogger().Info("DM channel created successfully", slog.String("channel_id", dmChannel.ID.String()))

	// Deduplicate user IDs (for self-DM case where both users are the same)
	userIDs := []shared.ActorID{currentUserID}
	if currentUserID != otherUserID {
		userIDs = append(userIDs, otherUserID)
	}

	getChatLogger().Info("Adding members to DM",
		slog.Int("count", len(userIDs)),
		slog.String("user_ids", fmt.Sprintf("%v", userIDs)))

	// Add users as members
	// DM creation is a system operation - bypass authorization
	for _, userID := range userIDs {
		getChatLogger().Info("Attempting to add user to DM",
			slog.String("user_id", userID.String()),
			slog.String("dm_id", dmChannel.ID.String()))
		addMemberReq := channel.AddMemberRequest{
			ChannelID:   channel.ChannelID(dmChannel.ID.String()),
			ActorID:     userID,
			RequesterID: nil,                // System operation - bypass authorization
			Role:        channel.RoleMember, // Both users are regular members in DM
		}

		membership, err := cs.channelService.AddMember(r.Context(), addMemberReq)
		if err != nil {
			getChatLogger().Info("FAILED to add user to DM",
				slog.String("user_id", userID.String()),
				slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf(`{"error": "Failed to add member to DM: %s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		getChatLogger().Info("Successfully added user to DM",
			slog.String("user_id", userID.String()),
			slog.String("role", membership.Role.String()))
	}

	getChatLogger().Info("Created new DM",
		slog.String("user1", string(currentUserID)),
		slog.String("user2", string(otherUserID)),
		slog.String("dm_id", dmChannel.ID.String()))

	// PRESENTATION LAYER: Return the new DM as DTO
	channelDTO := dto.ToChannelDTO(dmChannel)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(channelDTO)
}
