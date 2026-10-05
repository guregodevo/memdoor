package message

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"memdoor/pkg/channel"
	"memdoor/pkg/domain"
	"memdoor/pkg/platform"
	"memdoor/pkg/prompts"
	"memdoor/pkg/sandbox"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"

	"github.com/google/uuid"
)

// Logger is the duck-typed interface for structured logging.
// Satisfied by both gateway/logs.EventLogger and *slog.Logger.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

var logger Logger

var defaultUserID = uuid.MustParse("00000000-0000-0000-0000-000000000010")

func parseUserIDOrDefault(id string) uuid.UUID {
	if id == "" {
		logger.Warn("Empty ActorID used for sandbox context, using default UUID")
		return defaultUserID
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		logger.Debug("Non-UUID ActorID used for sandbox context, using default",
			slog.String("actor_id", id))
		return defaultUserID
	}
	return parsed
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BuddyRepository defines the minimal interface needed for looking up agent sandbox scopes
// This avoids circular dependency with pkg/repository
type BuddyRepository interface {
	GetByName(ctx context.Context, name string) (*domain.Buddy, error)
}

// ChannelRepository defines the minimal interface needed for fetching channel workspace IDs
type ChannelRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Channel, error)
	// GetWorkspaceSlug — see pkg/repository/interfaces.go for the full
	// rationale. Used to backfill SandboxContext.WorkspaceSlug on the A2A
	// path where ExecutionContext arrives without a slug.
	GetWorkspaceSlug(ctx context.Context, workspaceID uuid.UUID) (string, error)
}

// MessageBroadcaster is a callback for broadcasting messages to connected clients
// This allows the infrastructure layer (WebSocket) to be notified of new messages
// while keeping the domain layer clean and decoupled
type MessageBroadcaster func(msg *Message)

// ExecutionEventBroadcaster is a callback for broadcasting agent execution events
type ExecutionEventBroadcaster func(channelID string, agentID shared.ActorID, runID string, status string, data map[string]interface{})

// MessageIndexer is a callback for indexing messages for search
// Pattern: Published Interface - allows search infrastructure to subscribe to message creation events
type MessageIndexer func(msg *Message)

// AgentSecretCounter checks if an agent has secrets configured
type AgentSecretCounter interface {
	Count(ctx context.Context, agentID string) (int, error)
}

// WorkspaceLanguageResolver resolves the language for a workspace.
// Single source of truth for workspace language (from DB, not layout files).
type WorkspaceLanguageResolver interface {
	GetLanguage(ctx context.Context) string
}

// LinkPreviewEnricher is a callback for enriching link preview names (author, channel)
// Pattern: Dependency Inversion - domain layer calls back to gateway for presentation concerns
// This allows the domain to save enriched data without depending on presentation layer
type LinkPreviewEnricher func(ctx context.Context, previews []LinkPreview) []LinkPreview

// Service coordinates message operations across multiple aggregates
//
// Responsibilities:
// - Validate business rules (author is member, agents exist, etc.)
// - Coordinate message creation and persistence
// - Trigger agent executions for @mentions
// - Save agent responses as messages
type Service struct {
	logger               Logger
	messageRepo          Repository
	membershipRepo       channel.MembershipRepository
	channelRepo          ChannelRepository // For fetching workspace IDs
	buddyRepo            BuddyRepository   // For looking up agent sandbox scopes
	authRepo             AuthRepository    // For resolving email→ActorID for human mentions
	agentExecutor        platform.AgentExecutor
	remoteExecutor       platform.RemoteAgentExecutor // For remote agents (OpenAI-compatible endpoints)
	turnLedger           TurnLedger                   // Turn reliability ledger (duck-typed; optional)
	turnGate             *turnGate                    // One turn at a time per (channel, agent) — see turn_gate.go
	broadcaster          MessageBroadcaster           // Optional callback for real-time broadcasts
	executionBroadcaster ExecutionEventBroadcaster    // Optional callback for execution event broadcasts
	messageIndexer       MessageIndexer               // Optional callback for indexing messages for search
	linkPreviewEnricher  LinkPreviewEnricher          // Optional callback for enriching link preview names before persistence
	agentSecretCounter   AgentSecretCounter           // Optional: checks if agent has secrets for auto-injecting get_secret tool
	remoteToolExecutor   platform.RemoteToolExecutor  // Optional: tool dispatch for remote agents
	languageResolver     WorkspaceLanguageResolver    // Optional: resolves workspace language from DB
	messageGate          MessageGate                  // Optional: lets an unaddressed message reach an agent (decision_gate.go)
}

// NewService creates a new message service
func NewService(
	log Logger,
	messageRepo Repository,
	membershipRepo channel.MembershipRepository,
	channelRepo ChannelRepository,
	buddyRepo BuddyRepository,
	authRepo AuthRepository,
	agentExecutor platform.AgentExecutor,
	remoteExecutor platform.RemoteAgentExecutor,
) *Service {
	logger = log
	return &Service{
		logger:         log,
		messageRepo:    messageRepo,
		membershipRepo: membershipRepo,
		channelRepo:    channelRepo,
		buddyRepo:      buddyRepo,
		authRepo:       authRepo,
		agentExecutor:  agentExecutor,
		remoteExecutor: remoteExecutor,
		turnGate:       newTurnGate(),
	}
}

// SetBroadcaster sets the callback for broadcasting messages to connected clients
func (s *Service) SetBroadcaster(broadcaster MessageBroadcaster) {
	s.broadcaster = broadcaster
}

// SetExecutionBroadcaster sets the callback for broadcasting agent execution events
func (s *Service) SetExecutionBroadcaster(broadcaster ExecutionEventBroadcaster) {
	s.executionBroadcaster = broadcaster
}

// SetMessageIndexer sets the callback for indexing messages for search
// Pattern: Published Interface - allows search infrastructure to subscribe to message creation
func (s *Service) SetMessageIndexer(indexer MessageIndexer) {
	s.messageIndexer = indexer
}

// SetLinkPreviewEnricher sets the callback for enriching link preview names before persistence
// Pattern: Dependency Inversion - allows domain to save enriched data without depending on presentation layer
func (s *Service) SetLinkPreviewEnricher(enricher LinkPreviewEnricher) {
	s.linkPreviewEnricher = enricher
}

// SetAgentSecretCounter sets the counter for auto-injecting get_secret tool
func (s *Service) SetAgentSecretCounter(counter AgentSecretCounter) {
	s.agentSecretCounter = counter
}

// SetRemoteToolExecutor sets the tool dispatcher for remote agents
func (s *Service) SetRemoteToolExecutor(executor platform.RemoteToolExecutor) {
	s.remoteToolExecutor = executor
}

func (s *Service) SetLanguageResolver(resolver WorkspaceLanguageResolver) {
	s.languageResolver = resolver
}

// GetRepository returns the message repository (for direct database operations)
func (s *Service) GetRepository() Repository {
	return s.messageRepo
}

// PostMessageRequest encapsulates parameters for posting a message
type PostMessageRequest struct {
	ChannelID           string
	AuthorID            shared.ActorID
	Content             MessageContent
	SkipMembershipCheck bool   // Trusted internal callers (system:internal) bypass membership validation
	PermissionMode      string // Claude-style interaction mode for the turn: "default"|"acceptEdits"|"plan" (empty = default)
	Workdir             string // Client's launch directory — the coder's confinement roots here (Claude-CLI semantics); empty = server default
}

// resolveMentions parses @mentions from text and resolves them to ActorIDs
// This is domain logic: name resolution belongs in the service layer, not the gateway
func (s *Service) resolveMentions(ctx context.Context, text string) []Mention {
	mentionNames := ParseMentions(text)
	mentions := make([]Mention, 0, len(mentionNames))

	for _, name := range mentionNames {
		var actorID shared.ActorID
		var displayName string

		// Try agent/buddy first
		buddy, err := s.buddyRepo.GetByName(ctx, name)
		if err == nil && buddy != nil {
			actorID = shared.ActorID(fmt.Sprintf("agent:%s", buddy.Name))
			displayName = buddy.Name
		} else {
			// Try username
			user, err := s.authRepo.GetUserByUsername(ctx, name)
			if err == nil && user != nil {
				actorID = shared.ActorID(user.GetID())
				displayName = user.GetUsername()
			} else if strings.Contains(name, "@") {
				// Backward compatibility: email format
				user, _, err := s.authRepo.GetUserByEmail(ctx, name)
				if err == nil && user != nil {
					actorID = shared.ActorID(user.GetID())
					displayName = user.GetUsername()
				}
			}
		}

		if actorID != "" {
			mentions = append(mentions, Mention{ActorID: actorID, Name: displayName})
		}
	}

	return mentions
}

