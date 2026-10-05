package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"memdoor/pkg/decision"
	"memdoor/pkg/decision/systemone"
)

// scriptedDecisions is called from parallel batches (jev_core.go), so it
// locks: unguarded, two batches appending at once panicked the suite.
type scriptedDecisions struct {
	mu    sync.Mutex
	p     func(question string) float64
	fail  bool
	calls int
}

func (s *scriptedDecisions) Evaluate(_ context.Context, req decision.Request, _ decision.Options) decision.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.fail {
		return decision.Unavailable(decision.ReasonNotConfigured, "none")
	}
	r := decision.Result{Status: decision.StatusOK, Answers: map[string]decision.Answer{}}
	for id, q := range req.Questions {
		r.Answers[id] = decision.Answer{Kind: decision.KindBoolean, ProbabilityTrue: s.p(q.Instructions)}
	}
	return r
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestJgrepRanksFiltersAndMerges(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"a/retry.go": "package a\n\nfunc resumeTurn() {\n\t// retry here\n\tretry()\n}\n",
		"b/log.go":   "package b\n// retry is mentioned in a comment only\n",
		"c/x.txt":    "retry\n",
		".git/HEAD":  "retry\n",
	})
	svc := &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "retry.go") {
			return 0.93
		}
		return 0.1
	}}
	res, err := RunJgrep(context.Background(), svc, JgrepInput{Path: dir, Task: "find where a turn is resumed", Pattern: "retry", Glob: "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Ranked || res.Candidates != 2 || res.Files != 2 {
		t.Fatalf("candidates=%d files=%d ranked=%v (.git and *.txt must be skipped; two adjacent matches merge)", res.Candidates, res.Files, res.Ranked)
	}
	if len(res.Kept) != 1 || !strings.HasSuffix(res.Kept[0].File, filepath.Join("a", "retry.go")) || res.Kept[0].P != 0.93 {
		t.Fatalf("kept: %+v", res.Kept)
	}
	if out := res.Render(); !strings.Contains(out, "kept 1 of 2") || !strings.Contains(out, "p=0.93") {
		t.Fatalf("render: %s", out)
	}
}

func TestJgrepFallsBackToPlainGrep(t *testing.T) {
	dir := writeTree(t, map[string]string{"a.go": "x := retry()\n"})
	res, err := RunJgrep(context.Background(), &scriptedDecisions{fail: true}, JgrepInput{Path: dir, Task: "t", Pattern: "retry"})
	if err != nil || res.Ranked || len(res.Kept) != 1 || !strings.Contains(res.Render(), "UNRANKED") {
		t.Fatalf("fallback: %+v %v", res, err)
	}
	if _, err := RunJgrep(context.Background(), &scriptedDecisions{}, JgrepInput{Path: dir, Pattern: "retry"}); err == nil {
		t.Fatal("missing task accepted")
	}
}

func TestJgrepBatches(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString("hit\n\n\n\n\n\n\n\n") // far apart: one hunk each
	}
	dir := writeTree(t, map[string]string{"many.go": b.String()})
	svc := &scriptedDecisions{p: func(string) float64 { return 0.9 }}
	res, _ := RunJgrep(context.Background(), svc, JgrepInput{Path: dir, Task: "t", Pattern: "hit", Keep: 5})
	if res.Candidates != 60 || svc.calls != 3 || len(res.Kept) != 5 || res.Truncated {
		t.Fatalf("candidates=%d calls=%d kept=%d", res.Candidates, svc.calls, len(res.Kept))
	}
}

