package message

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
)

// unansweredMention is the name a turn opens with ("@name …") when it
// resolved to nobody: not an agent here, not a peer's.
func unansweredMention(userMsg *Message) (string, bool) {
	text := strings.TrimSpace(userMsg.Content.Text)
	if !strings.HasPrefix(text, "@") {
		return "", false
	}
	names := ParseMentions(text)
	if len(names) == 0 {
		return "", false
	}
	first := names[0]
	for _, m := range userMsg.Content.Mentions {
		if strings.EqualFold(m.Name, first) || strings.EqualFold(m.ActorID.ID(), first) {
			return "", false
		}
	}
	return first, true
}

// askedByPeer: another gateway asked for this turn through a peer mention.
// Its reply is posted in that gateway's thread, so the names in it are that
// gateway's to resolve; nothing is woken here.
func askedByPeer(ctx context.Context) bool {
	asked, _ := ctx.Value(sharedctx.AskedByPeerKey).(bool)
	return asked
}

// asyncTurnContext is the turn's context detached from its lifetime (the
// turn's ctx is canceled when it ends) with what the next turn needs from
// it: who and where (the execution context, with its mention depth), the
// project and the mode. A turn woken by an agent's reply used to get the
// execution context alone and run in the scratch folder instead of the
// thread's project.
func asyncTurnContext(ctx context.Context) context.Context {
	next := context.Background()
	if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
		next = shared.WithExecutionContext(next, execCtx)
	}
	if wd, _ := ctx.Value(sharedctx.WorkdirKey).(string); wd != "" {
		next = context.WithValue(next, sharedctx.WorkdirKey, wd)
	}
	if mode, _ := ctx.Value(sharedctx.PermissionModeKey).(string); mode != "" {
		next = context.WithValue(next, sharedctx.PermissionModeKey, mode)
	}
	return next
}

// peerAskText is what the peer's agent is asked: the turn without the
// mention of itself, and where the reply goes, so that it can address an
// agent of the asking gateway by name (that gateway resolves the reply's
// mentions, in the thread the reply lands in).
func peerAskText(userMsg *Message, peer, agent string) string {
	text := strings.TrimSpace(strings.ReplaceAll(userMsg.Content.Text, "@"+peer+":"+agent, ""))
	who := "a person"
	if userMsg.AuthorID.IsAgent() {
		who = "the agent " + userMsg.AuthorID.ID()
	}
	return text + fmt.Sprintf("\n\n(Asked from another gateway by %s, as @%s:%s. Your reply is posted in the thread there, as %s:%s; "+
		"a @name in it addresses an agent of that gateway, @<peer>:<agent> one of its peers, and wakes nobody here.)",
		who, peer, agent, peer, agent)
}

// finishPeerTurn is a mentioned peer agent's turn: the text, without the
// mention itself, goes to the peer; what comes back is saved as the
// thread's reply from that agent and broadcast, as a local agent's answer
// is, and the names in it wake agents here as a local agent's reply would
// (the mention depth bounds the exchange, as it bounds any A2A chain).
func (s *Service) finishPeerTurn(ctx context.Context, userMsg *Message, agentID shared.ActorID, runID, turnID, sessionKey, peer, agent string) error {
	if s.turnLedger != nil && turnID != "" {
		s.turnLedger.TurnRunning(ctx, turnID)
	}
	logger.Info("Asking a peer's agent",
		slog.String("peer", peer), slog.String("agent", agent), slog.String("channel_id", userMsg.ChannelID),
		slog.String("asked_by", userMsg.AuthorID.String()))
	reply, err := s.peers.Ask(ctx, peer, agent, peerAskText(userMsg, peer, agent), sessionKey, runID)
	var mentions []Mention
	if err != nil {
		s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "failed", map[string]interface{}{"error": err.Error()})
		reply = fmt.Sprintf("%s did not answer: %v", agentID.ID(), err)
	} else {
		mentions = s.peerReplyMentions(ctx, userMsg.ChannelID, agentID, reply)
	}
	msg, merr := NewReply(userMsg.ChannelID, agentID, MessageContent{Text: reply, Mentions: mentions}, userMsg.GetThreadRootID())
	if merr != nil {
		return merr
	}
	saveCtx := context.Background()
	if execCtx := shared.GetExecutionContext(ctx); execCtx != nil {
		saveCtx = shared.WithExecutionContext(saveCtx, execCtx)
	}
	id, serr := s.messageRepo.Save(saveCtx, msg)
	if serr != nil {
		return fmt.Errorf("could not save %s's reply: %w", agentID.ID(), serr)
	}
	msg.ID = id
	if s.broadcaster != nil {
		s.broadcaster(msg)
	}
	if err != nil {
		return fmt.Errorf("peer %s: %w", peer, err)
	}
	s.broadcastExecutionEvent(userMsg.ChannelID, agentID, runID, "completed", map[string]interface{}{"message_id": id.String()})
	if s.turnLedger != nil && turnID != "" {
		s.turnLedger.TurnResponded(ctx, turnID, int64(id))
	}
	if msg.HasMentions() {
		logger.Info("A peer's reply mentions agents here, triggering A2A communication",
			slog.String("agent_id", agentID.String()), slog.Int("mentions_count", len(mentions)))
		go s.HandleAgentMentions(asyncTurnContext(ctx), msg)
	}
	return nil
}

// peerReplyMentions are the names a peer's reply addresses, resolved here:
// agents of this gateway, people, this gateway's peers; never the peer
// agent itself.
func (s *Service) peerReplyMentions(ctx context.Context, channelID string, self shared.ActorID, reply string) []Mention {
	resolved := s.resolveMentions(ctx, reply)
	others := make([]Mention, 0, len(resolved))
	for _, m := range resolved {
		if m.ActorID != self {
			others = append(others, m)
		}
	}
	if len(others) == 0 {
		return nil
	}
	authorized, unauthorized, err := s.validateMentions(ctx, channelID, others)
	if err != nil {
		logger.Warn("Could not validate a peer reply's mentions", slog.String("error", err.Error()))
		return nil
	}
	if len(unauthorized) > 0 {
		logger.Warn("A peer reply's mentions left out", slog.Any("mentions", unauthorized))
	}
	return authorized
}
