package a2a

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/shared"
)

// channelFromSessionKey extracts the channel id encoded in a
// "workspace:<ws>:channel:<id>" session key, or "" for a key that carries no
// channel (e.g. an "agent:<id>:..." subagent key). The A2A reply/announce
// prompts read the requester's and target's channels for context; deriving them
// from the session keys the message already carries beats plumbing new fields
// through every A2AMessage construction site.
func channelFromSessionKey(sessionKey string) string {
	parts := strings.Split(sessionKey, ":")
	if len(parts) >= 4 && parts[2] == "channel" {
		return parts[3]
	}
	return ""
}

// A2AHandler processes A2A messages and delivers them to target agents
// Pattern: OpenClaw src/agents/tools/sessions-send-tool.ts handleA2AMessage()
type A2AHandler struct {
	messageQueue     chan *shared.A2AMessage
	deliveryFunc     MessageDeliveryFunc
	announceFunc     AnnounceFunc
	shutdownChan     chan struct{}
	maxConcurrent    int
	defaultTimeout   time.Duration
	maxPingPongTurns int // Max ping-pong turns (0 = fire-and-forget)
}

// MessageDeliveryFunc delivers a message to a target session and returns the response
// This is injected by the gateway to integrate with the existing session system
type MessageDeliveryFunc func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error)

// AnnounceFunc announces a response back to the requester session
type AnnounceFunc func(ctx context.Context, requesterSessionKey string, announcement string) error

// A2AHandlerConfig configures the A2A handler
type A2AHandlerConfig struct {
	MessageQueue     chan *shared.A2AMessage
	DeliveryFunc     MessageDeliveryFunc
	AnnounceFunc     AnnounceFunc
	MaxConcurrent    int           // Max concurrent A2A message processing
	DefaultTimeout   time.Duration // Default timeout for A2A message processing
	MaxPingPongTurns int           // Max ping-pong turns (0 = fire-and-forget)
}

// NewA2AHandler creates a new A2A message handler
func NewA2AHandler(config A2AHandlerConfig) *A2AHandler {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10 // Default: 10 concurrent A2A messages
	}

	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	return &A2AHandler{
		messageQueue:     config.MessageQueue,
		deliveryFunc:     config.DeliveryFunc,
		announceFunc:     config.AnnounceFunc,
		shutdownChan:     make(chan struct{}),
		maxConcurrent:    config.MaxConcurrent,
		defaultTimeout:   config.DefaultTimeout,
		maxPingPongTurns: config.MaxPingPongTurns,
	}
}

// Start begins processing A2A messages
func (h *A2AHandler) Start(ctx context.Context) {
	log := logs.New("A2A")
	log.Debug("Starting A2A message handler")

	// Semaphore for concurrency control
	sem := make(chan struct{}, h.maxConcurrent)

	for {
		select {
		case <-ctx.Done():
			log.Debug("Context cancelled, shutting down")
			return

		case <-h.shutdownChan:
			log.Debug("Shutdown signal received")
			return

		case msg := <-h.messageQueue:
			// Acquire semaphore slot
			sem <- struct{}{}

			// Process message in goroutine
			go func(m *shared.A2AMessage) {
				defer func() { <-sem }() // Release semaphore

				if err := h.processMessage(ctx, m); err != nil {
					log.WithError(err).Error("Error processing message",
						slog.String("requester", m.RequesterAgentID),
						slog.String("target", m.TargetAgentID))
				}
			}(msg)
		}
	}
}

// Stop stops the A2A handler
func (h *A2AHandler) Stop() {
	close(h.shutdownChan)
}

