package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"memdoor/pkg/domain"
	"memdoor/pkg/repository"
)

type agentTurnRepository struct {
	db *sql.DB
}

func NewAgentTurnRepository(db *sql.DB) repository.AgentTurnRepository {
	return &agentTurnRepository{db: db}
}

// Begin records a dispatch idempotently: the UNIQUE(channel, agent,
// trigger) key means re-delivering the same trigger returns the existing
// turn with created=false instead of starting a second one.
func (r *agentTurnRepository) Begin(ctx context.Context, t *domain.AgentTurn) (created bool, err error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO agent_turns
		 (turn_id, channel_id, agent_id, trigger_message_id, state, dispatched_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.TurnID, t.ChannelID, t.AgentID, t.TriggerMessageID,
		string(domain.TurnDispatched), t.DispatchedAt.UnixMilli())
	if err != nil {
		return false, fmt.Errorf("begin agent turn: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SetState is an IDEMPOTENT transition: `running` applies only from
// `dispatched`, and the FIRST terminal state wins — a late failure hook
// cannot downgrade a recorded response, and replaying any hook is a no-op.
func (r *agentTurnRepository) SetState(ctx context.Context, turnID string, state domain.TurnState, errMsg string, responseMessageID int64) error {
	now := time.Now().UnixMilli()
	var started, ended interface{}
	guard := `state NOT IN ('responded','failed','lost')` // terminal is final
	if state == domain.TurnRunning {
		started = now
		guard = `state = 'dispatched'` // running only from dispatched
	}
	if state.Terminal() {
		ended = now
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE agent_turns SET state=?,
		   started_at=COALESCE(started_at, ?),
		   ended_at=COALESCE(?, ended_at),
		   error=CASE WHEN ?='' THEN error ELSE ? END,
		   response_message_id=CASE WHEN ?=0 THEN response_message_id ELSE ? END
		 WHERE turn_id=? AND `+guard,
		string(state), started, ended, errMsg, errMsg, responseMessageID, responseMessageID, turnID)
	if err != nil {
		return fmt.Errorf("set turn state: %w", err)
	}
	return nil
}

// Stuck returns non-terminal turns dispatched before the cutoff — the
// janitor's query: these are the silent failures.
func (r *agentTurnRepository) Stuck(ctx context.Context, before time.Time) ([]*domain.AgentTurn, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT turn_id, channel_id, agent_id, trigger_message_id, state, dispatched_at,
		        started_at, ended_at, response_message_id, error
		 FROM agent_turns
		 WHERE state IN ('dispatched','running') AND dispatched_at < ?`,
		before.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query stuck turns: %w", err)
	}
	defer rows.Close()
	return scanTurns(rows)
}

// Since returns turns in a channel dispatched at/after the cutoff —
// reconnect reconciliation for clients that missed events.
func (r *agentTurnRepository) Since(ctx context.Context, channelID string, since time.Time) ([]*domain.AgentTurn, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT turn_id, channel_id, agent_id, trigger_message_id, state, dispatched_at,
		        started_at, ended_at, response_message_id, error
		 FROM agent_turns
		 WHERE channel_id=? AND dispatched_at >= ?
		 ORDER BY dispatched_at ASC`,
		channelID, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query turns since: %w", err)
	}
	defer rows.Close()
	return scanTurns(rows)
}

func scanTurns(rows *sql.Rows) ([]*domain.AgentTurn, error) {
	var out []*domain.AgentTurn
	for rows.Next() {
		var t domain.AgentTurn
		var state string
		var dispatched int64
		var started, ended sql.NullInt64
		if err := rows.Scan(&t.TurnID, &t.ChannelID, &t.AgentID, &t.TriggerMessageID,
			&state, &dispatched, &started, &ended, &t.ResponseMessageID, &t.Error); err != nil {
			return nil, err
		}
		t.State = domain.TurnState(state)
		t.DispatchedAt = time.UnixMilli(dispatched)
		if started.Valid {
			ts := time.UnixMilli(started.Int64)
			t.StartedAt = &ts
		}
		if ended.Valid {
			ts := time.UnixMilli(ended.Int64)
			t.EndedAt = &ts
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}
