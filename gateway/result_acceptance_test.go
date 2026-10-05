package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/subagents"
	"memdoor/pkg/decision"
)

type fakeAcceptSvc struct {
	p    float64
	unav bool
}

func (f *fakeAcceptSvc) Evaluate(_ context.Context, _ decision.Request, _ decision.Options) decision.Result {
	if f.unav {
		return decision.Unavailable(decision.ReasonNotConfigured, "test")
	}
	return decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{
		"unfinished": {Kind: decision.KindBoolean, ProbabilityTrue: f.p},
	}}
}

type fakeMeta map[string]interface{}

func (m fakeMeta) SetMetadata(k string, v interface{})           { m[k] = v }
func (m fakeMeta) GetMetadataValue(k string) (interface{}, bool) { v, ok := m[k]; return v, ok }

type fakeRuns struct {
	reactivated []string
	registered  []*subagents.SubagentRunRecord
}

func (r *fakeRuns) ReactivateChildSession(k string) { r.reactivated = append(r.reactivated, k) }
func (r *fakeRuns) Register(rec *subagents.SubagentRunRecord) error {
	r.registered = append(r.registered, rec)
	return nil
}

type fakeEnqueue struct {
	runID, session, agent, message, workdir string
	tools                                   []string
	timeout                                 time.Duration
	calls                                   int
}

func (e *fakeEnqueue) EnqueueSubagentJob(runID, sessionKey, agentID, message string, tools []string, systemPrompt, workdir string, timeout time.Duration) error {
	e.calls++
	e.runID, e.session, e.agent, e.message, e.tools, e.workdir, e.timeout = runID, sessionKey, agentID, message, tools, workdir, timeout
	return nil
}

func acceptanceUnderTest(p float64, meta fakeMeta, set map[string]string) (*resultAcceptance, *fakeRuns, *fakeEnqueue) {
	runs, enq := &fakeRuns{}, &fakeEnqueue{}
	a := &resultAcceptance{
		svc:     &fakeAcceptSvc{p: p},
		read:    func(k string) string { return set[k] },
		runs:    runs,
		enqueue: enq,
		session: func(string) (sessionMeta, error) { return meta, nil },
		resolve: func(string) ([]string, string) { return []string{"bash", "read_file"}, "coder prompt" },
	}
	return a, runs, enq
}

func record() *subagents.SubagentRunRecord {
	return &subagents.SubagentRunRecord{
		RunID:               "run-1",
		ChildSessionKey:     "agent:coder:subagent:run-1",
		RequesterSessionKey: "agent:planner:main",
		RequesterDisplayKey: "Agent planner",
		Task:                "Create the kvstore module and make go test -race pass.",
		Cleanup:             "delete",
		Outcome:             &subagents.SubagentOutcome{Status: subagents.OutcomeOK},
	}
}

// The first unfinished result goes back to the child: same session, same
// task, the agent read from the child's key, the requester's workdir.
func TestResultAcceptanceHandsBackOnce(t *testing.T) {
	meta := fakeMeta{"requester_workdir": "/work/here"}
	a, runs, enq := acceptanceUnderTest(0.96, meta, nil)
	rv := a.review(context.Background(), record(), "I will create store.go, then the tests, then the CLI.")
	if !rv.Judged || !rv.Unfinished || !rv.HandedBack || rv.Note != "" {
		t.Fatalf("first review: %+v", rv)
	}
	if len(runs.reactivated) != 1 || runs.reactivated[0] != "agent:coder:subagent:run-1" {
		t.Fatalf("reactivated %v", runs.reactivated)
	}
	if len(runs.registered) != 1 || runs.registered[0].ChildSessionKey != "agent:coder:subagent:run-1" ||
		runs.registered[0].Task != record().Task || runs.registered[0].RunID == "run-1" {
		t.Fatalf("registered %+v", runs.registered)
	}
	if enq.calls != 1 || enq.agent != "coder" || enq.session != "agent:coder:subagent:run-1" || enq.workdir != "/work/here" ||
		!strings.Contains(enq.message, record().Task) || !strings.Contains(enq.message, "judged unfinished") || enq.runID != runs.registered[0].RunID {
		t.Fatalf("enqueue %+v", enq)
	}
	if _, ok := meta[acceptanceHandbackKey]; !ok {
		t.Fatal("hand-back not recorded on the session")
	}

	// The second result is announced whatever the judge says, with a note.
	rv = a.review(context.Background(), record(), "Still planning: next I write the tests.")
	if rv.HandedBack || enq.calls != 1 || !strings.Contains(rv.Note, "still judged UNFINISHED") {
		t.Fatalf("second review: %+v (enqueue calls %d)", rv, enq.calls)
	}
	a.svc = &fakeAcceptSvc{p: 0.05}
	rv = a.review(context.Background(), record(), "Done: ok kvstore/store 1.4s")
	if rv.HandedBack || rv.Unfinished || !strings.Contains(rv.Note, "now judged done") {
		t.Fatalf("accepted after hand-back: %+v", rv)
	}
}

