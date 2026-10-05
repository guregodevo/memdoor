package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
)

// Shadow routing (docs/roadmap/SHOULD.md, "Per-session model routing",
// phase 3). Every turn is scored for
// "steps without progress" with the v1 rule, and what an escalation rule
// WOULD have done is logged: nothing about the turn changes. The log is
// what calibrates the rule before phase 4 escalates for real — the
// research is clear that intervening on a poorly calibrated signal costs
// more solved tasks than it gains.
//
// One JSON line per turn in ~/.memdoor/route_shadow.jsonl: the session,
// agent, tier and model, the score and its components, whether and when
// the rule would have fired, and a compact record of every tool call
// (kind, hashes, error) so a different rule can be replayed offline.

const (
	shadowWindow      = 8 // calls the score looks back over
	shadowMinCalls    = 3 // no score before this many calls
	shadowFireTwice   = 6 // fire at this score on two consecutive checks…
	shadowFireOnce    = 9 // …or at this one once
	shadowReadStreak  = 6 // reads in a row after the first edit
	shadowHardRepeat  = 5 // identical calls that fire on their own
	shadowHardFailRun = 5 // failing test runs that fire on their own
	// shadowExploreEvery: reads made before any edit at which the decision
	// model is asked whether the run should stop exploring (turn_verdict.go
	// exploring). Asked again at every multiple — a "no" at 30 reads is not
	// a "no" at 90. The score above cannot see this run: reading only counts
	// after the first edit, and distinct reads never repeat, so live
	// 2026-09-28 a "continue the work" turn made 129 calls, 0 edits, score 0.
	shadowExploreEvery = 30
)

var (
	goFailRe     = regexp.MustCompile(`(?m)^\s*--- FAIL: (\S+)`)
	pytestFailRe = regexp.MustCompile(`(?m)^FAILED (\S+)`)
	goOKRe       = regexp.MustCompile(`(?m)^(ok\s|PASS\b)`)
)

// shadowCall is one tool call as the scorer sees it.
type shadowCall struct {
	I         int      `json:"i"`
	Tool      string   `json:"tool"`
	Kind      string   `json:"kind"` // read | edit | test | build | other
	Args      string   `json:"args"` // hash prefix of the input
	Result    string   `json:"result"`
	Err       string   `json:"err,omitempty"` // hash prefix of the error
	IsErr     bool     `json:"is_error"`
	Failing   []string `json:"failing,omitempty"` // failing tests, a test run
	Green     bool     `json:"green,omitempty"`   // a test run that passed
	BuildFail bool     `json:"build_fail,omitempty"`
	Ms        int64    `json:"ms"`
	// Head is the call in words — tool, the start of its input, the start of
	// its error — for the decision model to read when the score fires
	// (turn_verdict.go stuck). Not logged: the hashes are.
	Head string `json:"-"`
}

// routeShadow scores one turn.
type routeShadow struct {
	mu          sync.Mutex
	session     string
	agent       string
	tier        int
	start       time.Time
	calls       []shadowCall
	edited      bool // an edit has happened this turn
	truncations int
	announced   bool // the turn-end verdict found an announcement
	resets      int  // times progress reset the score
	progressAt  int  // calls before this index are forgiven
	lastScore   int
	maxScore    int
	fired       bool
	firedSeen   bool // justFired handed the firing out once
	firedAt     int
	firedScore  int
	firedParts  map[string]int
	exploreAsks int // checks handed out since exploreFrom
	exploreFrom int // index after the last successful edit: reads count from here
}

// newRouteShadow starts scoring a turn; nil when shadow logging is off.
func newRouteShadow(ctx context.Context, agent string) *routeShadow {
	sid, _ := ctx.Value(sharedctx.SessionIDKey).(string)
	tier, _ := ctx.Value(sharedctx.TierKey).(int)
	return &routeShadow{session: sid, agent: agent, tier: tier, start: time.Now()}
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:6])
}

