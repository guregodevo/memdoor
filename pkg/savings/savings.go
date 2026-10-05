// Package savings is the receipt for the one claim Memdoor makes: that the
// decision model makes the same work cost less (docs/features/DECIDE.md).
//
// Greg, 2026-09-27, reading OpenRouter's app ranking: "let's focus on cheaper
// by efficiency". A claim measured once in a lab is marketing; a claim measured
// on the person's own turns, every turn, is a receipt they can check against
// their provider's invoice. So every place that drops bytes before they reach a
// model appends one line here, and `memdoor savings` adds the month up.
//
// What is counted is only ever what was measured at the moment it happened:
//
//   - a judged read: the bytes the tool WOULD have returned unjudged against
//     the bytes it did return
//   - the per-turn toolbox: the tool-schema bytes not submitted, times the
//     calls of that turn (schemas are resent on every call)
//   - a turn the decision model ended for want of progress: counted, never
//     converted into tokens, because nobody knows what the next call would
//     have cost
//
// The ledger is append-only JSON Lines at ~/.memdoor/savings.jsonl, one line per
// event, never rewritten — the same shape as the meter it sits beside.
package savings

import (
	"encoding/json"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Kind is what produced a saving.
const (
	KindJudgedRead = "judged_read" // a tool returned only what the task needed
	KindToolbox    = "toolbox"     // schemas the turn did not have to carry
	KindEarlyStop  = "early_stop"  // a turn ended for want of progress
)

// Entry is one measured saving.
type Entry struct {
	At   time.Time `json:"ts"`
	Kind string    `json:"kind"`
	// Tool names the tool for a judged read ("jgrep", "read_file", …).
	Tool string `json:"tool,omitempty"`
	// RawBytes is what would have been sent, KeptBytes what was sent. For the
	// toolbox, Raw is the full palette's schema bytes and Kept the submitted
	// ones, both multiplied by Calls.
	RawBytes  int `json:"raw_bytes,omitempty"`
	KeptBytes int `json:"kept_bytes,omitempty"`
	// Calls is how many model calls carried the narrowed toolbox.
	Calls int `json:"calls,omitempty"`
	// Model is the model that would have read it, when the caller knows it.
	Model string `json:"model,omitempty"`
}

// Saved is the bytes this entry kept out of a request.
func (e Entry) Saved() int {
	if e.RawBytes <= e.KeptBytes {
		return 0
	}
	return e.RawBytes - e.KeptBytes
}

var (
	mu   sync.Mutex
	path string // overridden in tests
)

// SetLedger points the ledger somewhere other than ~/.memdoor/savings.jsonl.
func SetLedger(p string) {
	mu.Lock()
	defer mu.Unlock()
	path = p
}

func ledger() string {
	if path != "" {
		return path
	}
	return shared.MemdoorHome("savings.jsonl")
}

// Record appends one saving. It never fails a turn: a ledger that cannot be
// written is a lost receipt, not a broken tool.
func Record(e Entry) {
	if e.Kind == "" || (e.Kind != KindEarlyStop && e.Saved() <= 0) {
		return
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	mu.Lock()
	defer mu.Unlock()
	p := ledger()
	if p == "" {
		return
	}
	if path == "" {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	if b, err := json.Marshal(e); err == nil {
		_, _ = f.Write(append(b, '\n'))
	}
}

// Summary is a month of the ledger.
type Summary struct {
	Month         string // "2026-09"
	JudgedCalls   int    // judged reads that dropped something
	JudgedRaw     int64  // bytes those reads would have returned
	JudgedKept    int64  // bytes they did return
	ToolboxSaved  int64  // schema bytes the calls that ran did not carry
	ToolboxCalls  int    // calls that ran on a narrowed toolbox
	EarlyStops    int    // turns the decision model ended
	ByTool        map[string]int64
	ModelsSeen    map[string]int
	BytesPerToken float64 // the divisor used below, stated rather than hidden
}

// bytesPerToken is the conversion this package admits to using. Code and logs
// run about 3.6 bytes to a token on the tokenizers these models ship; 4 is the
// round number people recognise and it UNDERSTATES the saving slightly, which
// is the direction a receipt should err.
const bytesPerToken = 4

// TokensSaved is the whole month's saving in tokens, judged reads plus toolbox.
func (s Summary) TokensSaved() int64 {
	return (s.JudgedRaw - s.JudgedKept + s.ToolboxSaved) / bytesPerToken
}

// DollarsSaved prices the month at a given dollars-per-million-input-tokens.
// The caller decides which price is honest to use; `memdoor savings` uses the
// CHEAPEST model the month actually ran on, so the figure is a floor.
func (s Summary) DollarsSaved(perMillionIn float64) float64 {
	return float64(s.TokensSaved()) / 1e6 * perMillionIn
}

// ReadMonth sums the ledger for the month containing when.
func ReadMonth(when time.Time) (Summary, error) {
	mu.Lock()
	p := ledger()
	mu.Unlock()
	s := Summary{
		Month: when.Format("2006-01"), ByTool: map[string]int64{},
		ModelsSeen: map[string]int{}, BytesPerToken: bytesPerToken,
	}
	if p == "" {
		return s, nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e Entry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.At.Format("2006-01") != s.Month {
			continue
		}
		if e.Model != "" {
			s.ModelsSeen[e.Model]++
		}
		switch e.Kind {
		case KindJudgedRead:
			s.JudgedCalls++
			s.JudgedRaw += int64(e.RawBytes)
			s.JudgedKept += int64(e.KeptBytes)
			if e.Tool != "" {
				s.ByTool[e.Tool] += int64(e.Saved())
			}
		case KindToolbox:
			s.ToolboxCalls++
			s.ToolboxSaved += int64(e.Saved())
		case KindEarlyStop:
			s.EarlyStops++
		}
	}
	return s, nil
}
