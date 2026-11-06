package providers

import (
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/metering"
)

// Meter — thin adapter over pkg/metering's ledger (the billing-terms
// domain). Kept so existing callers keep their names; the canonical types
// live in pkg/metering.
type MeterEntry = metering.Entry

// RecordMeter appends one turn to the ledger. Metering must never break a
// turn that already succeeded, so failures are swallowed after a log line.
func RecordMeter(e MeterEntry) {
	if e.TS.IsZero() {
		e.TS = time.Now()
	}
	l, err := metering.NewJSONLLedger("")
	if err != nil {
		return
	}
	if err := l.Append(e); err != nil {
		logs.New("Meter").Warn("meter append failed: " + err.Error())
	}
	meterObservers.Range(func(_, fn any) bool {
		fn.(func(MeterEntry))(e)
		return true
	})
}

// meterObservers are told of every ledger row as it is written: a workflow
// run sums its steps' tokens from them while it runs (the run panel shows
// each step's tokens live, #94), with no read of the whole ledger.
var (
	meterObservers  sync.Map // int → func(MeterEntry)
	meterObserverMu sync.Mutex
	meterObserverN  int
)

// OnMeter calls fn with each ledger row written from now on, until the
// returned func is called. fn runs on the caller's goroutine: keep it short.
func OnMeter(fn func(MeterEntry)) func() {
	meterObserverMu.Lock()
	meterObserverN++
	key := meterObserverN
	meterObserverMu.Unlock()
	meterObservers.Store(key, fn)
	return func() { meterObservers.Delete(key) }
}

// ReadMeter returns all ledger entries, oldest first.
func ReadMeter() ([]MeterEntry, error) {
	l, err := metering.NewJSONLLedger("")
	if err != nil {
		return nil, err
	}
	return l.All()
}
