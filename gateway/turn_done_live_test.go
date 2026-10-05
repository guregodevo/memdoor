package gateway

import (
	"context"
	"os"
	"testing"
)

// The done check measured on Jev, with the states of two live turns that
// were judged unfinished although done (2026-10-05): a spawned child that
// wrote a file and read it back, and a scheduled check whose command exits
// 0 either way. Paid: MEMDOOR_INTEGRATION_PAID=1 runs it, on the decision
// key in the environment. It prints P; the threshold is turnDoneDefaultThreshold.
func TestTurnDoneMeasuredOnJev(t *testing.T) {
	if testing.Short() || os.Getenv("MEMDOOR_INTEGRATION_PAID") != "1" {
		t.Skip("paid: set MEMDOOR_INTEGRATION_PAID=1 to measure the done check on the decision model")
	}
	svc, err := newDecisionService(func(string) string { return "" })
	if err != nil || svc == nil {
		t.Skipf("no decision service: %v", err)
	}
	v := &turnVerdict{svc: svc, read: func(string) string { return "" }}
	cases := []struct {
		name, request, reply string
		executed             []ToolExecutionInfo
		wantUnfinished       bool
	}{
		{"a file written and read back is done",
			"Create a file hello.txt in the current working directory containing the single line \"hi\", then run \"ls -la\" to confirm it exists. Report back the confirmation.",
			"Created `hello.txt` (3 bytes) and confirmed via `ls -la` that it exists in the working directory. Confirmed: `hello.txt` contains exactly \"hi\".",
			[]ToolExecutionInfo{
				{Name: "write_file", Input: `{"path":"hello.txt","content":"hi\n"}`, Output: "ok"},
				{Name: "bash", Input: `{"command":"ls -la hello.txt && cat hello.txt"}`, Output: "-rw-r--r--@ 1 gregoryd  wheel  3 Oct  5 00:12 hello.txt\nhi\n"},
			}, false},
		{"a check that found nothing yet is done",
			"Does a file named READY exist in this directory? Check with: test -f READY. If it does NOT exist, answer exactly HEARTBEAT_OK. If it exists, say: READY is here.",
			"HEARTBEAT_OK",
			[]ToolExecutionInfo{
				{Name: "bash", Input: `{"command":"test -f READY && echo EXISTS || echo NO"}`, Output: "NO\n"},
			}, false},
		{"a check that found it is done",
			"Does a file named READY exist in this directory? Check with: test -f READY. If it does NOT exist, answer exactly HEARTBEAT_OK. If it exists, say: READY is here.",
			"READY is here.",
			[]ToolExecutionInfo{
				{Name: "bash", Input: `{"command":"test -f READY && echo EXISTS || echo NO"}`, Output: "EXISTS\n"},
			}, false},
		{"a change with nothing after it and a claim is unfinished",
			"Fix the nil pointer in sum.go and make sure the tests pass.",
			"Fixed the nil pointer; all tests pass now.",
			[]ToolExecutionInfo{
				{Name: "write_file", Input: `{"path":"sum.go","content":"package x"}`, Output: "ok"},
			}, true},
	}
	for _, c := range cases {
		r := receiptsOf(c.executed)
		p, retry := v.unfinished(context.Background(), "coder", c.request, c.reply, r)
		t.Logf("%-55s P(unfinished)=%.2f retry=%v  checkedAfterChange=%v\n    evidence: %q", c.name, p, retry, r.checkedAfterChange, r.evidence())
		if retry != c.wantUnfinished {
			t.Errorf("%s: retry=%v, want %v (P=%.2f)", c.name, retry, c.wantUnfinished, p)
		}
	}
}