// addDMAutoMentions adds agent mentions for DM channels when no explicit mentions exist
// In DM channels, every message should trigger agent responses (like Slack DMs)
func (s *Service) addDMAutoMentions(ctx context.Context, channelID string, authorID shared.ActorID) []Mention {
	channelUUID, err := uuid.Parse(channelID)
	if err != nil {
		return nil
	}

	ch, err := s.channelRepo.GetByID(ctx, channelUUID)
	if err != nil || !strings.HasPrefix(ch.Name, "dm-") {
		return nil
	}

	members, err := s.membershipRepo.ListByChannel(ctx, channel.ChannelID(channelID), shared.AllPages())
	if err != nil {
		return nil
	}

	var mentions []Mention
	for _, member := range members {
		if member.ID.ActorID.IsAgent() && member.ID.ActorID != authorID {
			mentions = append(mentions, Mention{
				ActorID: member.ID.ActorID,
				Name:    member.ID.ActorID.ID(),
			})
			logger.Info("DM auto-mention: added agent",
				slog.String("agent_id", member.ID.ActorID.String()),
				slog.String("channel_id", channelID))
		}
	}

	return mentions
}

// PostMessage creates and saves a new message with business rule validation
//
// Business rules enforced:
// - Author must be a member of the channel
// - Message content must be valid
// - Mentions are resolved from text if not pre-resolved
// - DM channels auto-mention agents when no explicit mentions
//
// Side effects:
// - If message contains @mentions, triggers agent execution (async)
func (s *Service) PostMessage(ctx context.Context, req PostMessageRequest) (*Message, error) {
	// Validate author is member of channel (skip for system:internal or trusted internal callers)
	if req.AuthorID != "system:internal" && !req.SkipMembershipCheck {
		isMember, err := s.membershipRepo.IsMember(ctx, channel.ChannelID(req.ChannelID), req.AuthorID)
		if err != nil {
			return nil, fmt.Errorf("failed to check membership: %w", err)
		}
		if !isMember {
			return nil, fmt.Errorf("author %s is not a member of channel %s", req.AuthorID, req.ChannelID)
		}
	}

	// Resolve mentions from text if not pre-resolved by caller
	if len(req.Content.Mentions) == 0 {
		req.Content.Mentions = s.resolveMentions(ctx, req.Content.Text)
	}

	// DM auto-mention: if still no mentions, check if this is a DM channel with agents
	if len(req.Content.Mentions) == 0 {
		req.Content.Mentions = s.addDMAutoMentions(ctx, req.ChannelID, req.AuthorID)
	}

	// Validate mentioned agents are members of the channel
	// Filter out unauthorized mentions before processing
	authorizedMentions, unauthorizedMentions, err := s.validateMentions(ctx, req.ChannelID, req.Content.Mentions)
	if err != nil {
		return nil, fmt.Errorf("failed to validate mentions: %w", err)
	}

	// Log warning if unauthorized mentions detected
	if len(unauthorizedMentions) > 0 {
		unauthorizedNames := make([]string, 0, len(unauthorizedMentions))
		for _, mention := range unauthorizedMentions {
			unauthorizedNames = append(unauthorizedNames, mention.Name)
		}
		logger.Warn("Filtered unauthorized mentions",
			slog.String("channel_id", req.ChannelID),
			slog.String("author_id", req.AuthorID.String()),
			slog.Any("unauthorized_mentions", unauthorizedNames))
	}

	// Use only authorized mentions
	req.Content.Mentions = authorizedMentions

	// Unfurl deep links in message text (before creating message)
	unfurledContent, err := s.unfurlDeepLinks(ctx, req.Content)
	if err != nil {
		// Log error but don't fail the message - unfurling is non-critical
		logger.Warn("Failed to unfurl deep links", slog.String("error", err.Error()))
		unfurledContent = req.Content // Use original content if unfurling fails
	}

	// Enrich link preview names BEFORE saving to database (so they persist)
	// Pattern: Dependency Inversion - domain calls back to gateway for presentation concerns
	if s.linkPreviewEnricher != nil && len(unfurledContent.LinkPreviews) > 0 {
		unfurledContent.LinkPreviews = s.linkPreviewEnricher(ctx, unfurledContent.LinkPreviews)
		logger.Debug("Enriched link preview names before save",
			slog.Int("preview_count", len(unfurledContent.LinkPreviews)))
	}

	// Create message aggregate (with unfurled links)
	msg, err := NewMessage(req.ChannelID, req.AuthorID, unfurledContent)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// Save to database
	msgID, err := s.messageRepo.Save(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	// Update message with assigned ID
	msg.ID = msgID

	// Index message for search (async, non-blocking)
	// Pattern: Published Interface - notify search infrastructure of new message
	if s.messageIndexer != nil {
		go s.messageIndexer(msg)
	}

	// Trigger agent mentions asynchronously (don't block response)
	// IMPORTANT: Create detached context with ExecutionContext from HTTP request
	// We can't use the original ctx because it gets canceled when HTTP response is sent
	// An unaddressed human message also goes through HandleAgentMentions when a
	// message gate is attached: the gate, not a mention, may hand it to an agent.
	if msg.HasMentions() || s.gateCandidate(msg) {
		// Extract ExecutionContext and attach to background context (detached from HTTP lifecycle)
		asyncCtx := context.Background()

		// Try to get ExecutionContext first (new pattern)
		if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
			asyncCtx = shared.WithExecutionContext(asyncCtx, execCtx)
		} else {
			// Fallback: reconstruct from legacy individual context values
			execCtx := shared.FromContextValues(ctx)
			if execCtx != nil {
				asyncCtx = shared.WithExecutionContext(asyncCtx, execCtx)
			}
		}

		// Carry the turn's interaction mode (plan/acceptEdits/default) to the agent
		// run so executeTool can gate mutating tools in plan mode (Claude-style).
		if req.PermissionMode != "" {
			asyncCtx = context.WithValue(asyncCtx, sharedctx.PermissionModeKey, req.PermissionMode)
		}
		// The client's launch directory (Claude-CLI semantics): the coder works on
		// THAT project. Validated absolute+existing here; empty falls back later.
		if req.Workdir != "" && filepath.IsAbs(req.Workdir) {
			if fi, err := os.Stat(req.Workdir); err == nil && fi.IsDir() {
				asyncCtx = context.WithValue(asyncCtx, sharedctx.WorkdirKey, req.Workdir)
			}
		}

		go s.HandleAgentMentions(asyncCtx, msg)
	}

	return msg, nil
}

// PostSystemAnnouncement posts a system-generated announcement as a threaded reply
// This is used for subagent completions and other automated notifications
// Unlike PostMessage, this bypasses membership checks since system announcements
// are not user-generated and should always be allowed
// Pattern: Duck-typed interface - authorID is interface{} for compatibility with gateway.SystemAnnouncementPoster
func (s *Service) PostSystemAnnouncement(ctx context.Context, channelID string, authorID interface{}, text string, parentMessageID int64) error {
	log := logger

	// Type assert authorID to shared.ActorID
	actorID, ok := authorID.(shared.ActorID)
	if !ok {
		log.Error("Invalid authorID type for system announcement",
			slog.String("expected", "shared.ActorID"),
			slog.String("got", fmt.Sprintf("%T", authorID)))
		return fmt.Errorf("authorID must be of type shared.ActorID, got %T", authorID)
	}

	log.Debug("Creating system announcement",
		slog.String("channel_id", channelID),
		slog.String("actor_id", string(actorID)),
		slog.Int64("parent_message_id", parentMessageID),
		slog.Int("text_length", len(text)))

	// Create message content
	content := MessageContent{
		Text:     text,
		Mentions: []Mention{}, // System announcements don't have mentions
	}

	var msg *Message
	var err error

	// Handle parentMessageID=0 as non-threaded message (e.g., heartbeat announcements)
	// This allows system announcements to post regular messages, not just threaded replies
	if parentMessageID == 0 {
		log.Debug("Creating non-threaded message (parentMessageID=0)")
		msg, err = NewMessage(channelID, actorID, content)
		if err != nil {
			log.Error("Failed to create message",
				slog.Any("error", err),
				slog.String("channel_id", channelID),
				slog.String("actor_id", string(actorID)))
			return fmt.Errorf("failed to create message: %w", err)
		}
	} else {
		log.Debug("Creating threaded reply message",
			slog.Int64("parent_message_id", parentMessageID))
		msg, err = NewReply(channelID, actorID, content, MessageID(parentMessageID))
		if err != nil {
			log.Error("Failed to create reply message",
				slog.Any("error", err),
				slog.String("channel_id", channelID),
				slog.String("actor_id", string(actorID)),
				slog.Int64("parent_message_id", parentMessageID))
			return fmt.Errorf("failed to create reply message: %w", err)
		}
	}

	// Save to database
	msgID, err := s.messageRepo.Save(ctx, msg)
	if err != nil {
		log.Error("Failed to save system announcement to database",
			slog.Any("error", err),
			slog.String("channel_id", channelID),
			slog.String("actor_id", string(actorID)))
		return fmt.Errorf("failed to save announcement: %w", err)
	}

	// Update message with assigned ID
	msg.ID = msgID

	// Index message for search (async, non-blocking)
	if s.messageIndexer != nil {
		go s.messageIndexer(msg)
	}

	// Broadcast announcement to WebSocket clients
	if s.broadcaster != nil {
		s.broadcaster(msg)
	}

	logger.Info("Posted system announcement",
		slog.Any("message_id", msgID),
		slog.Int64("parent_message_id", parentMessageID),
		slog.String("channel_id", channelID))

	return nil
}