// callKind classifies a tool call by what it does to the working tree.
func callKind(tool, input string) string {
	switch tool {
	case "read_file", "jread", "grep", "jgrep", "glob", "locate", "jlogs", "notes", "web_fetch", "todo_read", "agent_log":
		return "read"
	case "apply_patch", "write_file", "edit_file":
		return "edit"
	case "bash":
		var in struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal([]byte(input), &in)
		c := strings.TrimSpace(in.Command)
		switch {
		case strings.Contains(c, "go test") || strings.Contains(c, "pytest") || strings.Contains(c, "npm test") ||
			strings.Contains(c, "npx jest") || strings.Contains(c, "npx vitest") || strings.Contains(c, "cargo test") || strings.Contains(c, "make test"):
			return "test"
		case strings.Contains(c, "go build") || strings.Contains(c, "go vet") || strings.Contains(c, "make build") ||
			strings.Contains(c, "npm run build") || strings.Contains(c, "tsc") || strings.Contains(c, "cargo build") || strings.Contains(c, "gofmt -l"):
			return "build"
		case strings.Contains(c, "sed -i") || strings.Contains(c, "gofmt -w") || strings.Contains(c, " > ") || strings.Contains(c, ">>") ||
			strings.Contains(c, "tee ") || strings.HasPrefix(c, "mv ") || strings.HasPrefix(c, "rm ") || strings.HasPrefix(c, "cp ") ||
			strings.Contains(c, "git apply") || strings.Contains(c, "git checkout") || strings.HasPrefix(c, "cat >"):
			return "edit"
		case strings.HasPrefix(c, "cat ") || strings.HasPrefix(c, "ls") || strings.HasPrefix(c, "head ") || strings.HasPrefix(c, "tail ") ||
			strings.HasPrefix(c, "sed -n") || strings.HasPrefix(c, "grep ") || strings.HasPrefix(c, "rg ") || strings.HasPrefix(c, "find ") ||
			strings.HasPrefix(c, "git log") || strings.HasPrefix(c, "git status") || strings.HasPrefix(c, "git diff") || strings.HasPrefix(c, "wc "):
			return "read"
		}
	}
	return "other"
}

// failingTests reads the failing set out of a test run's output.
func failingTests(out string) []string {
	set := map[string]bool{}
	for _, m := range goFailRe.FindAllStringSubmatch(out, -1) {
		set[m[1]] = true
	}
	for _, m := range pytestFailRe.FindAllStringSubmatch(out, -1) {
		set[m[1]] = true
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// observe records a tool call and rescores the turn.
func (s *routeShadow) observe(tool, input string, info ToolExecutionInfo) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := shadowCall{I: len(s.calls), Tool: tool, Kind: callKind(tool, input), Args: shortHash(tool + "\x00" + input),
		Result: shortHash(info.Output), IsErr: info.Error != "", Ms: info.DurationMs}
	if c.IsErr {
		c.Err = shortHash(info.Error)
	}
	c.Head = tool + " " + clipHead(strings.Join(strings.Fields(input), " "), 160)
	if c.IsErr {
		c.Head += " → error: " + clipHead(strings.Join(strings.Fields(info.Error), " "), 160)
	}
	switch c.Kind {
	case "edit":
		if !c.IsErr {
			s.edited = true
		}
	case "test":
		c.Failing = failingTests(info.Output + "\n" + info.Error)
		c.Green = !c.IsErr && len(c.Failing) == 0 && goOKRe.MatchString(info.Output)
		if c.IsErr && len(c.Failing) == 0 {
			c.Failing = []string{"?"} // failed, names unknown: counts as one
		}
	case "build":
		c.BuildFail = c.IsErr
	}
	s.calls = append(s.calls, c)
	// Progress — a green run or a failing set that shrank — forgives what
	// came before it.
	if c.Kind == "test" {
		if prev := s.lastTestRun(len(s.calls) - 1); c.Green || (prev != nil && len(prev.Failing) > 0 && len(c.Failing) < len(prev.Failing)) {
			s.progressAt = len(s.calls)
			s.resets++
			s.lastScore = 0
		}
	}
	s.check()
}

// lastTestRun is the test run before index i, or nil.
func (s *routeShadow) lastTestRun(i int) *shadowCall {
	for j := i - 1; j >= 0; j-- {
		if s.calls[j].Kind == "test" {
			return &s.calls[j]
		}
	}
	return nil
}

// observeTruncation: a reply cut at max_tokens.
func (s *routeShadow) observeTruncation() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.truncations++
	s.mu.Unlock()
}

