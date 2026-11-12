package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"memdoor/pkg/decision"
	"memdoor/pkg/decision/systemone"
)

// servingSays is a Service that also says how many requests it serves at once,
// the way the gateway's service does for a provider.
type servingSays struct {
	serviceFunc
	n int
}

func (s servingSays) Parallel(string) int { return s.n }

// inFlight counts how many decision requests a fake service had at once.
type inFlight struct {
	mu   sync.Mutex
	now  int
	most int
}

func (f *inFlight) enter() {
	f.mu.Lock()
	f.now++
	if f.now > f.most {
		f.most = f.now
	}
	f.mu.Unlock()
}

func (f *inFlight) leave() {
	f.mu.Lock()
	f.now--
	f.mu.Unlock()
}

func (f *inFlight) peak() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.most
}

// answering builds a service that holds every request open for hold and
// answers all of its questions, while count watches the concurrency.
func answering(count *inFlight, hold time.Duration) serviceFunc {
	return func(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
		count.enter()
		defer count.leave()
		time.Sleep(hold)
		r := decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{}}
		for id := range req.Questions {
			r.Answers[id] = decision.Answer{Kind: decision.KindBoolean, ProbabilityTrue: 0.9}
		}
		return r
	}
}

// candidates is a jgrep-sized read: 192 hunks, 8 decisions requests.
func candidates() []string {
	items := make([]string, 192)
	for i := range items {
		items[i] = fmt.Sprintf("func filler%d() {}", i)
	}
	return items
}

// A local runtime answers one request at a time. Sending eight at once only
// queues them on its compute lock, and the later ones come back as a deadline
// and leave the read unjudged (issue #288, 2026-11-11).
func TestJudgeRelevanceSendsOneAtATimeToALocalRuntime(t *testing.T) {
	var count inFlight
	svc := servingSays{serviceFunc: answering(&count, 2*time.Millisecond), n: 1}

	ps, err := judgeRelevance(context.Background(), svc, relevanceSpec{Task: "t", Question: "Relevant?", Purpose: "test"}, candidates())
	if err != nil {
		t.Fatalf("every batch must be judged: %v", err)
	}
	if peak := count.peak(); peak != 1 {
		t.Fatalf("a runtime that serves one at a time gets one at a time: peak %d", peak)
	}
	for i, p := range ps {
		if p != 0.9 {
			t.Fatalf("item %d judged %v, want every batch answered", i, p)
		}
	}
}

// Hosted Jev serves many at once, so the judge keeps its eight in flight.
func TestJudgeRelevanceSendsEightAtOnceToHostedJev(t *testing.T) {
	var count inFlight
	svc := answering(&count, 20*time.Millisecond)

	ps, err := judgeRelevance(context.Background(), svc, relevanceSpec{Task: "t", Question: "Relevant?", Purpose: "test"}, candidates())
	if err != nil {
		t.Fatalf("every batch must be judged: %v", err)
	}
	if peak := count.peak(); peak != jevParallel {
		t.Fatalf("hosted Jev keeps %d in flight, peak %d", jevParallel, peak)
	}
	for i, p := range ps {
		if p != 0.9 {
			t.Fatalf("item %d judged %v, want every batch answered", i, p)
		}
	}
}

// The whole path, as it runs: jgrep's 192 candidates judged through the real
// System One client pointed at a localhost runtime, one request at a time.
func TestJudgeRelevanceOverALocalDecisionEndpoint(t *testing.T) {
	var count inFlight
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.enter()
		defer count.leave()
		var req struct {
			Questions map[string]any `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		time.Sleep(2 * time.Millisecond)
		answers := map[string]any{}
		for id := range req.Questions {
			answers[id] = map[string]any{"type": "noul", "noul": 0.9}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "local", "answers": answers})
	}))
	defer srv.Close()
	client, err := systemone.New(systemone.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("systemone.New: %v", err)
	}
	if client.Parallel() != 1 {
		t.Fatalf("a localhost endpoint serves one at a time, not %d", client.Parallel())
	}
	svc, err := decision.NewService(func(string) string { return "systemone" }, client)
	if err != nil {
		t.Fatalf("decision.NewService: %v", err)
	}
	ps, err := judgeRelevance(context.Background(), svc, relevanceSpec{Task: "t", Question: "Relevant?", Purpose: "test"}, candidates())
	if err != nil {
		t.Fatalf("every batch must be judged: %v", err)
	}
	if peak := count.peak(); peak != 1 {
		t.Fatalf("a local runtime gets one request at a time: peak %d", peak)
	}
	for i, p := range ps {
		if p != 0.9 {
			t.Fatalf("item %d judged %v, want every batch answered", i, p)
		}
	}
}

// Two judged reads at once (two windows, a spawned run, team mode) still send
// a localhost runtime one request at a time: the limit is the runtime's,
// shared by every caller of the service, not one per read (issue #288).
func TestTwoJudgedReadsAtOnceShareALocalRuntime(t *testing.T) {
	var count inFlight
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.enter()
		defer count.leave()
		var req struct {
			Questions map[string]any `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		time.Sleep(2 * time.Millisecond)
		answers := map[string]any{}
		for id := range req.Questions {
			answers[id] = map[string]any{"type": "noul", "noul": 0.9}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "local", "answers": answers})
	}))
	defer srv.Close()
	client, err := systemone.New(systemone.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("systemone.New: %v", err)
	}
	svc, err := decision.NewService(func(string) string { return "systemone" }, client)
	if err != nil {
		t.Fatalf("decision.NewService: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = judgeRelevance(context.Background(), svc, relevanceSpec{Task: "t", Question: "Relevant?", Purpose: "test"}, candidates())
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("read %d: every batch must be judged: %v", i, err)
		}
	}
	if peak := count.peak(); peak != 1 {
		t.Fatalf("two reads at once sent the runtime %d requests together", peak)
	}
}
