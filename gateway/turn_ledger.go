package gateway

import (
	"context"
	"fmt"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
	"memdoor/pkg/shared"
)

// turnLedger adapts the sqlite AgentTurnRepository to the message
// service's duck-typed TurnLedger port. Recording must never break a turn:
// failures log and continue.
type turnLedger struct {
	repo repository.AgentTurnRepository
}

func newTurnLedger(repo repository.AgentTurnRepository) *turnLedger {
	return &turnLedger{repo: repo}
}

func (l *turnLedger) BeginTurn(ctx context.Context, channelID, agentID string, triggerMessageID int64) (string, bool, error) {
	t := &domain.AgentTurn{
		TurnID:           fmt.Sprintf("turn_%d_%s", time.Now().UnixNano(), agentID),
		ChannelID:        channelID,
		AgentID:          agentID,
		TriggerMessageID: triggerMessageID,
		State:            domain.TurnDispatched,
		DispatchedAt:     time.Now(),
	}
	created, err := l.repo.Begin(ctx, t)
	if err != nil {
		return "", false, err
	}
	return t.TurnID, created, nil
}

func (l *turnLedger) TurnRunning(ctx context.Context, turnID string) {
	if err := l.repo.SetState(ctx, turnID, domain.TurnRunning, "", 0); err != nil {
		logs.New("Turns").Warn("ledger running-mark failed: " + err.Error())
	}
}

func (l *turnLedger) TurnResponded(ctx context.Context, turnID string, responseMessageID int64) {
	if err := l.repo.SetState(ctx, turnID, domain.TurnResponded, "", responseMessageID); err != nil {
		logs.New("Turns").Warn("ledger responded-mark failed: " + err.Error())
	}
}

func (l *turnLedger) TurnFailed(ctx context.Context, turnID string, errMsg string) {
	if err := l.repo.SetState(ctx, turnID, domain.TurnFailed, errMsg, 0); err != nil {
		logs.New("Turns").Warn("ledger failed-mark failed: " + err.Error())
	}
}

// turnJanitorInterval and turnStuckAfter bound how long a turn may sit
// non-terminal before it is declared LOST and announced. The deadline must
// exceed the longest healthy turn (a slow model writing a big patch ran
// 7 minutes live).
const (
	turnJanitorInterval = 2 * time.Minute
	turnStuckAfter      = 20 * time.Minute
)

// lostTurns filters the deadline's candidates down to turns that are truly
// dead. A deadline alone cannot tell a dead turn from a long one: measured
// live 2026-08-31, a turn 22 minutes in — two long generations, a truncation
// recovery, tool calls still landing — was declared lost and a "re-send"
// announcement posted while it worked. active reports whether a turn for
// (channel, agent) is executing right now (the turn gate is held); a running
// turn is left alone and the next sweep re-examines it. Turns orphaned by a
// restart hold nothing and are still caught. A nil probe falls back to the
// bare deadline — losing the probe must not silence the janitor.
func lostTurns(stuck []*domain.AgentTurn, active func(channelID, agentID string) bool) []*domain.AgentTurn {
	if active == nil {
		return stuck
	}
	var lost []*domain.AgentTurn
	for _, t := range stuck {
		if active(t.ChannelID, t.AgentID) {
			logs.New("Turns").Info(fmt.Sprintf("turn past deadline but still executing — left alone (agent %s, channel %s)",
				t.AgentID, t.ChannelID))
			continue
		}
		lost = append(lost, t)
	}
	return lost
}

// startTurnJanitor makes silence impossible: any turn still non-terminal
// past the deadline is marked LOST and ANNOUNCED into its channel — the
// user learns a reply died instead of wondering.
func (s *Server) startTurnJanitor() {
	if s.repoFactory == nil {
		return
	}
	repo := s.repoFactory.AgentTurns()
	go func() {
		ticker := time.NewTicker(turnJanitorInterval)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			stuck, err := repo.Stuck(ctx, time.Now().Add(-turnStuckAfter))
			if err != nil {
				cancel()
				continue
			}
			// Duck-typed probe, same pattern as SystemAnnouncementPoster: the
			// interface is declared here in the consumer, pkg/message satisfies
			// it without knowing.
			var active func(string, string) bool
			if probe, ok := s.messageService.(interface {
				TurnActive(channelID, agentID string) bool
			}); ok {
				active = probe.TurnActive
			}
			for _, t := range lostTurns(stuck, active) {
				_ = repo.SetState(ctx, t.TurnID, domain.TurnLost, "no terminal state within deadline", 0)
				logs.New("Turns").Warn(fmt.Sprintf("turn LOST: agent %s in channel %s never completed (trigger msg %d, dispatched %s)",
					t.AgentID, t.ChannelID, t.TriggerMessageID, t.DispatchedAt.Format("15:04:05")))
				if s.messageService != nil {
					actor := shared.NewAgentActorID("deployer")
					_ = s.messageService.PostSystemAnnouncement(ctx, t.ChannelID, actor,
						fmt.Sprintf("⚠ The reply from **%s** to message %d was lost (no completion within %d min). Re-send the message to retry.",
							t.AgentID, t.TriggerMessageID, int(turnStuckAfter.Minutes())), int64(t.TriggerMessageID))
				}
			}
			cancel()
		}
	}()
}