// observeAnnounced: the turn-end verdict found a reply that only announced
// work (turn_verdict.go). It is the last signal of a turn, so it rescores.
func (s *routeShadow) observeAnnounced() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.announced = true
	s.check()
	s.mu.Unlock()
}

// score applies the v1 rule to the window. Returns the score, its parts
// and whether a hard trigger fired.
func (s *routeShadow) score() (int, map[string]int, bool) {
	parts := map[string]int{}
	hard := false
	from := s.progressAt
	if len(s.calls)-from > shadowWindow {
		from = len(s.calls) - shadowWindow
	}
	win := s.calls[from:]
	if len(s.calls) < shadowMinCalls {
		return 0, parts, false
	}
	// Identical calls.
	same := map[string]int{}
	for _, c := range win {
		same[c.Args]++
	}
	for _, n := range same {
		if n >= shadowHardRepeat {
			hard = true
		}
		if n >= 3 {
			parts["repeat"] = 3
		}
	}
	// Alternating pair: a b a b a b.
	if len(win) >= 6 {
		t := win[len(win)-6:]
		if t[0].Args != t[1].Args && t[0].Args == t[2].Args && t[2].Args == t[4].Args && t[1].Args == t[3].Args && t[3].Args == t[5].Args {
			parts["alternating"] = 2
		}
	}
	// Errors.
	errs := 0
	for _, c := range win {
		if c.IsErr {
			errs++
		}
	}
	if errs >= 3 {
		parts["errors"] = 2
		if n := len(win); n >= 2 && win[n-1].IsErr && win[n-2].IsErr && win[n-1].Err == win[n-2].Err {
			parts["errors"] = 3
		}
	}
	// Test or build fail streak with the failing set not shrinking.
	streak, lastFail := 0, -1
	for _, c := range win {
		switch {
		case c.Kind == "test" && len(c.Failing) > 0:
			if lastFail >= 0 && len(c.Failing) < lastFail {
				streak = 0
			}
			streak++
			lastFail = len(c.Failing)
		case c.BuildFail:
			streak++
		case c.Kind == "test" && c.Green:
			streak, lastFail = 0, -1
		}
	}
	if streak >= shadowHardFailRun {
		hard = true
	}
	if streak >= 3 {
		parts["fail_streak"] = 3
	}
	// Reads with no edit after the first edit.
	if s.edited {
		reads := 0
		for i := len(s.calls) - 1; i >= 0 && s.calls[i].Kind == "read"; i-- {
			reads++
		}
		if reads >= shadowReadStreak {
			parts["reads"] = 1
		}
	}
	if s.announced {
		parts["announced"] = 2
	}
	switch {
	case s.truncations >= 2:
		parts["truncated"] = 2
	case s.truncations == 1:
		parts["truncated"] = 1
	}
	total := 0
	for _, v := range parts {
		total += v
	}
	return total, parts, hard
}

// check rescores and records the first moment the rule would fire.
func (s *routeShadow) check() {
	total, parts, hard := s.score()
	if total > s.maxScore {
		s.maxScore = total
	}
	fire := hard || total >= shadowFireOnce || (total >= shadowFireTwice && s.lastScore >= shadowFireTwice)
	s.lastScore = total
	if fire && !s.fired {
		s.fired, s.firedAt, s.firedScore, s.firedParts = true, len(s.calls), total, parts
		logs.New("Agent").Warn("shadow routing: an escalation rule would fire (nothing changed)",
			slog.String("session", s.session), slog.String("agent", s.agent), slog.Int("tier", s.tier),
			slog.Int("score", total), slog.Bool("hard", hard), slog.Int("at_call", len(s.calls)), slog.Any("parts", parts))
	}
}