// TestJgrepLiveOnRepo runs jgrep on this repository through the real decision
// model and checks that the known answer survives the filter.
//
//	OPEN_ROUTER_API_KEY=... JGREP_E2E=1 go test ./tools/ -run TestJgrepLiveOnRepo -v
func TestJgrepLiveOnRepo(t *testing.T) {
	if os.Getenv("JGREP_E2E") != "1" {
		t.Skip("set JGREP_E2E=1 and OPEN_ROUTER_API_KEY")
	}
	c, err := systemone.New(systemone.Config{BaseURL: "https://openrouter.ai/api/alpha/decisions", APIKey: os.Getenv("OPEN_ROUTER_API_KEY"), Model: "~typesafe/jev-latest"})
	if err != nil {
		t.Fatal(err)
	}
	svc, _ := decision.NewService(func(string) string { return "systemone" }, c)
	repo, _ := filepath.Abs("..")
	cases := []struct {
		task, pattern, glob, want string
	}{
		{"Find where the gateway decides whether an agent should reply to a channel message nobody @mentioned.", "(?i)gate", "*.go", "gateway/decisions.go"},
		{"Find where a finished subagent's result is turned into the message that re-runs its requester.", "(?i)announce", "*.go", "gateway/server_jobs.go|gateway/subagents/announce.go"},
		{"Find where the agent turn loop gives up after the same tool keeps failing.", "(?i)fail", "*.go", "gateway/agent_runtime_process.go"},
	}
	for _, c := range cases {
		start := time.Now()
		res, err := RunJgrep(context.Background(), svc, JgrepInput{Task: c.task, Pattern: c.pattern, Glob: c.glob, Path: filepath.Join(repo, "gateway"), Keep: 8})
		if err != nil {
			t.Fatal(err)
		}
		rank := -1
		for i, h := range res.Kept {
			if rel, _ := filepath.Rel(repo, h.File); strings.Contains("|"+c.want+"|", "|"+rel+"|") {
				rank = i + 1
				break
			}
		}
		all, _, _ := collectHunks(GrepInput{Path: filepath.Join(repo, "gateway"), Glob: c.glob}, regexp.MustCompile(c.pattern))
		grepBytes := 0
		for _, h := range all {
			grepBytes += len(h.File) + len(h.Text) + 16
		}
		t.Logf("  plain grep output ~%d KB (~%dk tokens) vs jgrep %.1f KB", grepBytes/1024, grepBytes/4000, float64(len(res.Render()))/1024)
		var top []string
		for i, h := range res.Kept {
			if i == 3 {
				break
			}
			top = append(top, h.File+":"+fmt.Sprint(h.StartLine)+" p="+ftoa(h.P))
		}
		t.Logf("pattern=%q matched=%d judged=%d files=%d truncated=%v kept=%d ranked=%v in %s; want %s at rank %d; top: %s",
			c.pattern, res.Matched, res.Candidates, res.Files, res.Truncated, len(res.Kept), res.Ranked, time.Since(start).Round(time.Millisecond), c.want, rank, strings.Join(top, " | "))
		if !res.Ranked {
			t.Errorf("unranked: %s", res.Note)
		}
		if rank < 1 || rank > 3 {
			t.Errorf("%s not in the top 3 kept hunks", c.want)
		}
	}
}

func ftoa(f float64) string { return fmt.Sprintf("%.2f", f) }

func TestJgrepPrerankKeepsTaskWordsWhenCapped(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 300; i++ {
		files[fmt.Sprintf("noise/f%03d.go", i)] = "// gate\n"
	}
	files["zz/decisions.go"] = "// the message gate decides whether an agent should reply\n"
	dir := writeTree(t, files)
	var judged []string
	svc := &scriptedDecisions{p: func(q string) float64 {
		judged = append(judged, q)
		if strings.Contains(q, "decisions.go") {
			return 0.9
		}
		return 0.05
	}}
	res, err := RunJgrep(context.Background(), svc, JgrepInput{Path: dir, Task: "find where the message gate decides whether an agent should reply", Pattern: "gate"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 301 || res.Candidates != jgrepMaxCandidates || !res.Truncated {
		t.Fatalf("matched=%d judged=%d truncated=%v", res.Matched, res.Candidates, res.Truncated)
	}
	if len(res.Kept) != 1 || !strings.HasSuffix(res.Kept[0].File, "decisions.go") {
		t.Fatalf("the task-worded hunk must survive the cap and win: %+v", res.Kept)
	}
}

// A minified line is shown by its head, so one bundle cannot flood the judge
// or the window (live 2026-09-27: web/dist made the judge refuse the batch).
func TestJgrepClipsMinifiedLines(t *testing.T) {
	long := strings.Repeat("x", 50_000) + "retry"
	h := hunksFor("dist/app.js", []string{long}, []int{0})
	if len(h) != 1 || len(h[0].Text) > jgrepMaxLine+100 || !strings.Contains(h[0].Text, "50005 characters") {
		t.Fatalf("a 50 KB line must be clipped with its length, got %d bytes", len(h[0].Text))
	}
	if got := clipLine("short"); got != "short" {
		t.Fatalf("a normal line is untouched, got %q", got)
	}
}
