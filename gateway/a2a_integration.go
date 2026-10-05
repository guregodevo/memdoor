package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"memdoor/gateway/a2a"
	"memdoor/gateway/config"
	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
	"memdoor/gateway/tools"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// WireSessionsSendTool wires the sessions_send tool into the agent runtime
// Pattern: Post-construction dependency injection (same as sessions_spawn)
func (ar *AgentRuntime) WireSessionsSendTool(
	policy *authorization.A2APolicy,
	resolver *a2a.SessionResolver,
	messageQueue chan *shared.A2AMessage,
	requesterAgentID string,
) error {
	log := logs.New("A2A")

	log.Debug("Wiring sessions_send tool",
		slog.String("agent_id", requesterAgentID))

	// Create SessionsSendTool instance
	ar.sessionsSend = tools.NewSessionsSendTool(policy, resolver, messageQueue, requesterAgentID, ar.verbose)

	log.Debug("sessions_send tool wired successfully")

	return nil
}

// InitializeA2A initializes the A2A messaging system
// Returns: (handler, messageQueue, policy, resolver, error)
func (s *Server) InitializeA2A() (*a2a.A2AHandler, chan *shared.A2AMessage, *authorization.A2APolicy, *a2a.SessionResolver, error) {
	log := logs.New("A2A")

	// Load A2A policy from config
	policy := loadA2APolicy(s.cfg)

	if policy.Enabled {
		log.Debug("Policy enabled",
			slog.Int("rules", len(policy.Allow)))
	} else {
		log.Debug("Policy disabled (deny all)")
	}

	// Create resolver
	resolver := a2a.NewSessionResolver(s.verbose)

	// Create message queue
	messageQueue := make(chan *shared.A2AMessage, 100)

	// Create delivery function (delivers A2A messages to target sessions)
	deliveryFunc := func(ctx context.Context, targetSessionKey string, a2aPrompt string, message string) (string, error) {
		log.Debug("Delivering message to session",
			slog.String("session", targetSessionKey))

		// Create response channel
		responseChan := make(chan string, 1)
		errorChan := make(chan error, 1)

		// Create response writer that captures the response
		responseWriter := func(response interface{}) error {
			if responseMap, ok := response.(map[string]interface{}); ok {
				if text, ok := responseMap["text"].(string); ok {
					responseChan <- text
					return nil
				}
				if err, ok := responseMap["error"].(string); ok {
					errorChan <- fmt.Errorf("%s", err)
					return nil
				}
			}
			errorChan <- fmt.Errorf("invalid response format")
			return nil
		}

		// Build full message with A2A context prompt
		fullMessage := a2aPrompt + "\n\n" + message

		// Enqueue job for target agent
		job := &queue.AgentJob{
			SessionKey:     targetSessionKey,
			Message:        fullMessage,
			GlobalLane:     queue.LaneNested, // Use nested lane for A2A messages
			EnqueueTime:    time.Now(),
			Context:        ctx,
			ResponseWriter: responseWriter,
		}

		if err := s.queueManager.EnqueueJob(job); err != nil {
			return "", fmt.Errorf("failed to enqueue A2A message: %w", err)
		}

		// Wait for response with timeout
		timeout := 30 * time.Second
		select {
		case response := <-responseChan:
			return response, nil
		case err := <-errorChan:
			return "", err
		case <-time.After(timeout):
			return "", fmt.Errorf("timeout waiting for response from %s", targetSessionKey)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// Create announce function (announces responses back to requester)
	announceFunc := func(ctx context.Context, requesterSessionKey string, announcement string) error {
		log.Debug("Announcing to session",
			slog.String("session", requesterSessionKey))

		// Enqueue announcement as a new message to requester
		job := &queue.AgentJob{
			SessionKey:  requesterSessionKey,
			Message:     announcement,
			GlobalLane:  queue.LaneMain,
			EnqueueTime: time.Now(),
			Context:     ctx,
		}

		return s.queueManager.EnqueueJob(job)
	}

	// Create A2A handler with ping-pong configuration
	handler := a2a.NewA2AHandler(a2a.A2AHandlerConfig{
		MessageQueue:     messageQueue,
		DeliveryFunc:     deliveryFunc,
		AnnounceFunc:     announceFunc,
		MaxPingPongTurns: policy.GetMaxPingPongTurns(), // Week 28: Multi-turn conversations
	})

	log.Debug("A2A handler initialized",
		slog.Int("max_ping_pong_turns", policy.GetMaxPingPongTurns()))

	return handler, messageQueue, policy, resolver, nil
}

// loadA2APolicy loads A2A policy from config
func loadA2APolicy(cfg *config.Config) *authorization.A2APolicy {
	if cfg == nil || cfg.Tools == nil || cfg.Tools.AgentToAgent == nil {
		return authorization.DefaultA2APolicy()
	}

	a2aCfg := cfg.Tools.AgentToAgent

	policy := &authorization.A2APolicy{
		Enabled:          a2aCfg.Enabled,
		MaxPingPongTurns: a2aCfg.MaxPingPongTurns,
		Allow:            make([]authorization.A2ARule, 0, len(a2aCfg.Allow)),
	}

	for _, rule := range a2aCfg.Allow {
		policy.Allow = append(policy.Allow, authorization.A2ARule{
			From: rule.From,
			To:   rule.To,
		})
	}

	return policy
}