// exploreDue reports, once per shadowExploreEvery reads made since the last
// successful edit (or the turn's start), a turn that is reading without
// changing anything; n is that read count and edited whether the turn changed
// files before it. Counting from the last edit, not only in a turn with no
// edit at all: live 2026-09-28 a turn edited tunnel.go early, then made ~60
// reads of one goroutine dump, and no check ever fired.
func (s *routeShadow) exploreDue() (n int, edited, due bool) {
	if s == nil {
		return 0, false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	from := 0
	for i, c := range s.calls {
		if c.Kind == "edit" && !c.IsErr {
			from = i + 1
		}
	}
	if from != s.exploreFrom {
		s.exploreFrom, s.exploreAsks = from, 0
	}
	for _, c := range s.calls[from:] {
		if c.Kind == "read" {
			n++
		}
	}
	if n >= (s.exploreAsks+1)*shadowExploreEvery {
		s.exploreAsks++
		return n, from > 0, true
	}
	return n, from > 0, false
}

// justFired is true once, the first time the rule fires in this turn: the
// moment to ask the decision model whether the run is stuck.
func (s *routeShadow) justFired() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fired && !s.firedSeen {
		s.firedSeen = true
		return true
	}
	return false
}

// windowText is the last shadowWindow calls in words, oldest first.
func (s *routeShadow) windowText() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	from := len(s.calls) - shadowWindow
	if from < 0 {
		from = 0
	}
	var b strings.Builder
	for i, c := range s.calls[from:] {
		fmt.Fprintf(&b, "%d. %s\n", from+i+1, c.Head)
	}
	return b.String()
}

// shadowRecord is the line written per turn.
type shadowRecord struct {
	At          string         `json:"at"`
	Session     string         `json:"session"`
	Agent       string         `json:"agent"`
	Tier        int            `json:"tier"`
	Model       string         `json:"model,omitempty"`
	Seconds     float64        `json:"seconds"`
	Calls       int            `json:"calls"`
	Edited      bool           `json:"edited"`
	Truncations int            `json:"truncations"`
	Announced   bool           `json:"announced"`
	Resets      int            `json:"resets"`
	MaxScore    int            `json:"max_score"`
	Fired       bool           `json:"fired"`
	FiredAt     int            `json:"fired_at,omitempty"`
	FiredScore  int            `json:"fired_score,omitempty"`
	FiredParts  map[string]int `json:"fired_parts,omitempty"`
	Outcome     string         `json:"outcome"` // ended | error
	Error       string         `json:"error,omitempty"`
	ToolCalls   []shadowCall   `json:"tool_calls"`
}

// finish writes the turn's line. Turns with no tool call are not logged:
// there is nothing to score.
func (s *routeShadow) finish(resp *AgentResponse) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return
	}
	rec := shadowRecord{At: time.Now().Format(time.RFC3339), Session: s.session, Agent: s.agent, Tier: s.tier,
		Seconds: time.Since(s.start).Seconds(), Calls: len(s.calls), Edited: s.edited, Truncations: s.truncations,
		Announced: s.announced, Resets: s.resets, MaxScore: s.maxScore, Fired: s.fired, FiredAt: s.firedAt,
		FiredScore: s.firedScore, FiredParts: s.firedParts, Outcome: "ended", ToolCalls: s.calls}
	if resp != nil {
		rec.Model = resp.Model
		if resp.Error != "" {
			rec.Outcome, rec.Error = "error", truncateForLog(resp.Error, 200)
		}
	}
	if err := appendShadowRecord(rec); err != nil {
		logs.New("Agent").Debug("shadow routing log not written", slog.String("error", err.Error()))
	}
}

// shadowLogPath is ~/.memdoor/route_shadow.jsonl; a var so a test can point
// it elsewhere.
var shadowLogPath = func() string {
	return shared.MemdoorHome("route_shadow.jsonl")
}

var shadowLogMu sync.Mutex

func appendShadowRecord(rec shadowRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	shadowLogMu.Lock()
	defer shadowLogMu.Unlock()
	p := shadowLogPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}
