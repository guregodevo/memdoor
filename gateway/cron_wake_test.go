package gateway

import "testing"

// A scheduled check's answer wakes the conversation that scheduled it —
// except an ack (nothing to report) and an answer identical to the last
// run's (a poll saying "still running" every tick). OpenClaw's heartbeat
// rule, on the cron that Memdoor has instead of a periodic checklist.
func TestAScheduledChecksAnswerWakesUnlessQuiet(t *testing.T) {
	cases := []struct {
		name, text, last string
		wake             bool
	}{
		{"news wakes", "CI is green on main", "", true},
		{"news after a different answer wakes", "CI is green on main", "CI still running", true},
		{"the ack sleeps", "HEARTBEAT_OK", "", false},
		{"a short ack with trimmings sleeps", "HEARTBEAT_OK — nothing yet.", "", false},
		{"an ack buried in a report wakes", "HEARTBEAT_OK but the build log shows a new deprecation warning in pkg/x that someone should look at before Friday", "", true},
		{"the same answer twice sleeps", "CI still running", "CI still running", false},
		// The verification footer differs from run to run; the answer does not.
		{"the same answer under different footers sleeps", "READY is here.\n\n⚠ nothing changed · ran: ls -la READY PASS", "READY is here.\n\n⚠ nothing changed · ran: test -f READY PASS", false},
		{"the ack under a footer sleeps", "HEARTBEAT_OK\n\n⚠ nothing changed · ran: test -f READY && echo EXISTS || echo NO PASS · stat -f '%Sm %z bytes' READY PASS · ls -la PASS", "", false},
		{"news under a footer wakes", "READY is here.\n\n⚠ nothing changed · ran: test -f READY PASS", "HEARTBEAT_OK\n\n⚠ nothing changed · ran: test -f READY PASS", true},
	}
	// The note and the wake carry the answer alone, not the run's receipt.
	if got := cronAnswerBody("READY is here.\n\n⚠ nothing changed · ran: `test -f READY; echo \"exit=$?\"` PASS"); got != "READY is here." {
		t.Errorf("body = %q", got)
	}
	for _, c := range cases {
		wake, why := cronAnswerWakes(c.text, c.last)
		if wake != c.wake {
			t.Errorf("%s: wake=%v (%s), want %v", c.name, wake, why, c.wake)
		}
	}
}
