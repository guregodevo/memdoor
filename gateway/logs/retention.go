package logs

import (
	"context"
	"time"
)

// DefaultLogRetention is how long log events are kept when automatic
// retention is on but no override is configured.
const DefaultLogRetention = 30 * 24 * time.Hour // 30 days

// retentionSweepInterval is how often the background loop prunes by age.
const retentionSweepInterval = 6 * time.Hour

// DefaultLogMaxBytes is the ceiling on the log's size on disk; the oldest
// events go first. Checked every sizeSweepInterval: an age rule alone let
// a chatty scheduler write 40 GB in a night (2026-10-26) and 7 GB in half
// an hour (2026-10-27).
const DefaultLogMaxBytes = int64(1 << 30)

const sizeSweepInterval = 10 * time.Minute

// LogRetention is how long log events are kept.
func LogRetention() time.Duration { return DefaultLogRetention }

// StartRetention launches a background loop that prunes events older than
// retention — once shortly after boot, then every retentionSweepInterval —
// and, every sizeSweepInterval, the oldest events over maxBytes on disk.
// retention<=0 disables it (no goroutine spawned); maxBytes<=0 lifts the ceiling. The global storage is
// read at each tick, so this tolerates being called before storage is
// fully wired. Stops when ctx is cancelled.
func StartRetention(ctx context.Context, retention time.Duration, maxBytes int64) {
	if retention <= 0 {
		return
	}
	go func() {
		// Let the gateway settle before the first sweep.
		first := time.NewTimer(time.Minute)
		defer first.Stop()
		tick := time.NewTicker(retentionSweepInterval)
		defer tick.Stop()
		size := time.NewTicker(sizeSweepInterval)
		defer size.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-first.C:
				retentionSweep(ctx, retention)
				sizeSweep(ctx, maxBytes)
			case <-tick.C:
				retentionSweep(ctx, retention)
			case <-size.C:
				sizeSweep(ctx, maxBytes)
			}
		}
	}()
}

func retentionSweep(ctx context.Context, retention time.Duration) {
	s := GetGlobalStorage()
	if s == nil {
		return
	}
	if n, err := s.PruneBefore(ctx, time.Now().Add(-retention)); err == nil && n > 0 {
		New("logs-retention").Info("pruned old log events",
			"deleted", n, "retention", retention.String())
	}
}

func sizeSweep(ctx context.Context, maxBytes int64) {
	s := GetGlobalStorage()
	if s == nil || maxBytes <= 0 {
		return
	}
	if n, err := s.PruneToSize(ctx, maxBytes); err != nil {
		New("logs-retention").Warn("log size ceiling: prune failed", "error", err.Error())
	} else if n > 0 {
		New("logs-retention").Warn("log over its size ceiling: oldest events pruned",
			"deleted", n, "max_bytes", maxBytes)
	}
}
