package domain

import "time"

// AgentTurn is the ledger record of one agent dispatch — the reliability
// contract behind the TUI: every dispatched turn MUST reach a terminal
// state, and a turn that doesn't is detectable (and announced) instead of
// silent. The unique key (channel, agent, trigger message) makes dispatch
// IDEMPOTENT: re-delivering the same trigger cannot start a second turn.
type AgentTurn struct {
	TurnID            string     `json:"turn_id"`
	ChannelID         string     `json:"channel_id"`
	AgentID           string     `json:"agent_id"`
	TriggerMessageID  int64      `json:"trigger_message_id"`
	State             TurnState  `json:"state"`
	DispatchedAt      time.Time  `json:"dispatched_at"`
	StartedAt         *time.Time `json:"started_at,omitempty"` // first inference
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	ResponseMessageID int64      `json:"response_message_id,omitempty"`
	Error             string     `json:"error,omitempty"`
}

// TurnState is the turn lifecycle — a value object, never a bare string.
type TurnState string

const (
	TurnDispatched TurnState = "dispatched" // row exists; nothing can die silently after this
	TurnRunning    TurnState = "running"    // inference began
	TurnResponded  TurnState = "responded"  // a reply message was saved
	TurnFailed     TurnState = "failed"     // errored; Error says why
	TurnLost       TurnState = "lost"       // janitor verdict: never reached a terminal state
)

// Terminal reports whether the state is an end state.
func (s TurnState) Terminal() bool {
	return s == TurnResponded || s == TurnFailed || s == TurnLost
}