// EditMessageRequest encapsulates parameters for editing a message
type EditMessageRequest struct {
	MessageID MessageID
	AuthorID  shared.ActorID // Must match the original message author
	NewText   string
}

// EditMessage updates a message's content (only the author can edit)
func (s *Service) EditMessage(ctx context.Context, req EditMessageRequest) (*Message, error) {
	// Fetch existing message
	msg, err := s.messageRepo.FindByID(ctx, req.MessageID)
	if err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}

	// Verify author matches
	if msg.AuthorID != req.AuthorID {
		return nil, fmt.Errorf("only the message author can edit")
	}

	// Re-resolve mentions from new text
	mentions := s.resolveMentions(ctx, req.NewText)

	// Build new content
	newContent := MessageContent{
		Text:         req.NewText,
		Mentions:     mentions,
		LinkPreviews: msg.Content.LinkPreviews, // Preserve existing link previews
	}

	// Apply edit (sets UpdatedAt)
	if err := msg.EditContent(newContent); err != nil {
		return nil, fmt.Errorf("failed to edit content: %w", err)
	}

	// Persist
	if err := s.messageRepo.UpdateContent(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	// Note: Broadcasting is handled by the gateway with the correct event type (message.updated)

	return msg, nil
}

// ListMessages retrieves messages for a channel
func (s *Service) ListMessages(ctx context.Context, channelID string, page shared.CursorPage) (*PaginatedMessages, error) {
	result, err := s.messageRepo.ListByChannel(ctx, channelID, page)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	return result, nil
}

// HandleAgentMentions processes @mentions and triggers agent executions
// This runs asynchronously to avoid blocking the user's message post
//
// Thread Auto-Participation Pattern (Agent Teams + OpenClaw autoThread):
// - If message is in a thread (ParentID != nil), auto-deliver to the thread lead
// - Thread lead = first agent that responded in the thread (found by checking thread replies)
// - Lead receives ALL thread messages (no @mention needed) until it naturally stops responding
// - Explicitly @mentioned agents also execute (for bringing in teammates)
//
// Agent-to-Agent Communication Safeguards:
// - Enforces mention depth limit to prevent infinite loops
// - Logs all A2A interactions for debugging
//
// PUBLIC API: Can be called directly from gateway for thread replies
func (s *Service) HandleAgentMentions(ctx context.Context, userMsg *Message) {
	// Check mention depth limit for agent-to-agent communication
	// IMPORTANT: Humans ALWAYS bypass depth limits - they can interrupt at any time!
	execCtx := shared.GetExecutionContext(ctx)
	if execCtx != nil && userMsg.AuthorID.IsAgent() && !execCtx.CanMentionAgents() {
		logger.Warn("Mention depth limit reached, skipping agent mentions",
			slog.Int64("message_id", int64(userMsg.ID)),
			slog.Int("mention_depth", execCtx.MentionDepth),
			slog.String("author_id", userMsg.AuthorID.String()))
		return // Stop propagation to prevent infinite agent-to-agent loops
	}

	mentionedAgents := userMsg.GetMentionedAgents()
	agentsToExecute := make(map[string]shared.ActorID) // Use map to deduplicate

	// Add explicitly @mentioned agents
	for _, agentID := range mentionedAgents {
		agentsToExecute[agentID.String()] = agentID
	}

	// Log agent-to-agent mentions for debugging
	if execCtx != nil && userMsg.AuthorID.IsAgent() && len(mentionedAgents) > 0 {
		logger.Info("Agent-to-Agent mention detected",
			slog.String("from_agent", userMsg.AuthorID.String()),
			slog.Int("to_agents_count", len(mentionedAgents)),
			slog.Int("mention_depth", execCtx.MentionDepth))
	}

	// Thread Auto-Participation: If this is a thread reply, auto-deliver to thread lead
	if userMsg.ParentID != nil {
		threadLead := s.getThreadLead(ctx, *userMsg.ParentID)
		if threadLead != nil {
			agentsToExecute[threadLead.String()] = *threadLead
			logger.Info("Auto-delivering thread message to lead agent",
				slog.Int64("parent_id", int64(*userMsg.ParentID)),
				slog.String("lead_agent", threadLead.String()))
		}
	}

	// Gated turns: nobody was addressed and this is not a thread, so ask the
	// message gate (when one is configured) which agent members, if any,
	// should still answer. See decision_gate.go.
	if len(agentsToExecute) == 0 && userMsg.ParentID == nil {
		for _, agentID := range s.gatedAgents(ctx, userMsg) {
			agentsToExecute[agentID.String()] = agentID
		}
	}

	logger.Info("Processing agent mentions",
		slog.Int64("message_id", int64(userMsg.ID)),
		slog.Int("total_agents", len(agentsToExecute)),
		slog.Bool("is_thread_reply", userMsg.ParentID != nil))

	for _, agentID := range agentsToExecute {
		// Skip self-mentions: an agent should never trigger itself
		if agentID == userMsg.AuthorID {
			logger.Info("Skipping self-mention to prevent loop",
				slog.String("agent_id", agentID.String()))
			continue
		}

		// Increment mention depth for agent-to-agent communication
		executionCtx := ctx
		if execCtx != nil && userMsg.AuthorID.IsAgent() {
			// Agent mentioning another agent - increment depth
			incrementedCtx := execCtx.WithIncrementedDepth()
			executionCtx = shared.WithExecutionContext(ctx, incrementedCtx)
			logger.Info("Incremented mention depth for A2A communication",
				slog.String("from_agent", userMsg.AuthorID.String()),
				slog.String("to_agent", agentID.String()),
				slog.Int("new_depth", incrementedCtx.MentionDepth))
		}

		// Execute agent and save response
		if err := s.executeAgentAndSaveResponse(executionCtx, userMsg, agentID); err != nil {
			logger.Error(fmt.Sprintf("Failed to execute agent %s: %v", agentID.String(), err),
				slog.String("agent_id", agentID.String()),
				slog.String("error", err.Error()))
			// Continue with other agents even if one fails
		}
	}
}

// getThreadLead finds the lead agent for a thread by looking at thread replies
// Returns the first agent that responded in the thread (nil if no agent has responded yet)
func (s *Service) getThreadLead(ctx context.Context, parentID MessageID) *shared.ActorID {
	// Get all replies in the thread
	replies, err := s.messageRepo.ListReplies(ctx, parentID)
	if err != nil {
		logger.Error("Failed to get thread replies for lead detection",
			slog.Int64("parent_id", int64(parentID)),
			slog.String("error", err.Error()))
		return nil
	}

	// Find first agent response (thread lead)
	for _, reply := range replies {
		if reply.AuthorID.IsAgent() {
			return &reply.AuthorID
		}
	}

	return nil // No agent has responded yet in this thread
}

// executeAgentAndSaveResponse executes an agent via Published Interface and saves the response
// TurnLedger is the duck-typed port to the agent-turn ledger: dispatches
// are recorded BEFORE execution (idempotently), and every turn is driven
// to a terminal state. Wired by the gateway; nil = feature off.
type TurnLedger interface {
	// BeginTurn returns (turnID, fresh). fresh=false means this exact
	// trigger already has a turn — the dispatch is a duplicate and must be
	// SKIPPED (idempotency).
	BeginTurn(ctx context.Context, channelID, agentID string, triggerMessageID int64) (string, bool, error)
	TurnRunning(ctx context.Context, turnID string)
	TurnResponded(ctx context.Context, turnID string, responseMessageID int64)
	TurnFailed(ctx context.Context, turnID string, errMsg string)
}

// SetTurnLedger wires the reliability ledger (gateway startup).
func (s *Service) SetTurnLedger(tl TurnLedger) { s.turnLedger = tl }

// TurnActive reports whether a turn for (channel, agent) is executing right
// now. The gateway's turn janitor consults it before declaring a turn lost:
// a dispatch-time deadline alone condemned a legitimate 22-minute turn whose
// tool calls were still landing (live 2026-08-31).
func (s *Service) TurnActive(channelID, agentID string) bool {
	return s.turnGate != nil && s.turnGate.held(channelID, agentID)
}

func (s *Service) executeAgentAndSaveResponse(ctx context.Context, userMsg *Message, agentID shared.ActorID) error {
	// ONE TURN AT A TIME per (channel, agent). Every dispatch arrives on its
	// own goroutine, and without this a "continue" typed mid-turn ran
	// concurrently with the turn it meant to follow — two streams interleaved
	// into one transcript (see turn_gate.go). Waiting here preserves arrival
	// order and the Claude-CLI semantics the TUI promises: typing mid-run
	// queues.
	if s.turnGate != nil {
		release := s.turnGate.acquire(userMsg.ChannelID, agentID.String())
		defer release()
	}

	// TURN LEDGER: record the dispatch before ANYTHING can fail silently.
	// A duplicate trigger (same channel+agent+message) is skipped — turns
	// are idempotent.
	turnID := ""
	if s.turnLedger != nil {
		id, fresh, lerr := s.turnLedger.BeginTurn(ctx, userMsg.ChannelID, agentID.String(), int64(userMsg.ID))
		if lerr != nil {
			logger.Warn("turn ledger begin failed — continuing unledgered", slog.String("error", lerr.Error()))
		} else if !fresh {
			logger.Info("duplicate dispatch skipped (turn ledger)",
				slog.String("agent", agentID.String()), slog.Int64("trigger", int64(userMsg.ID)))
			return nil
		} else {
			turnID = id
		}
	}
	err0 := s.executeAgentTurn(ctx, userMsg, agentID, turnID)
	if s.turnLedger != nil && turnID != "" && err0 != nil {
		s.turnLedger.TurnFailed(ctx, turnID, err0.Error())
	}
	return err0
}