func TestResultAcceptanceLeavesFinishedAlone(t *testing.T) {
	a, runs, enq := acceptanceUnderTest(0.08, fakeMeta{}, nil)
	rv := a.review(context.Background(), record(), "Done: go vet clean; ok kvstore/store 1.9s")
	if !rv.Judged || rv.Unfinished || rv.HandedBack || rv.Note != "" || enq.calls != 0 || len(runs.reactivated) != 0 {
		t.Fatalf("%+v", rv)
	}
}

func TestResultAcceptanceSkips(t *testing.T) {
	plan := "I will create store.go first."
	// Off by setting.
	a, _, enq := acceptanceUnderTest(0.99, fakeMeta{}, map[string]string{settingResultAcceptance: "off"})
	if rv := a.review(context.Background(), record(), plan); rv.Judged || enq.calls != 0 {
		t.Fatalf("off: %+v", rv)
	}
	// Unavailable model: nothing happens.
	a, _, enq = acceptanceUnderTest(0.99, fakeMeta{}, nil)
	a.svc = &fakeAcceptSvc{unav: true}
	if rv := a.review(context.Background(), record(), plan); rv.Judged || enq.calls != 0 {
		t.Fatalf("unavailable: %+v", rv)
	}
	// A failed run is announced as the failure it is.
	a, _, enq = acceptanceUnderTest(0.99, fakeMeta{}, nil)
	rec := record()
	rec.Outcome = &subagents.SubagentOutcome{Status: subagents.OutcomeError, Error: "boom"}
	if rv := a.review(context.Background(), rec, "error text"); rv.Judged || enq.calls != 0 {
		t.Fatalf("error outcome: %+v", rv)
	}
	// A flow step belongs to the flow driver.
	a, _, enq = acceptanceUnderTest(0.99, fakeMeta{}, nil)
	rec = record()
	rec.Label = "flow-step"
	if rv := a.review(context.Background(), rec, plan); rv.Judged || enq.calls != 0 {
		t.Fatalf("flow step: %+v", rv)
	}
	// Nil reviewer (decisions not wired).
	var none *resultAcceptance
	if rv := none.review(context.Background(), record(), plan); rv.Judged {
		t.Fatal("nil reviewer judged")
	}
}

// When the child cannot be re-run, the result is announced with the verdict.
func TestResultAcceptanceAnnouncesWhenHandBackImpossible(t *testing.T) {
	a, _, enq := acceptanceUnderTest(0.9, fakeMeta{}, nil)
	a.resolve = func(string) ([]string, string) { return nil, "" }
	rv := a.review(context.Background(), record(), "I will start by reading the files.")
	if rv.HandedBack || enq.calls != 0 || !strings.Contains(rv.Note, "judged UNFINISHED") {
		t.Fatalf("%+v", rv)
	}
}

func TestResultAcceptanceThreshold(t *testing.T) {
	a, _, _ := acceptanceUnderTest(0.5, fakeMeta{}, map[string]string{settingResultAcceptanceThreshold: "0.4"})
	if rv := a.review(context.Background(), record(), "half done"); !rv.Unfinished {
		t.Fatalf("threshold 0.4 not applied: %+v", rv)
	}
	a, _, _ = acceptanceUnderTest(0.5, fakeMeta{}, map[string]string{settingResultAcceptanceThreshold: "nonsense"})
	if rv := a.review(context.Background(), record(), "half done"); rv.Unfinished {
		t.Fatalf("bad threshold must fall back to %.1f: %+v", resultAcceptanceDefaultThreshold, rv)
	}
	if agentFromChildKey("agent:coder:subagent:abc") != "coder" || agentFromChildKey("agent:main:main") != "" {
		t.Fatal("agentFromChildKey")
	}
}
