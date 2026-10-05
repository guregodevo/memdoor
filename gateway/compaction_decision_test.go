package gateway

import (
	"testing"

	"memdoor/gateway/compaction"
	ctxmgmt "memdoor/gateway/context"
)

func TestCompactionDecision(t *testing.T) {
	over := &ctxmgmt.CheckResult{CanProceed: false, ShouldCompact: true, Utilization: 99}
	hot := &ctxmgmt.CheckResult{CanProceed: true, ShouldCompact: true, Utilization: 50}
	cool := &ctxmgmt.CheckResult{CanProceed: true, ShouldCompact: false, Utilization: 10}
	cases := []struct {
		name    string
		check   *ctxmgmt.CheckResult
		midTurn bool
		want    compaction.CompactionType
	}{
		{"nil check", nil, false, compaction.CompactionNone},
		{"over the limit compacts even mid-turn", over, true, compaction.CompactionOverLimit},
		{"threshold at turn start", hot, false, compaction.CompactionThreshold},
		{"threshold deferred mid-turn", hot, true, compaction.CompactionNone},
		{"cool", cool, false, compaction.CompactionNone},
		// No floor below the threshold for a codebase agent: a 20% floor
		// compacted a 1M window at 200K.
		{"a doer under the threshold is not compacted", &ctxmgmt.CheckResult{CanProceed: true, Utilization: 25}, false, compaction.CompactionNone},
	}
	for _, c := range cases {
		got, why := compactionDecision(c.check, c.midTurn)
		if got != c.want {
			t.Errorf("%s: got %v (%s), want %v", c.name, got, why, c.want)
		}
	}
}