func (s *Service) executeAgentTurn(ctx context.Context, userMsg *Message, agentID shared.ActorID, turnID string) error {
	// Parse channel ID to UUID
	channelUUID, err := uuid.Parse(userMsg.ChannelID)
	if err != nil {
		return fmt.Errorf("invalid channel ID '%s': %w", userMsg.ChannelID, err)
	}

	// Get the channel first - we need it for both ExecutionContext fallback and sandbox context
	ch, err := s.channelRepo.GetByID(ctx, channelUUID)
	if err != nil {
		return fmt.Errorf("failed to get channel '%s': %w", userMsg.ChannelID, err)
	}

	// Get ExecutionContext from context (type-safe struct extraction)
	// Pattern: Auth middleware → ExecutionContext → async goroutine → agent execution
	// No ctx.Value() calls - compile-time type safety!
	execCtx := shared.GetExecutionContext(ctx)
	if execCtx == nil {
		// No ExecutionContext found - create one from channel (fallback for unauthenticated requests)
		logger.Warn("ExecutionContext not found in context, using channel fallback")
		execCtx = shared.MustNewExecutionContext(
			shared.NewHumanActorID("00000000-0000-0000-0000-000000000010"), // Default CLI user
			ch.WorkspaceID.String(),
		)
		logger.Info("Created default ExecutionContext from channel",
			slog.String("workspace_id", execCtx.WorkspaceID))
	} else {
		logger.Info("Got ExecutionContext",
			slog.String("workspace_id", execCtx.WorkspaceID),
			slog.String("workspace_slug", execCtx.WorkspaceSlug))
	}

	// Backfill WorkspaceSlug if upstream didn't set it. The A2A path
	// (an agent @-mentioning another) constructs ExecutionContext via the
	// fallback above (or via callers that don't carry slug), leaving
	// WorkspaceSlug empty — and the session key, the sandbox root and the
	// workspace-scoped tools all read it. Bug parked in SHOULD.md from
	// 2026-05-01, fixed 2026-05-02.
	if execCtx.WorkspaceSlug == "" {
		slug, slugErr := s.channelRepo.GetWorkspaceSlug(ctx, ch.WorkspaceID)
		if slugErr != nil {
			logger.Warn("Failed to resolve workspace slug for A2A backfill",
				slog.String("workspace_id", ch.WorkspaceID.String()),
				slog.String("error", slugErr.Error()))
		} else {
			execCtx.WorkspaceSlug = strings.TrimSpace(slug)
			logger.Info("Backfilled empty WorkspaceSlug from channel's workspace",
				slog.String("workspace_id", ch.WorkspaceID.String()),
				slog.String("workspace_slug", execCtx.WorkspaceSlug))
		}
	}

	// Now we have a guaranteed ExecutionContext - no nil checks needed below!
	_ = execCtx.ActorID // Available for future use (e.g., audit logging)

	// THE SESSION IS KEYED BY WORKSPACE. The workspace is the scope, and both
	// sides build the key through pkg/shared so the key a run emits on and the
	// key a /ws client subscribes to cannot drift. Falls back to the workspace
	// id for rows with no slug.
	sessionScope := execCtx.WorkspaceSlug
	if sessionScope == "" {
		sessionScope = execCtx.WorkspaceID
	}
	sessionKey := shared.NewChannelSessionID(sessionScope, userMsg.ChannelID)

	// Get thread root ID for flat threading (Slack-style)
	// Pattern: All replies point to the same thread root, not nested
	threadRootID := userMsg.GetThreadRootID()

	// Load recent channel history so the agent has conversation context.
	// Fetch messages before the current one and convert them to SessionMessages.
	historyMessages := loadChannelHistory(ctx, s.messageRepo, userMsg.ChannelID, userMsg.ID, agentID, 20)

	session := &simpleSessionContext{
		id:   sessionKey,
		typ:  "channel",
		msgs: historyMessages,
		meta: map[string]interface{}{
			"channel_id":        userMsg.ChannelID,
			"agent_id":          agentID.String(),
			"parent_message_id": int64(threadRootID), // Thread root for flat threading (Slack-style)
		},
	}
	logger.Debug("Loaded conversation history for agent",
		slog.String("agent", agentID.String()),
		slog.String("channel", userMsg.ChannelID),
		slog.Int("messages", len(historyMessages)))

	runID := fmt.Sprintf("run-%d-%s", userMsg.ID, agentID.ID())

	// Broadcast execution started event
	s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "started", nil)

	// Look up buddy to get sandbox scope for security enforcement
	agentName := agentID.ID() // Extract agent name from ActorID (e.g., "agent:coder" -> "coder")
	logger.Debug("Looking up agent buddy",
		slog.String("agent_id", agentID.String()),
		slog.String("agent_name", agentName),
		slog.String("channel_id", userMsg.ChannelID))

	buddy, err := s.buddyRepo.GetByName(ctx, agentName)
	if err != nil {
		logger.Error("Failed to look up buddy for agent",
			slog.String("agent_id", agentID.String()),
			slog.String("agent_name", agentName),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to look up agent '%s': %w", agentName, err)
	}

	logger.Debug("Found agent buddy",
		slog.String("agent_id", agentID.String()),
		slog.String("sandbox_scope", string(buddy.SandboxScope)),
		slog.String("execution_type", buddy.ExecutionType),
		slog.Bool("has_remote_config", buddy.RemoteConfig != nil))

	// Create sandbox context for security enforcement
	// Pattern: Same as gateway/chat_server.go:executeAgentMention
	// Note: Use channel's WorkspaceID (UUID) for sandbox isolation, not session's workspace_id (string)
	sandboxCtx := sandbox.SandboxContext{
		WorkspaceID:      ch.WorkspaceID,        // Use channel's UUID-based workspace_id
		WorkspaceSlug:    execCtx.WorkspaceSlug, // resolved per-request
		ChannelID:        channelUUID,
		InitiatingUserID: parseUserIDOrDefault(execCtx.ActorID.ID()),
		CurrentAgentID:   agentID.String(),
		AgentScope:       buddy.SandboxScope,
	}

	// Add sandbox context to Go context for thread-safe propagation
	ctx = context.WithValue(ctx, sharedctx.SandboxContextKey, sandboxCtx)

	// Auto-inject get_secret tool if agent has secrets configured
	agentTools := make([]string, len(buddy.Tools))
	copy(agentTools, buddy.Tools)
	if s.agentSecretCounter != nil {
		if count, err := s.agentSecretCounter.Count(ctx, agentID.String()); err == nil && count > 0 {
			hasGetSecret := false
			for _, t := range agentTools {
				if t == "get_secret" {
					hasGetSecret = true
					break
				}
			}
			if !hasGetSecret {
				agentTools = append(agentTools, "get_secret")
				logger.Info("Auto-injected get_secret tool",
					slog.String("agent_name", agentName),
					slog.Int("secret_count", count))
			}
		}
	}

	// Add buddy.Tools to context for runtime tool filtering
	// This allows per-agent tool configuration from database to override config files
	if len(agentTools) > 0 {
		ctx = context.WithValue(ctx, sharedctx.BuddyToolsKey, agentTools)
		logger.Info("Added agent tools to context",
			slog.String("agent_name", agentName),
			slog.Int("tool_count", len(agentTools)),
			slog.Any("tools", agentTools))
	} else {
		logger.Info("No buddy.Tools to add to context - will use default profile",
			slog.String("agent_name", agentName))
	}

	// Add buddy name to context for per-agent memory databases
	ctx = context.WithValue(ctx, "buddy_agent_name", agentName)

	// Mark as buddy-driven chat (drives lean prompt mode); LLM
	// provider/model are workspace-wide via byok, not per-buddy.
	ctx = context.WithValue(ctx, sharedctx.IsBuddyChatKey, true)

	// Add learning_enabled to context for memory retrieval and reflection
	if buddy.LearningEnabled {
		ctx = context.WithValue(ctx, sharedctx.BuddyLearningKey, true)
		logger.Info("Learning enabled for agent",
			slog.String("agent_name", agentName))
	}

	// Execute agent via Published Interface with a hang-backstop timeout.
	// History: 180s killed multi-round verifier flows; 300s killed long coder
	// turns before they answered. The timeout exists to reap hung turns, not
	// to pace slow models: 30 minutes.
	//
	// This is the OUTER net. The runtime sets the real per-turn backstop
	// from the same setting (gateway/agent_runtime_process.go) and gives an
	// editor twice that, because a 20-minute recording re-edited on the
	// leased brain legitimately runs past thirty minutes. This deadline was
	// the same thirty and killed that turn at 12:03:14 (2026-09-13) with the
	// plan half made — the inner doubling never got to apply. So this one is
	// twice the setting plus a minute: it catches a hung turn the runtime's
	// own deadline somehow did not, and is never the binding one.
	timeout := 2*1800*time.Second + time.Minute
	// Without progress, not in total (sharedctx.WithIdleTimeout): the
	// runtime reports every model answer and tool result, so a working
	// turn is never reaped, however long it runs.
	ctx, cancel := sharedctx.WithIdleTimeout(ctx, timeout)
	defer cancel()

	logger.Info("Executing agent via ProcessMessage",
		slog.String("agent_id", agentID.String()),
		slog.String("channel_id", userMsg.ChannelID),
		slog.String("session_key", sessionKey),
		slog.String("run_id", runID),
		slog.String("execution_type", buddy.ExecutionType),
		slog.Int64("message_id", int64(userMsg.ID)))

	// Route to appropriate executor based on agent's execution type
	var result *platform.ExecutionResult
	if buddy.ExecutionType == "remote" && s.remoteExecutor != nil {
		// Remote agent execution (OpenAI-compatible endpoint)
		logger.Info("🔴 ROUTING TO REMOTE EXECUTOR",
			slog.String("agent_id", agentID.String()),
			slog.String("execution_type", buddy.ExecutionType),
			slog.String("endpoint", buddy.RemoteConfig.Endpoint),
			slog.String("model", buddy.RemoteConfig.Model))

		result, err = s.executeRemoteAgent(ctx, buddy, userMsg.Content.Text, session, ch)
	} else {
		// Local agent execution (default LLM API)
		logger.Info("🔵 ROUTING TO LOCAL EXECUTOR",
			slog.String("agent_id", agentID.String()),
			slog.String("execution_type", buddy.ExecutionType),
			slog.Bool("has_remote_executor", s.remoteExecutor != nil))

		// Build extra system prompt from buddy's personality and system prompt.
		// A per-workspace <ws>/agents/<name>.prompt.md REPLACES buddy.SystemPrompt;
		// a <ws>/agents/<name>.examples.md is APPENDED as supplemental calibration.
		// The assembly lives in prompts.BuildAgentExtraPrompt, shared with the
		// gateway's spawned agents, so the same agent sends the same bytes.
		personality := ""
		if buddy.Personality != nil {
			personality = *buddy.Personality
		}
		agentSystemPrompt := ""
		if buddy.SystemPrompt != nil {
			agentSystemPrompt = *buddy.SystemPrompt
		}
		language := ""
		if s.languageResolver != nil {
			language = s.languageResolver.GetLanguage(ctx)
		}
		extraPrompt := prompts.BuildAgentExtraPrompt(personality, agentSystemPrompt, execCtx.WorkspaceSlug, agentName, language)

		if s.turnLedger != nil && turnID != "" {
			s.turnLedger.TurnRunning(ctx, turnID)
		}
		result, err = s.agentExecutor.ProcessMessage(ctx, userMsg.Content.Text, session, runID, extraPrompt)
	}

	if err != nil {
		logger.Error("Agent execution failed in ProcessMessage",
			slog.String("agent_id", agentID.String()),
			slog.String("error", err.Error()))
	} else {
		logger.Info("Agent execution succeeded",
			slog.String("agent_id", agentID.String()),
			slog.String("model", result.Model),
			slog.Int("response_length", len(result.Text)))
	}
	if err != nil {
		// Broadcast execution failed event
		s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "failed", map[string]interface{}{
			"error": err.Error(),
		})

		// Send user-friendly error message so the user knows what happened
		// Error messages are also threaded replies (like Slack)
		errorMessage := s.createUserFriendlyErrorMessage(err, agentID)
		errorResponseContent := MessageContent{
			Text:     errorMessage,
			Mentions: []Mention{},
		}

		errorResponseMsg, msgErr := NewReply(userMsg.ChannelID, agentID, errorResponseContent, userMsg.ID)
		if msgErr == nil {
			// Use a fresh context for saving error message (original ctx may be cancelled due to timeout)
			saveCtx := context.Background()
			if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
				saveCtx = shared.WithExecutionContext(saveCtx, execCtx)
			}

			// Save and broadcast error message to user
			errorMsgID, saveErr := s.messageRepo.Save(saveCtx, errorResponseMsg)
			if saveErr == nil {
				errorResponseMsg.ID = errorMsgID
				if s.broadcaster != nil {
					s.broadcaster(errorResponseMsg)
				}
				logger.Info("Sent error message to user",
					slog.String("agent_id", agentID.String()),
					slog.String("error", err.Error()))
			} else {
				logger.Error("Failed to save error message",
					slog.String("agent_id", agentID.String()),
					slog.String("save_error", saveErr.Error()),
					slog.String("original_error", err.Error()))
			}
		} else {
			logger.Error("Failed to create error message",
				slog.String("agent_id", agentID.String()),
				slog.String("message_error", msgErr.Error()),
				slog.String("original_error", err.Error()))
		}

		return fmt.Errorf("agent execution failed: %w", err)
	}

	logger.Info("Agent responded",
		slog.String("agent_id", agentID.String()),
		slog.String("response_preview", result.Text[:min(100, len(result.Text))]),
		slog.Int("tools_executed", len(result.ToolsExecuted)))

	// Check for REPLY_SKIP token (OpenClaw pattern for A2A collaboration control)
	// Pattern: Agent explicitly opts out of posting message and triggering A2A mentions
	// This prevents echo loops and allows agents to stay silent when appropriate
	if IsReplySkip(result.Text) {
		logger.Info("Agent used REPLY_SKIP token, skipping message post and A2A processing",
			slog.String("agent_id", agentID.String()),
			slog.String("channel_id", userMsg.ChannelID))

		// Broadcast execution completed (agent executed successfully but chose silence)
		if s.broadcaster != nil {
			s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "skipped", map[string]interface{}{
				"reason": "REPLY_SKIP",
			})
		}

		if s.turnLedger != nil && turnID != "" {
			s.turnLedger.TurnResponded(ctx, turnID, 0) // terminal: silent by choice
		}
		return nil // Silent success - agent executed but chose not to post or trigger A2A
	}

	// Parse mentions from agent response text (Agent-to-Agent Communication)
	// Pattern: Agents can @mention other agents for collaboration
	// Safeguards: Max 3 mentions per message, depth limit enforced later
	mentionedNames := ParseMentions(result.Text)
	logger.Info("Parsed raw mentions from agent response text",
		slog.String("agent_id", agentID.String()),
		slog.Int("raw_mentions_count", len(mentionedNames)),
		slog.Any("raw_mentions", mentionedNames),
		slog.String("response_preview", result.Text[:min(200, len(result.Text))]))

	const maxMentionsPerMessage = 3
	if len(mentionedNames) > maxMentionsPerMessage {
		mentionedNames = mentionedNames[:maxMentionsPerMessage]
		logger.Warn("Agent mentioned too many agents, truncating to max",
			slog.String("agent_id", agentID.String()),
			slog.Int("total_mentions", len(ParseMentions(result.Text))),
			slog.Int("allowed", maxMentionsPerMessage))
	}

	// Get channel members to validate mentions (prevents "@mentions" false positives)
	// Pattern: Duck-typed interface - only @mention agents that are in this channel
	channelMembers, err := s.membershipRepo.ListByChannel(ctx, channel.ChannelID(userMsg.ChannelID), shared.AllPages())
	if err != nil {
		logger.Warn("Failed to fetch channel members for mention validation, skipping validation",
			slog.String("channel_id", userMsg.ChannelID),
			slog.String("error", err.Error()))
		channelMembers = nil // Continue without validation
	} else {
		logger.Info("Fetched channel members for mention validation",
			slog.String("channel_id", userMsg.ChannelID),
			slog.Int("total_members", len(channelMembers)))
	}

	// Build map of valid member ActorIDs in this channel
	validMemberIDs := make(map[shared.ActorID]bool)
	if channelMembers != nil {
		for _, member := range channelMembers {
			validMemberIDs[member.ID.ActorID] = true
		}
		logger.Info("Built valid member IDs set from channel members",
			slog.String("channel_id", userMsg.ChannelID),
			slog.Int("valid_members_count", len(validMemberIDs)))
	}

	// Build Mention objects (validate member exists and is in channel)
	mentions := make([]Mention, 0, len(mentionedNames))
	skippedMentions := []string{}
	for _, name := range mentionedNames {
		var actorID shared.ActorID
		var found bool
		var actorType string

		// Detect mention type: if contains '@', it's an email mention; otherwise it's an agent name
		if strings.Contains(name, "@") {
			// Email mention (e.g., admin@localhost) - need to resolve email to ActorID
			if s.authRepo != nil {
				user, _, err := s.authRepo.GetUserByEmail(ctx, name)
				if err != nil {
					logger.Warn("Failed to look up user by email for mention",
						slog.String("email", name),
						slog.String("error", err.Error()))
					skippedMentions = append(skippedMentions, name)
					continue
				}
				if user == nil {
					logger.Warn("User not found for email mention",
						slog.String("email", name))
					skippedMentions = append(skippedMentions, name)
					continue
				}

				// Extract ActorID from user (duck-typed interface)
				humanActorID := shared.ActorID(user.GetID())
				if err := humanActorID.Validate(); err == nil && humanActorID.IsHuman() {
					// Check if this human is in the channel
					if channelMembers == nil || validMemberIDs[humanActorID] {
						actorID = humanActorID
						actorType = "human"
						found = true
						logger.Info("Validated human mention via email lookup",
							slog.String("email", name),
							slog.String("actor_id", humanActorID.String()))
					} else {
						logger.Warn("User found but not in channel",
							slog.String("email", name),
							slog.String("actor_id", humanActorID.String()))
					}
				} else {
					logger.Warn("Invalid ActorID from user lookup",
						slog.String("email", name),
						slog.String("actor_id", user.GetID()))
					skippedMentions = append(skippedMentions, name)
					continue
				}
			} else {
				logger.Warn("Email mention detected but authRepo not configured",
					slog.String("email", name))
				skippedMentions = append(skippedMentions, name)
				continue
			}
		} else {
			// Try agent name first (e.g., @writer)
			agentActorID := shared.NewAgentActorID(name)
			if err := agentActorID.Validate(); err == nil {
				if channelMembers == nil || validMemberIDs[agentActorID] {
					actorID = agentActorID
					actorType = "agent"
					found = true
				}
			}

			// Fallback: try username lookup for human mentions (e.g., @admin)
			if !found && s.authRepo != nil {
				user, err := s.authRepo.GetUserByUsername(ctx, name)
				if err == nil && user != nil {
					humanActorID := shared.ActorID(user.GetID())
					if err := humanActorID.Validate(); err == nil && humanActorID.IsHuman() {
						if channelMembers == nil || validMemberIDs[humanActorID] {
							actorID = humanActorID
							actorType = "human"
							found = true
							logger.Info("Validated human mention via username lookup",
								slog.String("username", name),
								slog.String("actor_id", humanActorID.String()))
						}
					}
				}
			}
		}

		if found {
			mentions = append(mentions, Mention{
				ActorID: actorID,
				Name:    name,
			})
			logger.Info("Validated mention - member is in channel",
				slog.String("mentioned_member", name),
				slog.String("actor_type", actorType),
				slog.String("channel_id", userMsg.ChannelID),
				slog.String("from_agent", agentID.String()))
		} else if !strings.Contains(name, "@") {
			// Only log skip warning for non-email mentions (email mentions logged above)
			skippedMentions = append(skippedMentions, name)
			logger.Warn("Skipping mention of agent not in channel (prevents false positives)",
				slog.String("mentioned_agent", name),
				slog.String("channel_id", userMsg.ChannelID),
				slog.String("from_agent", agentID.String()))
		}
	}

	logger.Info("Final mention validation results",
		slog.String("agent_id", agentID.String()),
		slog.Int("total_parsed", len(mentionedNames)),
		slog.Int("valid_mentions", len(mentions)),
		slog.Int("skipped_mentions", len(skippedMentions)),
		slog.Any("valid_mentions_list", mentions),
		slog.Any("skipped_mentions_list", skippedMentions))

	// Replace self-mentions with @me to prevent agent self-mention loops
	// Pattern: When an agent receives a message from another agent, it sees its own name in the prompt
	// This causes agents to potentially mention themselves, creating unnecessary loops
	// Solution: Replace @agentName with @me when the agent is mentioning itself
	responseText := result.Text
	selfMentionPattern := "@" + agentName
	if strings.Contains(responseText, selfMentionPattern) {
		responseText = strings.ReplaceAll(responseText, selfMentionPattern, "@me")
		logger.Info("Replaced self-mention with @me to prevent agent loop",
			slog.String("agent_id", agentID.String()),
			slog.String("agent_name", agentName),
			slog.String("pattern", selfMentionPattern))
	}

	// Filter out self-mentions from the mentions slice to prevent agent self-mention loops
	// The text was already replaced above, but the parsed mentions still contain the agent's own ActorID
	filteredMentions := make([]Mention, 0, len(mentions))
	for _, m := range mentions {
		if m.ActorID != agentID {
			filteredMentions = append(filteredMentions, m)
		}
	}
	if len(filteredMentions) < len(mentions) {
		logger.Info("Filtered self-mention from mentions list",
			slog.String("agent_id", agentID.String()),
			slog.Int("before", len(mentions)),
			slog.Int("after", len(filteredMentions)))
	}

	// An empty final response is silent SUCCESS, not an error. It happens on a
	// tool-only turn — the model ran a long tool and returned no prose.
	// Building a message with empty text fails validation
	// ("text cannot be empty"), and that error used to return below BEFORE the
	// "completed" broadcast — so the TUI never got a completion event and its
	// spinner hung forever. Treat it like REPLY_SKIP: signal completion, post
	// nothing. (Surfacing a tool-derived summary instead is a future nicety.)
	if strings.TrimSpace(responseText) == "" {
		if s.turnLedger != nil && turnID != "" {
			s.turnLedger.TurnResponded(ctx, turnID, 0) // terminal: empty final text
		}
		logger.Info("Agent produced empty final text — treating as silent completion",
			slog.String("agent_id", agentID.String()),
			slog.Int("tools_executed", len(result.ToolsExecuted)))
		s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "completed", map[string]interface{}{
			"tools_count": len(result.ToolsExecuted),
			"silent":      true,
		})
		return nil
	}

	// Create response message from agent
	responseContent := MessageContent{
		Text:     responseText,
		Mentions: filteredMentions,
	}

	// DM channels: top-level messages (like Slack DMs), not threaded replies
	// Regular channels: threaded replies (Slack-style)
	isDM := strings.HasPrefix(ch.Name, "dm-")
	var responseMsg *Message
	if isDM {
		responseMsg, err = NewMessage(userMsg.ChannelID, agentID, responseContent)
	} else {
		threadRootID = userMsg.GetThreadRootID()
		responseMsg, err = NewReply(userMsg.ChannelID, agentID, responseContent, threadRootID)
	}
	if err != nil {
		// Broadcast a terminal event before returning so the TUI spinner clears —
		// any return between the success branch and the "completed" broadcast
		// below must signal the run is over, or the client waits forever.
		s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to create response message: %w", err)
	}

	// Save agent response to database. Detach from the 300s agent
	// timeout ctx the same way the error-path save and the async
	// mentions handler do — by the time we reach this point the LLM
	// call has already finished (either cleanly or via the OAI
	// streaming-salvage path) and the timeout's job is done.
	// Inheriting ctx here causes "context deadline exceeded" on the
	// DB insert when the LLM stream was salvaged at the very last
	// moment, dropping a valid (if truncated) agent response on the
	// floor. ExecutionContext (workspace, user, etc.) is preserved.
	saveCtx := context.Background()
	if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
		saveCtx = shared.WithExecutionContext(saveCtx, execCtx)
	}
	responseMsgID, err := s.messageRepo.Save(saveCtx, responseMsg)
	if err != nil {
		s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to save agent response: %w", err)
	}

	// Update message with assigned ID
	responseMsg.ID = responseMsgID
	if s.turnLedger != nil && turnID != "" {
		s.turnLedger.TurnResponded(saveCtx, turnID, int64(responseMsgID))
	}

	logger.Info("Saved agent response as message",
		slog.String("agent_id", agentID.String()),
		slog.Int64("message_id", int64(responseMsgID)))

	// Broadcast execution completed event
	s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "completed", map[string]interface{}{
		"message_id":  responseMsgID.String(),
		"tools_count": len(result.ToolsExecuted),
	})

	// Broadcast agent response to connected clients (if broadcaster is set)
	if s.broadcaster != nil {
		s.broadcaster(responseMsg)
	}

	// Trigger agent-to-agent mentions (if any) - Agent response may mention other agents!
	// This enables true multi-agent collaboration where agents can recruit teammates
	if responseMsg.HasMentions() {
		logger.Info("Agent response contains mentions, triggering A2A communication",
			slog.String("agent_id", agentID.String()),
			slog.Int("mentions_count", len(responseMsg.Content.Mentions)))

		// CRITICAL FIX: Create detached context with ExecutionContext (don't use ctx - it may be canceled!)
		// Pattern: Same as PostMessage - detach from HTTP lifecycle
		asyncCtx := context.Background()
		if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
			asyncCtx = shared.WithExecutionContext(asyncCtx, execCtx)
		}

		// Run async to avoid blocking this response
		go s.HandleAgentMentions(asyncCtx, responseMsg)
	}

	return nil
}

