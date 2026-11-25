package gateway

import (
	"context"
	"encoding/json"
	"memdoor/tools"
	"strings"
	"testing"
	"time"
)

// The streamed bash call waits the one-shot tool's time limit
// (tools.BashTimeout), not 30 s of its own, and then returns.
func TestStreamedBashUsesTheBashTimeout(t *testing.T) {
	old := bashStreamMaxDuration
	bashStreamMaxDuration = 0
	defer func() { bashStreamMaxDuration = old }()
	oldLimit := tools.BashTimeoutLimit
	tools.BashTimeoutLimit = time.Second
	defer func() { tools.BashTimeoutLimit = oldLimit }()

	ctx, endTurn := withTurnEnd(context.Background())
	defer endTurn()
	in, _ := json.Marshal(map[string]string{"command": "echo started; sleep 5"})
	start := time.Now()
	out, err := bashStreamer{}.Stream(ctx, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 4*time.Second {
		t.Fatalf("the 1 s limit was not applied: took %s", took)
	}
	for _, want := range []string{"started", "still running after 1s"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
