package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/pkg/domain"
	"net/http"
	"time"

	"memdoor/gateway/broadcast"
	"memdoor/gateway/dto"
	"memdoor/gateway/logs"
	"memdoor/gateway/presence"
	"memdoor/gateway/queue"
	"memdoor/gateway/websocket"
	"memdoor/pkg/authorization"
	"memdoor/pkg/channel"
	"memdoor/pkg/filestore"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/repository"
	"memdoor/pkg/shared"
)

var chatLog *logs.EventLogger

func getChatLogger() *logs.EventLogger {
	if chatLog == nil {
		chatLog = logs.New("HTTP")
	}
	return chatLog
}

// hubBroadcaster adapts websocket.Hub to presence.Broadcaster interface
type hubBroadcaster struct {
	hub *websocket.Hub
}

func (h *hubBroadcaster) BroadcastToAll(event broadcast.Event) {
	// Convert broadcast event to WebSocket message
	data, _ := json.Marshal(event.Data)
	wsMessage := websocket.WSMessage{
		Type: websocket.PresenceUpdated,
		Data: data,
	}
	h.hub.BroadcastToAll(wsMessage)
}

// AgentMentionHandler is a duck-typed interface for triggering agent mentions
// Pattern: Duck typing to avoid importing concrete types from pkg/message
// Implementation: pkg/message.Service.HandleAgentMentions
type AgentMentionHandler interface {
	HandleAgentMentions(ctx context.Context, msg *message.Message)
}

// statsCache holds cached platform statistics with TTL
type statsCache struct {
	ttl time.Duration
}

// newStatsCache creates a new stats cache with specified TTL
func newStatsCache(ttl time.Duration) *statsCache {
	return &statsCache{
		ttl: ttl,
	}
}

// ChatServer handles HTTP REST API and WebSocket for the chat interface
type ChatServer struct {
	hub          *websocket.Hub
	agentRuntime *AgentRuntime       // AI agent runtime for execution
	queueManager *queue.QueueManager // Queue manager for per-session serialization
	buddyRepo    repository.BuddyRepository
	channelRepo  repository.ChannelRepository
	reactionRepo reaction.Repository
	cronRepo     repository.CronJobRepository // Cron job repository

	// onCronJobAdded / onCronJobRemoved tell the RUNNING scheduler about jobs
	// created or deleted at runtime. The scheduler is owned by the gateway
	// server, and this package does not import it — the owner sets these.
	//
	// Without them a job was stored, listed, and never fired: the scheduler
	// registers jobs once at start-up (2026-08-24).
	// The scheduler is the SERVICE: it persists and schedules together, and
	// rolls the store back if scheduling fails. Delegating to it keeps ONE
	// write path — the scheduler's store is backed by the same repository, so
	// writing here as well would persist the job twice.
	onCronJobAdded   func(*domain.CronJob) error
	onCronJobRemoved func(string) error
	cronHistoryRepo  repository.CronHistoryRepository // Cron execution history
	agentSecretRepo  repository.AgentSecretRepository // Per-agent scoped secrets
	channelService   *channel.ChannelService
	messageService   *message.Service             // DDD service layer (handles validation + persistence + agent execution)
	authzService     *authorization.Service       // Authorization service for permission checks
	nameEnricher     *dto.DisplayNameEnricher     // Presentation layer: enriches domain models with UI display names
	repoFactory      repository.RepositoryFactory // Repository factory for DisplayNameEnricher initialization
	presenceTracker  *presence.Tracker            // Real-time presence tracking for online/offline status
	statsCache       *statsCache                  // In-memory cache for /api/stats endpoint (landing page performance)
	fileService      *filestore.Service           // File sharing service
	tokenVerifier    authorization.AuthService    // Token verifier for file download auth via query param
}