// broadcastExecutionEvent sends an execution event to connected clients via the broadcaster
func (s *Service) broadcastExecutionEvent(channelID string, agentID shared.ActorID, runID string, status string, data map[string]interface{}) {
	if s.executionBroadcaster == nil {
		return
	}

	s.executionBroadcaster(channelID, agentID, runID, status, data)
}

// createUserFriendlyErrorMessage converts technical errors into user-friendly messages
func (s *Service) createUserFriendlyErrorMessage(err error, agentID shared.ActorID) string {
	errorStr := err.Error()

	// Check for common error patterns and provide helpful explanations
	switch {
	case strings.Contains(errorStr, "context deadline exceeded"):
		return "⏱️ Sorry, my request timed out. This usually happens when:\n" +
			"- The task is taking longer than 3 minutes\n" +
			"- There are network connectivity issues\n" +
			"- The AI service is experiencing high load\n\n" +
			"Please try:\n" +
			"1. Breaking your request into smaller steps\n" +
			"2. Simplifying the task\n" +
			"3. Trying again in a moment"

	case strings.Contains(errorStr, "no such file or directory"):
		// Extract filename if possible
		return "📁 I couldn't access the file you requested. This might be because:\n" +
			"- The file path doesn't exist in my sandbox\n" +
			"- I don't have permission to access files outside my workspace\n" +
			"- The file path is relative to a different directory\n\n" +
			"My sandbox scope: Check my settings to see what files I can access.\n\n" +
			"You can:\n" +
			"1. Use absolute paths if I have broader permissions\n" +
			"2. Copy/paste the file content directly\n" +
			"3. Ask an admin to adjust my sandbox permissions"

	case strings.Contains(errorStr, "permission denied"):
		return "🔒 I don't have permission to perform that action. " +
			"Please check my sandbox settings or ask an administrator for help."

	case strings.Contains(errorStr, "not found"):
		return "🔍 I couldn't find what you're looking for. Please check:\n" +
			"- The spelling and path are correct\n" +
			"- The resource exists\n" +
			"- I have access to that location"

	default:
		// Generic error message with the actual error
		return fmt.Sprintf("❌ I encountered an error while processing your request:\n\n```\n%s\n```\n\n"+
			"If this keeps happening, please contact an administrator or try rephrasing your request.", errorStr)
	}
}