// processMessage processes a single A2A message
func (h *A2AHandler) processMessage(ctx context.Context, msg *shared.A2AMessage) error {
	log := logs.New("A2A")
	log.Debug("Processing message", slog.String("formatted", FormatA2AMessageForLog(msg)))

	// Create timeout context
	timeout := time.Duration(msg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = h.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build A2A context prompt
	a2aPrompt := BuildA2AContextPrompt(msg)

	// Deliver initial message to target agent
	initialReply, err := h.deliveryFunc(ctx, msg.TargetSessionKey, a2aPrompt, msg.Message)
	if err != nil {
		return fmt.Errorf("failed to deliver message to %s: %w", msg.TargetAgentID, err)
	}

	log.Debug("Received initial response",
		slog.String("target", msg.TargetAgentID),
		slog.String("response", TruncateMessage(initialReply, 100)))

	// Run ping-pong loop (multi-turn conversations)
	history, err := h.runPingPongLoop(ctx, msg, initialReply)
	if err != nil {
		return fmt.Errorf("ping-pong failed: %w", err)
	}

	// Run announcement step (formatted announcement with history)
	announcement, err := h.runAnnouncementStep(ctx, msg, history)
	if err != nil {
		return fmt.Errorf("announcement failed: %w", err)
	}

	// Check if announcement should be skipped
	if announcement == "" {
		log.Debug("No announcement (ANNOUNCE_SKIP)")
		return nil
	}

	// Announce back to requester
	if err := h.announceFunc(ctx, msg.RequesterSessionKey, announcement); err != nil {
		return fmt.Errorf("failed to announce to %s: %w", msg.RequesterAgentID, err)
	}

	log.Debug("Successfully announced response", slog.String("requester", msg.RequesterAgentID))

	return nil
}

// run PingPongLoop executes multi-turn conversation between requester and target
// Pattern: OpenClaw src/agents/tools/sessions-send-tool.a2a.ts lines 60-96
func (h *A2AHandler) runPingPongLoop(
	ctx context.Context,
	msg *shared.A2AMessage,
	initialReply string,
) (*shared.ConversationHistory, error) {
	// Initialize conversation history
	history := &shared.ConversationHistory{
		OriginalMessage: msg.Message,
		RoundOneReply:   initialReply,
		LatestReply:     initialReply,
		TurnCount:       0,
	}

	// If maxPingPongTurns is 0, fire-and-forget (no ping-pong)
	if h.maxPingPongTurns == 0 {
		return history, nil
	}

	// Initialize ping-pong state
	state := &shared.PingPongState{
		CurrentSessionKey: msg.RequesterSessionKey,
		NextSessionKey:    msg.TargetSessionKey,
		IncomingMessage:   initialReply,
		History:           history,
		Turn:              1,
		MaxTurns:          h.maxPingPongTurns,
	}

	log := logs.New("A2A")
	log.Debug("Starting ping-pong loop", slog.Int("max_turns", h.maxPingPongTurns))

	// Ping-pong loop: alternate between agents
	for state.Turn <= state.MaxTurns {
		// Determine current role
		currentRole := "requester"
		if state.CurrentSessionKey == msg.TargetSessionKey {
			currentRole = "target"
		}

		log.Debug("Ping-pong turn",
			slog.Int("turn", state.Turn),
			slog.Int("max_turns", state.MaxTurns),
			slog.String("current_role", currentRole))

		// Build reply context prompt for this turn
		replyParams := shared.A2AReplyContextParams{
			RequesterSessionKey: msg.RequesterSessionKey,
			RequesterChannel:    channelFromSessionKey(msg.RequesterSessionKey),
			TargetSessionKey:    msg.TargetSessionKey,
			TargetChannel:       channelFromSessionKey(msg.TargetSessionKey),
			CurrentRole:         currentRole,
			Turn:                state.Turn,
			MaxTurns:            state.MaxTurns,
		}
		replyPrompt := BuildA2AReplyContext(replyParams)

		// Deliver message with reply context prompt
		replyText, err := h.deliveryFunc(ctx, state.CurrentSessionKey, replyPrompt, state.IncomingMessage)
		if err != nil {
			return history, fmt.Errorf("ping-pong turn %d failed: %w", state.Turn, err)
		}

		// Check for REPLY_SKIP (agent wants to end conversation)
		if shared.IsReplySkip(replyText) {
			log.Debug("Agent sent REPLY_SKIP, ending conversation", slog.Int("turn", state.Turn))
			break
		}

		// Update latest reply
		state.History.LatestReply = replyText
		state.History.TurnCount = state.Turn

		log.Debug("Received ping-pong reply",
			slog.Int("turn", state.Turn),
			slog.String("reply", TruncateMessage(replyText, 100)))

		// Swap agents for next turn
		swap := state.CurrentSessionKey
		state.CurrentSessionKey = state.NextSessionKey
		state.NextSessionKey = swap

		// Update incoming message for next turn
		state.IncomingMessage = replyText

		// Increment turn
		state.Turn++
	}

	log.Debug("Ping-pong loop completed", slog.Int("turn_count", state.History.TurnCount))

	return history, nil
}

// runAnnouncementStep executes the final announcement step
// Pattern: OpenClaw src/agents/tools/sessions-send-tool.a2a.ts lines 98-135
func (h *A2AHandler) runAnnouncementStep(
	ctx context.Context,
	msg *shared.A2AMessage,
	history *shared.ConversationHistory,
) (string, error) {
	// Build announce context prompt
	announceParams := shared.A2AAnnounceContextParams{
		RequesterSessionKey: msg.RequesterSessionKey,
		RequesterChannel:    channelFromSessionKey(msg.RequesterSessionKey),
		TargetSessionKey:    msg.TargetSessionKey,
		TargetChannel:       channelFromSessionKey(msg.TargetSessionKey),
		OriginalMessage:     history.OriginalMessage,
		RoundOneReply:       history.RoundOneReply,
		LatestReply:         history.LatestReply,
	}
	announcePrompt := BuildA2AAnnounceContext(announceParams)

	// Execute announcement step on target agent
	announceReply, err := h.deliveryFunc(ctx, msg.TargetSessionKey, announcePrompt, "Agent-to-agent announce step.")
	if err != nil {
		return "", fmt.Errorf("announcement step failed: %w", err)
	}

	// Check for ANNOUNCE_SKIP (agent wants to remain silent)
	if shared.IsAnnounceSkip(announceReply) {
		log := logs.New("A2A")
		log.Debug("Agent sent ANNOUNCE_SKIP, no announcement will be made")
		return "", nil // Empty string signals no announcement
	}

	log := logs.New("A2A")
	log.Debug("Formatted announcement", slog.String("announcement", TruncateMessage(announceReply, 100)))

	return announceReply, nil
}

// GetQueueStatus returns current queue status
func (h *A2AHandler) GetQueueStatus() QueueStatus {
	return QueueStatus{
		QueueLength:   len(h.messageQueue),
		QueueCap:      cap(h.messageQueue),
		MaxConcurrent: h.maxConcurrent,
	}
}

// QueueStatus represents A2A queue status
type QueueStatus struct {
	QueueLength   int `json:"queueLength"`
	QueueCap      int `json:"queueCap"`
	MaxConcurrent int `json:"maxConcurrent"`
}