// NewChatServer creates a new chat server
func NewChatServer(
	buddyRepo repository.BuddyRepository,
	channelRepo repository.ChannelRepository,
	reactionRepo reaction.Repository,
	cronRepo repository.CronJobRepository,
	cronHistoryRepo repository.CronHistoryRepository,
	agentSecretRepo repository.AgentSecretRepository,
	channelService *channel.ChannelService,
	messageService *message.Service,
	authzService *authorization.Service,
	agentRuntime *AgentRuntime,
	queueManager *queue.QueueManager,
	repoFactory repository.RepositoryFactory,
) *ChatServer {
	hub := websocket.NewHub()

	chatServer := &ChatServer{
		hub:             hub,
		buddyRepo:       buddyRepo,
		channelRepo:     channelRepo,
		reactionRepo:    reactionRepo,
		cronRepo:        cronRepo,
		cronHistoryRepo: cronHistoryRepo,
		agentSecretRepo: agentSecretRepo,
		channelService:  channelService,
		messageService:  messageService,
		authzService:    authzService,
		agentRuntime:    agentRuntime,
		queueManager:    queueManager,
		repoFactory:     repoFactory,
		statsCache:      newStatsCache(30 * time.Second), // Cache stats for 30 seconds (landing page performance)
	}

	// Initialize presence tracker with Hub as broadcaster
	chatServer.presenceTracker = presence.NewTracker(&hubBroadcaster{hub: hub})
	hub.SetPresenceTracker(chatServer.presenceTracker)
	getChatLogger().Info("PresenceTracker initialized and wired to Hub")

	// Initialize presentation layer enricher with database access
	if repoFactory != nil {
		if db, ok := repoFactory.DB().(*sql.DB); ok && db != nil {
			chatServer.nameEnricher = dto.NewDisplayNameEnricher(db, buddyRepo, true)
			getChatLogger().Info("DisplayNameEnricher initialized (presentation layer, caching enabled)")
		}
	}

	// The handlers are registered on the gateway's main mux by the server
	// lifecycle, behind its auth middleware. ChatServer owns no listener.
	return chatServer
}

// GetHub returns the WebSocket hub
func (cs *ChatServer) GetHub() *websocket.Hub {
	return cs.hub
}

// CreateBroadcaster creates a MessageBroadcaster callback for the MessageService.
// This bridges the domain layer (MessageService) to presentation layer (WebSocket Hub + DTO).
func (cs *ChatServer) CreateBroadcaster() message.MessageBroadcaster {
	return func(msg *message.Message) {
		ctx := context.Background()

		messageDTO := cs.enrichAndConvert(ctx, msg, nil)

		// Build the WSMessage for channel broadcast
		channelWSMsg := websocket.WSMessage{
			Type: websocket.MessageCreated,
			Data: mustMarshal(messageDTO),
		}

		// Collect mentioned user IDs (excluding author)
		var mentionUserIDs []string
		if len(msg.Content.Mentions) > 0 {
			for _, mention := range msg.Content.Mentions {
				if mention.ActorID != msg.AuthorID {
					mentionUserIDs = append(mentionUserIDs, mention.ActorID.String())
				}
			}
		}

		// Broadcast directly to the local WebSocket hub.
		cs.hub.BroadcastToChannel(msg.ChannelID, channelWSMsg)

		if len(mentionUserIDs) > 0 {
			mentionEvent := websocket.WSMessage{
				Type: websocket.MentionCreated,
				Data: mustMarshal(messageDTO),
			}
			for _, userID := range mentionUserIDs {
				cs.hub.BroadcastToUser(userID, mentionEvent)
			}
		}

		getChatLogger().Info("Broadcast message to channel",
			slog.Int64("message_id", int64(msg.ID)),
			slog.String("author_id", msg.AuthorID.String()),
			slog.String("channel_id", msg.ChannelID))
	}
}