// simpleSessionContext implements platform.SessionContext for Published Interface
type simpleSessionContext struct {
	id   string
	typ  string
	msgs []platform.SessionMessage
	meta map[string]interface{}
}

func (s *simpleSessionContext) GetID() string {
	return s.id
}

func (s *simpleSessionContext) GetType() string {
	return s.typ
}

func (s *simpleSessionContext) GetMessages() []platform.SessionMessage {
	return s.msgs
}

func (s *simpleSessionContext) GetMetadata() map[string]interface{} {
	return s.meta
}

// validateMentions checks if mentioned agents are members of the channel
// Returns (authorized mentions, unauthorized mentions, error)
// Only agents need authorization - human mentions are always allowed
// Pattern: Specification/Predicate - fetches members once, then filters using predicate
func (s *Service) validateMentions(ctx context.Context, channelID string, mentions []Mention) ([]Mention, []Mention, error) {
	if len(mentions) == 0 {
		return nil, nil, nil
	}

	// Fetch all channel members once (avoid N+1 query problem)
	memberships, err := s.membershipRepo.ListByChannel(ctx, channel.ChannelID(channelID), shared.AllPages())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list channel members: %w", err)
	}

	// Build predicate: Set of member ActorIDs for O(1) lookup
	memberSet := make(map[shared.ActorID]bool)
	for _, membership := range memberships {
		memberSet[membership.ID.ActorID] = true
	}

	// Apply specification pattern: filter mentions using predicate
	// Auto-join agents that are mentioned but not yet members
	authorized := make([]Mention, 0, len(mentions))
	unauthorized := make([]Mention, 0)

	execCtx := shared.GetExecutionContext(ctx)
	isRegularUser := execCtx != nil && execCtx.UserRole == string(domain.UserRoleUser)

	for _, mention := range mentions {
		// RBAC: Regular users can only mention channel members
		if isRegularUser {
			if !memberSet[mention.ActorID] {
				logger.Info("Blocked non-admin user from mentioning non-member",
					slog.String("mentioned", mention.ActorID.String()),
					slog.String("channel", channelID))
				unauthorized = append(unauthorized, mention)
				continue
			}
			authorized = append(authorized, mention)
			continue
		}

		// Admin path: human mentions always allowed
		if !mention.ActorID.IsAgent() {
			authorized = append(authorized, mention)
			continue
		}

		// Admin path: auto-join agent to channel if not a member
		if !memberSet[mention.ActorID] {
			membership, err := channel.NewMembership(channel.ChannelID(channelID), mention.ActorID, channel.RoleMember, nil)
			if err == nil {
				if saveErr := s.membershipRepo.Save(ctx, membership); saveErr == nil {
					memberSet[mention.ActorID] = true
					logger.Info("Auto-joined agent to channel on mention",
						slog.String("agent", mention.ActorID.String()),
						slog.String("channel", channelID))
				}
			}
		}

		if memberSet[mention.ActorID] {
			authorized = append(authorized, mention)
		} else {
			unauthorized = append(unauthorized, mention)
		}
	}

	return authorized, unauthorized, nil
}

