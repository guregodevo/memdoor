package a2a

import (
	"context"
	"strings"
	"sync"
	"testing"

	"memdoor/pkg/shared"
)

// This drives the REAL A2A handler path (processMessage → runPingPongLoop →
// BuildA2AReplyContext) with a stub delivery func, and asserts the requester's
// and target's channels — derived from the session keys via channelFromSessionKey
// — actually reach the prompt the target agent sees. It is the deterministic
// stand-in for a live A2A run: a model won't reliably emit an
// agent-to-agent @mention, so this exercises the delivery hop directly.
//
// Before the fix these were hardcoded "" and never appeared in any prompt.
func TestA2AChannelsReachTheReplyPrompt(t *testing.T) {
	var mu sync.Mutex
	var prompts []string

	h := NewA2AHandler(A2AHandlerConfig{
		MessageQueue: make(chan *shared.A2AMessage, 1),
		DeliveryFunc: func(_ context.Context, _ string, a2aPrompt string, _ string) (string, error) {
			mu.Lock()
			prompts = append(prompts, a2aPrompt)
			mu.Unlock()
			return "ok, continuing", nil // not REPLY_SKIP → the ping-pong turn runs
		},
		AnnounceFunc:     func(_ context.Context, _ string, _ string) error { return nil },
		MaxPingPongTurns: 1, // one ping-pong turn → the reply-context prompt is built
		MaxConcurrent:    1,
	})

	msg := &shared.A2AMessage{
		RequesterSessionKey: "workspace:acme:channel:general",
		RequesterAgentID:    "companion",
		TargetSessionKey:    "workspace:acme:channel:ops",
		TargetAgentID:       "researcher",
		Message:             "please acknowledge",
		TimeoutSeconds:      5,
	}

	if err := h.processMessage(context.Background(), msg); err != nil {
		t.Fatalf("processMessage: %v", err)
	}

	mu.Lock()
	all := strings.Join(prompts, "\n---\n")
	mu.Unlock()

	if !strings.Contains(all, "Agent 1 (requester) channel: general.") {
		t.Errorf("requester channel 'general' never reached the A2A prompt.\nPrompts:\n%s", all)
	}
	if !strings.Contains(all, "Agent 2 (target) channel: ops.") {
		t.Errorf("target channel 'ops' never reached the A2A prompt.\nPrompts:\n%s", all)
	}
}
