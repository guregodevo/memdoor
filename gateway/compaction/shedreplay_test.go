package compaction

// A replay of real session files measuring ElideSpent run on EVERY request
// instead of only when the conversation is compacted: what it saves (tokens
// no longer sent) and what it costs (a stub changes an earlier message, so
// the prefix cache misses from there). Run with
//
//	go test ./gateway/compaction/ -run TestReplayShedCost -v \
//	  -args -shed-trace <sessions.json>,<sessions.json>,...
//
// Each session is split into the requests the model was sent (a Boundary
// record starts a reshaped conversation) and accounted request by request:
// the longest common message prefix with the previous request is a cache
// hit, the rest a miss. Cached input is priced at 2%, 10% and 20% of
// uncached (DeepSeek V4.1 Flash and MiMo are 2%, GLM 5.3 Flash 20%).
//
// Result, 2026-09-30, every coder session on the dev machine (375 sessions,
// 4,546 requests): +16.4% at 2%, +4.8% at 10%, +0.7% at 20%. A coder's
// repeated calls are small (4% fewer tokens sent) and each stub costs the
// cache (25% more misses), so the shed stays in compaction only.

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/gateway/context"
	"memdoor/pkg/llm"
)

var shedTrace = flag.String("shed-trace", "", "session JSONL file(s), comma separated, to replay")

// loadSnapshots replays a session JSONL the way LoadRecentMessages reads
// it: records append to the conversation, a Boundary-carried block replaces
// it (a /compact reshape). A snapshot is taken at every request point —
// after each tool result, which is when the next model call happens.
func loadSnapshots(t *testing.T, path string) [][]llm.MessageParam {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var recs []messageRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec messageRecord
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	var snaps [][]llm.MessageParam
	var conv []llm.MessageParam
	flush := func() {
		if len(conv) > 0 {
			cp := make([]llm.MessageParam, len(conv))
			copy(cp, conv)
			snaps = append(snaps, cp)
		}
	}
	for _, rec := range recs {
		if rec.Boundary {
			conv = nil // a reshaped conversation starts here
		}
		conv = append(conv, rec.Message)
		if len(rec.Message.Content) > 0 && rec.Message.Content[0].OfToolResult != nil {
			flush() // the next request happens here
		}
	}
	flush()
	return snaps
}

// shedStats is a session's cost/benefit accounting.
type shedStats struct {
	requests     int
	baselineSent int // tokens sent, as recorded
	treatedSent  int // tokens sent with the per-request shed
	baselineMiss int // uncached tokens, as recorded
	treatedMiss  int // uncached tokens, with the shed
}

// longestCommonPrefix counts the leading messages two snapshots share
// byte-for-byte — the provider-side prefix that survives.
func longestCommonPrefix(a, b []llm.MessageParam) int {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		ab, _ := json.Marshal(a[i])
		bb, _ := json.Marshal(b[i])
		if string(ab) != string(bb) {
			return i
		}
	}
	return n
}

// cost is the billable input in full-price tokens when a cached token costs
// ratio of an uncached one: misses at full price, hits (sent minus miss) at
// the ratio.
func (st shedStats) cost(ratio float64, treated bool) float64 {
	if treated {
		return float64(st.treatedMiss) + ratio*float64(st.treatedSent-st.treatedMiss)
	}
	return float64(st.baselineMiss) + ratio*float64(st.baselineSent-st.baselineMiss)
}

func replayShed(t *testing.T, path string) shedStats {
	t.Helper()
	snaps := loadSnapshots(t, path)
	if len(snaps) < 2 {
		return shedStats{}
	}
	tc := context.NewTokenCounter("default", false)
	var st shedStats
	st.requests = len(snaps)
	var prevBase, prevTreat []llm.MessageParam
	for i, s := range snaps {
		treat, _ := ElideSpent(s)
		st.baselineSent += tc.CountConversationTokens(s)
		st.treatedSent += tc.CountConversationTokens(treat)
		if i > 0 {
			st.baselineMiss += tc.CountConversationTokens(s[longestCommonPrefix(s, prevBase):])
			st.treatedMiss += tc.CountConversationTokens(treat[longestCommonPrefix(treat, prevTreat):])
		} else {
			st.baselineMiss += tc.CountConversationTokens(s)
			st.treatedMiss += tc.CountConversationTokens(treat)
		}
		prevBase, prevTreat = s, treat
	}
	return st
}

func TestReplayShedCost(t *testing.T) {
	if *shedTrace == "" {
		t.Skip("no -shed-trace given; replay is for measurement, not CI")
	}
	var total shedStats
	files := 0
	for _, path := range strings.Split(*shedTrace, ",") {
		if strings.TrimSpace(path) == "" {
			continue
		}
		files++
		st := replayShed(t, strings.TrimSpace(path))
		total.requests += st.requests
		total.baselineSent += st.baselineSent
		total.treatedSent += st.treatedSent
		total.baselineMiss += st.baselineMiss
		total.treatedMiss += st.treatedMiss
		if st.requests > 0 {
			fmt.Printf("%s requests=%d sent=%dkt->%dkt miss=%dkt->%dkt cost@2%%=%+.1f%%\n",
				filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path)))), st.requests,
				st.baselineSent/1000, st.treatedSent/1000, st.baselineMiss/1000, st.treatedMiss/1000,
				100*(st.cost(0.02, true)-st.cost(0.02, false))/st.cost(0.02, false))
		}
	}
	if total.requests == 0 {
		t.Fatal("no replays ran")
	}
	for _, ratio := range []float64{0.02, 0.10, 0.20} {
		// Miss tokens pay full price; hit tokens (sent minus miss) get the
		// cached-input discount.
		baseCost, treatCost := total.cost(ratio, false), total.cost(ratio, true)
		fmt.Printf("cached@%.0f%%: baseline=%.1fkt treated=%.1fkt delta=%+.1fkt (%+.1f%%)\n",
			ratio*100, baseCost/1000, treatCost/1000, (treatCost-baseCost)/1000,
			100*(treatCost-baseCost)/baseCost)
	}
	fmt.Printf("files=%d requests=%d baseline_sent=%dkt treated_sent=%dkt miss_base=%dkt miss_treated=%dkt\n",
		files, total.requests, total.baselineSent/1000, total.treatedSent/1000,
		total.baselineMiss/1000, total.treatedMiss/1000)
}

// messageRecord mirrors gateway.MessageRecord (session_persistence.go) — the
// compaction package cannot import the gateway package without a cycle.
type messageRecord struct {
	Message   llm.MessageParam `json:"message"`
	Timestamp int64            `json:"timestamp"`
	Boundary  bool             `json:"boundary,omitempty"`
	Carried   bool             `json:"carried,omitempty"`
}