// UnfurlDeepLinks is the public API for unfurling deep links in message content
// This is used by both regular messages and thread replies
func (s *Service) UnfurlDeepLinks(ctx context.Context, content MessageContent) (MessageContent, error) {
	return s.unfurlDeepLinks(ctx, content)
}

// unfurlDeepLinks detects message deep links in text and fetches previews
// Deep link format: http://localhost:XXXX?channel=ID&message=ID
func (s *Service) unfurlDeepLinks(ctx context.Context, content MessageContent) (MessageContent, error) {
	logger.Info("unfurlDeepLinks called",
		slog.String("text", content.Text),
		slog.Bool("contains_http", strings.Contains(content.Text, "http")))

	// Quick check: does the text contain http?
	if !strings.Contains(content.Text, "http") {
		logger.Debug("No HTTP URLs in text, skipping unfurling")
		return content, nil // No URLs, skip processing
	}

	// Find all URLs in the text
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	urls := urlRegex.FindAllString(content.Text, -1)

	logger.Info("Found URLs in text",
		slog.Int("url_count", len(urls)),
		slog.Any("urls", urls))

	if len(urls) == 0 {
		return content, nil
	}

	// Filter for message deep links (have both channel and message params)
	var previews []LinkPreview
	for _, urlStr := range urls {
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			logger.Warn("Failed to parse URL", slog.String("url", urlStr), slog.String("error", err.Error()))
			continue // Skip invalid URLs
		}

		channelID := parsedURL.Query().Get("channel")
		messageID := parsedURL.Query().Get("message")

		logger.Info("Parsed URL query params",
			slog.String("url", urlStr),
			slog.String("channel_id", channelID),
			slog.String("message_id", messageID))

		if channelID == "" || messageID == "" {
			continue // Not a message deep link
		}

		// Fetch the referenced message
		msgIDInt, err := strconv.ParseInt(messageID, 10, 64)
		if err != nil {
			logger.Warn("Invalid message ID in deep link", slog.String("messageID", messageID))
			continue
		}

		refMsg, err := s.messageRepo.FindByID(ctx, MessageID(msgIDInt))
		if err != nil {
			logger.Warn("Failed to fetch message for deep link preview",
				slog.String("messageID", messageID),
				slog.String("error", err.Error()))
			continue
		}

		// Get channel name - try to fetch from channel service
		channelName := channelID // Default to ID if name lookup fails
		// Note: We access channel data through the channel package's types
		// The actual repository method name may vary by implementation

		// Get author name (if enricher is available)
		authorName := refMsg.AuthorID.String()
		// Note: We don't have access to nameEnricher here in the domain layer
		// The gateway will enrich names when converting to DTO

		// Create preview
		preview := LinkPreview{
			URL:  urlStr,
			Type: "message",
			Data: &LinkPreviewMessage{
				MessageID:   messageID,
				ChannelID:   channelID,
				ChannelName: channelName,
				AuthorID:    refMsg.AuthorID,
				AuthorName:  authorName,
				Text:        refMsg.Content.Text,
				CreatedAt:   refMsg.CreatedAt.Format(time.RFC3339),
			},
		}

		previews = append(previews, preview)
		logger.Info("Created link preview",
			slog.String("url", urlStr),
			slog.String("message_id", messageID),
			slog.String("channel_id", channelID))
	}

	// Add previews to content
	content.LinkPreviews = previews
	logger.Info("unfurlDeepLinks completed",
		slog.Int("preview_count", len(previews)))
	return content, nil
}