// CreateExecutionBroadcaster creates an ExecutionEventBroadcaster callback for the MessageService
// This bridges the domain layer (MessageService) to infrastructure layer (WebSocket Hub) for execution events
func (cs *ChatServer) CreateExecutionBroadcaster() message.ExecutionEventBroadcaster {
	return func(channelID string, agentID shared.ActorID, runID string, status string, data map[string]interface{}) {
		// Build event data
		eventData := map[string]interface{}{
			"channel_id":   channelID,
			"agent_id":     agentID.String(),
			"run_id":       runID,
			"status":       status,
			"execution_id": runID, // For backward compatibility
		}

		// Merge additional data if provided
		if data != nil {
			eventData["data"] = data
		}

		// Determine WebSocket message type based on status
		var messageType string
		switch status {
		case "started":
			messageType = "execution.started"
		case "failed":
			messageType = "execution.failed"
		case "completed":
			messageType = "execution.completed"
		default:
			messageType = "execution.event"
		}

		// Broadcast to channel
		cs.hub.BroadcastToChannel(channelID, websocket.WSMessage{
			Type: websocket.MessageType(messageType),
			Data: mustMarshal(eventData),
		})

		getChatLogger().Info("Broadcast execution event",
			slog.String("status", status),
			slog.String("agent_id", agentID.String()),
			slog.String("run_id", runID),
			slog.String("channel_id", channelID))
	}
}

// CreateLinkPreviewEnricher creates a LinkPreviewEnricher callback for the MessageService
// This bridges the domain layer (MessageService) to presentation layer (DTO enrichment)
// Pattern: Dependency Inversion - domain calls back to gateway for enrichment before persistence
func (cs *ChatServer) CreateLinkPreviewEnricher() message.LinkPreviewEnricher {
	return func(ctx context.Context, previews []message.LinkPreview) []message.LinkPreview {
		// PRESENTATION LAYER: Enrich link preview names before domain saves to database
		return dto.EnrichLinkPreviewNames(
			ctx,
			previews,
			cs.nameEnricher,
			cs.channelRepo,
		)
	}
}

// requireAuth extracts the authenticated actor from the request context.
// Returns the actor ID and true if authenticated, or writes a 401 and returns false.
func requireAuth(w http.ResponseWriter, r *http.Request) (shared.ActorID, bool) {
	actorID := authorization.GetActorID(r.Context())
	if actorID == "" {
		http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
		return "", false
	}
	return actorID, true
}

// enrichAndConvert enriches a single domain message with display names and link previews,
// then converts it to a DTO for API responses. Consolidates the repeated enrichment pattern.
func (cs *ChatServer) enrichAndConvert(ctx context.Context, msg *message.Message, reactions []*reaction.Reaction, fileDTOs ...dto.FileRefDTO) dto.MessageDTO {
	if len(msg.Content.LinkPreviews) > 0 {
		msg.Content.LinkPreviews = dto.EnrichLinkPreviewNames(
			ctx,
			msg.Content.LinkPreviews,
			cs.nameEnricher,
			cs.channelRepo,
		)
	}

	authorName := ""
	if cs.nameEnricher != nil {
		authorName = cs.nameEnricher.LoadSingle(ctx, msg.AuthorID)
	} else {
		authorName = msg.AuthorID.String()
	}

	return dto.ToMessageDTO(msg, authorName, reactions, fileDTOs...)
}

// enrichAndConvertBatch enriches a slice of domain messages with display names and link previews,
// then converts them to DTOs for API responses.
func (cs *ChatServer) enrichAndConvertBatch(ctx context.Context, msgs []*message.Message, reactionsByMessage map[message.MessageID][]*reaction.Reaction) []dto.MessageDTO {
	// Batch-load display names
	displayNames := make(map[string]string)
	if cs.nameEnricher != nil {
		authorIDs := make([]shared.ActorID, 0, len(msgs))
		for _, msg := range msgs {
			authorIDs = append(authorIDs, msg.AuthorID)
		}
		displayNames = cs.nameEnricher.LoadBatch(ctx, authorIDs)
	}

	// Enrich link previews
	for _, msg := range msgs {
		if len(msg.Content.LinkPreviews) > 0 {
			msg.Content.LinkPreviews = dto.EnrichLinkPreviewNames(
				ctx,
				msg.Content.LinkPreviews,
				cs.nameEnricher,
				cs.channelRepo,
			)
		}
	}

	return dto.ToMessageDTOBatch(msgs, displayNames, reactionsByMessage)
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		getChatLogger().Error("Failed to marshal data, returning empty object",
			slog.String("error", err.Error()),
			slog.String("type", fmt.Sprintf("%T", v)))
		return json.RawMessage("{}")
	}
	return data
}
