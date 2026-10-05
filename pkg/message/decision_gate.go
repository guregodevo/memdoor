package message

import (
	"context"
	"log/slog"

	"memdoor/pkg/channel"
	"memdoor/pkg/shared"
)

// MessageGate decides whether an agent that nobody addressed should still
// take a turn on a channel message. Duck-typed: the gateway implements it
// over the decision model; the message service only asks.
//
// ok=false means the gate could not decide (no decision model configured,
// provider unavailable, deadline). That is never treated as "respond": an
// unaddressed message with no working gate goes to no one, exactly as before
// the gate existed.
type MessageGate interface {
	ShouldRespond(ctx context.Context, agentID shared.ActorID, msg *Message, recent []*Message) (respond bool, ok bool)
}

// gateContextMessages is how much channel history the gate sees before the
// message it judges — enough to tell a continuing conversation from a stray
// remark, small enough to stay one cheap prefill.
const gateContextMessages = 8

// SetMessageGate enables gated turns for unaddressed messages. Nil (the
// default) keeps the historical behavior: agents run only when @mentioned,
// in DMs, or as a thread lead.
func (s *Service) SetMessageGate(g MessageGate) { s.messageGate = g }

// gateCandidate is true for a message the gate may route: a gate is attached,
// a human wrote it, and it is not a thread reply (threads have a lead).
func (s *Service) gateCandidate(msg *Message) bool {
	return s.messageGate != nil && !msg.AuthorID.IsAgent() && msg.ParentID == nil
}

// gatedAgents returns the agent members of the message's channel that the
// gate says should respond to a human message no one addressed. Agent-
// authored messages are never gated: mentions already carry the A2A depth
// limit, and a gate would let two agents talk forever without one.
func (s *Service) gatedAgents(ctx context.Context, userMsg *Message) []shared.ActorID {
	if !s.gateCandidate(userMsg) {
		return nil
	}
	members, err := s.membershipRepo.ListByChannel(ctx, channel.ChannelID(userMsg.ChannelID), shared.AllPages())
	if err != nil {
		logger.Warn("Message gate: cannot list channel members", slog.String("error", err.Error()))
		return nil
	}
	var candidates []shared.ActorID
	for _, m := range members {
		if m.ID.ActorID.IsAgent() && m.ID.ActorID != userMsg.AuthorID {
			candidates = append(candidates, m.ID.ActorID)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	var recent []*Message
	if page, err := s.messageRepo.ListByChannel(ctx, userMsg.ChannelID, shared.NewCursorPage(int64(userMsg.ID), gateContextMessages)); err == nil {
		recent = page.Messages
	}
	var out []shared.ActorID
	for _, agentID := range candidates {
		respond, ok := s.messageGate.ShouldRespond(ctx, agentID, userMsg, recent)
		logger.Info("Message gate",
			slog.String("agent_id", agentID.String()),
			slog.Int64("message_id", int64(userMsg.ID)),
			slog.Bool("decided", ok),
			slog.Bool("respond", respond))
		if ok && respond {
			out = append(out, agentID)
		}
	}
	return out
}