// executeRemoteAgent executes a remote agent via OpenAI-compatible endpoint
// with full tool support, system prompt injection, and channel context
func (s *Service) executeRemoteAgent(
	ctx context.Context,
	buddy *domain.Buddy,
	userMessage string,
	session platform.SessionContext,
	ch *domain.Channel,
) (*platform.ExecutionResult, error) {
	// Validate remote config exists
	if buddy.RemoteConfig == nil {
		return nil, fmt.Errorf("remote agent '%s' missing remote configuration", buddy.Name)
	}

	// Convert domain.RemoteAgentConfig to platform.RemoteAgentConfig
	platformConfig := &platform.RemoteAgentConfig{
		Endpoint:         buddy.RemoteConfig.Endpoint,
		APIKey:           buddy.RemoteConfig.APIKey,
		Model:            buddy.RemoteConfig.Model,
		Timeout:          buddy.RemoteConfig.Timeout,
		MaxRetries:       buddy.RemoteConfig.MaxRetries,
		Temperature:      buddy.RemoteConfig.Temperature,
		TopP:             buddy.RemoteConfig.TopP,
		FrequencyPenalty: buddy.RemoteConfig.FrequencyPenalty,
		ToolChoice:       buddy.RemoteConfig.ToolChoice,
	}
	// Fall back to buddy-level temperature if remote config doesn't set one
	if platformConfig.Temperature == 0 {
		platformConfig.Temperature = buddy.Temperature
	}

	// Build system prompt from buddy config + channel context
	systemPrompt := s.buildRemoteSystemPrompt(ctx, buddy, ch)

	// Build message history with system prompt prepended
	messages := make([]platform.SessionMessage, 0, len(session.GetMessages())+3)
	if systemPrompt != "" {
		messages = append(messages, platform.SessionMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, session.GetMessages()...)

	messages = append(messages, platform.SessionMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now().UnixMilli(),
	})

	// Get tool definitions for this agent
	var toolDefs []platform.ToolDefinition
	if s.remoteToolExecutor != nil {
		toolDefs = s.remoteToolExecutor.GetToolDefinitions(ctx, buddy.Tools)
		logger.Info("Remote agent tools loaded",
			slog.String("agent", buddy.Name),
			slog.Int("tool_count", len(toolDefs)))
	}

	logger.Info("Calling remote agent",
		slog.String("agent", buddy.Name),
		slog.String("endpoint", platformConfig.Endpoint),
		slog.String("model", platformConfig.Model),
		slog.Int("tool_count", len(toolDefs)),
		slog.Bool("has_system_prompt", systemPrompt != ""))

	// Tool call loop: execute until no more tool_calls
	const maxToolRounds = 10
	var allToolsExecuted []platform.ToolExecution

	for round := 0; round <= maxToolRounds; round++ {
		// After first round, relax tool_choice so the model can return a final answer
		// (otherwise "required" forces infinite tool calls)
		roundConfig := *platformConfig
		if round > 0 && roundConfig.ToolChoice == "required" {
			roundConfig.ToolChoice = "auto"
		}
		remoteResult, err := s.remoteExecutor.Execute(ctx, &roundConfig, messages, toolDefs)
		if err != nil {
			return nil, fmt.Errorf("remote agent execution failed (round %d): %w", round, err)
		}

		// No tool calls — final response
		if len(remoteResult.ToolCalls) == 0 {
			result := &platform.ExecutionResult{
				Text: remoteResult.Text,
				Metadata: map[string]interface{}{
					"remote":      true,
					"endpoint":    platformConfig.Endpoint,
					"usage":       remoteResult.Usage,
					"tool_rounds": round,
				},
				ToolsExecuted: allToolsExecuted,
			}
			for k, v := range remoteResult.Metadata {
				result.Metadata[k] = v
			}

			logger.Info("Remote agent execution succeeded",
				slog.String("agent", buddy.Name),
				slog.Int("response_length", len(result.Text)),
				slog.Int("tool_rounds", round),
				slog.Int("tools_executed", len(allToolsExecuted)))

			return result, nil
		}

		// Tool calls present — execute locally and send results back
		if s.remoteToolExecutor == nil {
			logger.Warn("Remote agent requested tool calls but no tool executor configured",
				slog.String("agent", buddy.Name),
				slog.Int("tool_calls", len(remoteResult.ToolCalls)))
			// Return whatever text we have
			return &platform.ExecutionResult{
				Text: remoteResult.Text,
				Metadata: map[string]interface{}{
					"remote":               true,
					"endpoint":             platformConfig.Endpoint,
					"tool_calls_unhandled": len(remoteResult.ToolCalls),
				},
			}, nil
		}

		logger.Info("Remote agent requested tool calls",
			slog.String("agent", buddy.Name),
			slog.Int("round", round),
			slog.Int("tool_calls", len(remoteResult.ToolCalls)))

		// Add assistant message with tool calls to conversation
		// (The remote executor already parsed these from the response)
		messages = append(messages, platform.SessionMessage{
			Role:      "assistant",
			Content:   remoteResult.Text,
			ToolCalls: remoteResult.ToolCalls,
		})

		// Execute each tool call locally
		for _, tc := range remoteResult.ToolCalls {
			toolResult, toolErr := s.remoteToolExecutor.ExecuteToolCall(ctx, tc.Name, tc.Arguments)
			if toolErr != nil {
				toolResult = fmt.Sprintf("Error executing tool '%s': %s", tc.Name, toolErr.Error())
				logger.Error("Remote tool execution failed",
					slog.String("tool", tc.Name),
					slog.Any("error", toolErr))
			} else {
				logger.Info("Remote tool executed",
					slog.String("tool", tc.Name),
					slog.Int("result_length", len(toolResult)))
			}

			// Add tool result to conversation
			messages = append(messages, platform.SessionMessage{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: tc.ID,
			})

			// Track for audit
			allToolsExecuted = append(allToolsExecuted, platform.ToolExecution{
				Name:  tc.Name,
				Input: tc.Arguments,
			})
		}
	}

	return nil, fmt.Errorf("remote agent exceeded maximum tool rounds (%d)", maxToolRounds)
}

// buildRemoteSystemPrompt constructs the system prompt for a remote agent
func (s *Service) buildRemoteSystemPrompt(ctx context.Context, buddy *domain.Buddy, ch *domain.Channel) string {
	var parts []string

	// Agent personality
	if buddy.Personality != nil && *buddy.Personality != "" {
		parts = append(parts, "# Personality\n"+*buddy.Personality)
	}

	// Agent system prompt
	if buddy.SystemPrompt != nil && *buddy.SystemPrompt != "" {
		parts = append(parts, "# Agent Instructions\n"+*buddy.SystemPrompt)
	}

	// Channel context
	channelCtx := s.buildChannelContext(ctx, buddy.Name, ch)
	if channelCtx != "" {
		parts = append(parts, channelCtx)
	}

	return strings.Join(parts, "\n\n")
}

// buildChannelContext creates context about the channel and its members
func (s *Service) buildChannelContext(ctx context.Context, agentName string, ch *domain.Channel) string {
	var sb strings.Builder
	sb.WriteString("# Channel Context\n")
	sb.WriteString(fmt.Sprintf("You are in channel: **%s**\n", ch.Name))
	if ch.Description != nil && *ch.Description != "" {
		sb.WriteString(fmt.Sprintf("Channel description: %s\n", *ch.Description))
	}

	// Fetch channel members for mention context
	members, err := s.membershipRepo.ListByChannel(ctx, channel.ChannelID(ch.ID.String()), shared.AllPages())
	if err != nil {
		logger.Warn("Failed to fetch channel members for remote agent context",
			slog.Any("error", err))
		return sb.String()
	}

	if len(members) > 0 {
		sb.WriteString("\n## Channel Members\n")
		sb.WriteString("You can mention members using @name syntax:\n")
		for _, m := range members {
			actorID := m.ID.ActorID
			if actorID.String() == "agent:"+agentName {
				sb.WriteString(fmt.Sprintf("- @%s (you)\n", actorID.ID()))
				continue
			}
			role := m.Role.String()
			sb.WriteString(fmt.Sprintf("- @%s (%s, %s)\n", actorID.ID(), actorID.Type(), role))
		}
	}

	return sb.String()
}

// loadChannelHistory fetches recent messages from the channel and converts them
// into SessionMessages for the agent. The current message is excluded (it's added
// as the user prompt by the caller). Messages from the same agent become "assistant"
// turns; everything else becomes "user" turns.
func loadChannelHistory(ctx context.Context, repo Repository, channelID string, currentMsgID MessageID, agentID shared.ActorID, limit int) []platform.SessionMessage {
	if repo == nil || limit <= 0 {
		return []platform.SessionMessage{}
	}

	// ListByChannel with beforeID returns messages older than currentMsgID, newest first
	page := shared.NewCursorPage(int64(currentMsgID), limit)
	result, err := repo.ListByChannel(ctx, channelID, page)
	if err != nil || result == nil || len(result.Messages) == 0 {
		return []platform.SessionMessage{}
	}

	// Messages come back in chronological order (oldest first)
	sessionMsgs := make([]platform.SessionMessage, 0, len(result.Messages))
	for _, m := range result.Messages {
		if m.Content.Text == "" {
			continue
		}
		role := "user"
		if m.AuthorID == agentID {
			role = "assistant"
		}
		sessionMsgs = append(sessionMsgs, platform.SessionMessage{
			Role:      role,
			Content:   m.Content.Text,
			Timestamp: m.CreatedAt.UnixMilli(),
		})
	}
	return sessionMsgs
}
