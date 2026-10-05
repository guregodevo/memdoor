// Package usage is the domain for LLM token-usage accounting: per-call
// pricing and aggregation into a cost report.
//
// It is pure domain logic — no HTTP, no rendering, no cobra — so it is
// unit-testable in isolation and reusable by any adapter. Today the
// `memdoor tokens` CLI is the adapter (it fetches LLM-response events
// from the gateway, maps them to []usage.Call, calls a Reporter, and
// renders the Report). `ask --show-cost` and a future web cost panel
// are the next adapters; the duplicated cost formula they each carry
// should migrate here.
package usage

import (
	"time"
)

// Call is one LLM call's token usage — a value object.
type Call struct {
	Time         time.Time
	Agent        string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
}

// Rates is per-million-token pricing for one model.
type Rates struct {
	InputPerM       float64
	CachedInputPerM float64
	OutputPerM      float64
}

// Prices maps model → Rates and prices calls. A missing model costs $0
// and reports Priced=false so adapters can flag "unpriced, not free".
type Prices map[string]Rates

// GroupBy selects the aggregation key.
type GroupBy string

const (
	ByAgent GroupBy = "agent"
	ByModel GroupBy = "model"
)

// Row is one aggregated group (an agent or a model).
type Row struct {
	Key          string
	Calls        int
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	Dollars      float64
	Models       []string // distinct models seen, sorted
}

// Report is the aggregation result; Rows are sorted by descending cost.
type Report struct {
	Rows           []Row
	TotalCalls     int
	TotalIn        int64
	TotalOut       int64
	TotalDollars   float64
	UnpricedModels []string
}

// PricedCall is a Call with its computed cost, for the per-call view.
type PricedCall struct {
	Call
	Dollars float64
}

// Reporter is the domain seam adapters depend on. Implementations turn a
// stream of calls into a ranked cost report.
type Reporter interface {
	// Aggregate groups calls by agent or model and ranks groups by cost.
	Aggregate(calls []Call, by GroupBy) Report
	// Top prices every call and returns the n most expensive (n<=0 = all).
	Top(calls []Call, n int) []PricedCall
}
