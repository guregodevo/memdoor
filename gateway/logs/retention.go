package logs

import (
	"context"
	"time"
)

// DefaultLogRetention is how long log events are kept when automatic
// retention is on but no override is configured.
const DefaultLogRetention = 30 * 24 * time.Hour // 30 days

// retentionSweepInterval is how often the background loop prunes.
const retentionSweepInterval = 6 * time.Hour

// LogRetention is how long log events are kept.
func LogRetention() time.Duration { return DefaultLogRetention }

// StartRetention launches a background loop that prunes events older than
// retention — once shortly after boot, then every retentionSweepInterval.
// retention<=0 disables it (no goroutine spawned). The global storage is
// read at each tick, so this tolerates being called before storage is
// fully wired. Stops when ctx is cancelled.
func StartRetention(ctx context.Context, retention time.Duration) {
	if retention <= 0 {
		return
	}
	go func() {
		// Let the gateway settle before the first sweep.
		first := time.NewTimer(time.Minute)
		defer first.Stop()
		tick := time.NewTicker(retentionSweepInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-first.C:
				retentionSweep(ctx, retention)
			case <-tick.C:
				retentionSweep(ctx, retention)
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
