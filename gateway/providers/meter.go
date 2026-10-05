package providers

import (
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
}

// ReadMeter returns all ledger entries, oldest first.
func ReadMeter() ([]MeterEntry, error) {
	l, err := metering.NewJSONLLedger("")
	if err != nil {
		return nil, err
	}
	return l.All()
}
