package streaming

import (
	"memdoor/gateway/broadcast"
	"memdoor/gateway/infra"
)

// BroadcastBridge connects the streaming EventEmitter to the WebSocket broadcaster
// Pattern: OpenClaw event integration
type BroadcastBridge struct {
}

// ConnectInfraEventEmitter connects infra.EventEmitter to broadcast via streaming
// This allows existing infra events to be streamed via WebSocket
func ConnectInfraEventEmitter(infraEmitter *infra.EventEmitter, broadcaster *broadcast.SubscriptionManager, verbose bool) func() {
	// Register listener on infra.EventEmitter
	unregister := infraEmitter.OnEvent(func(event infra.AgentEvent) {
		// Convert infra.AgentEvent to broadcast format
		broadcastEvent := broadcast.Event{
			Type: "agent_event",
			Data: map[string]interface{}{
				"run_id":     event.RunID,
				"seq":        event.Seq,
				"stream":     string(event.Stream),
				"timestamp":  event.Timestamp,
				"data":       event.Data,
				"session_id": event.SessionID,
			},
		}

		// Broadcast to run subscribers
		broadcaster.BroadcastToRun(event.RunID, broadcastEvent)

		// Also broadcast to session subscribers (for TUI and other session-based clients)
		// Pattern: OpenClaw broadcasts to both runs and sessions
		if event.SessionID != "" {
			broadcaster.BroadcastToSession(event.SessionID, broadcastEvent)
		}
	})

	return unregister
}
