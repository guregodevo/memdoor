package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"memdoor/pkg/domain"
)

// The CLI's cronRunRecord must decode the gateway's real CronRunRecord JSON.
// This is the contract that broke: the CLI read "timestamp"/"status" while the
// gateway sent "start_time"/"success", so every history row printed <nil>.
// Marshalling the domain type and decoding it back proves the field names agree
// — a rename on either side fails here instead of silently blanking a column.
func TestCronRunRecordDecodesGatewayJSON(t *testing.T) {
	src := domain.CronRunRecord{
		JobID:      "hello-1m",
		StartTime:  time.Date(2026, 8, 24, 22, 7, 0, 0, time.UTC),
		DurationMs: 1955,
		Success:    true,
		Error:      "",
	}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatal(err)
	}

	var got cronRunRecord
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	if got.JobID != "hello-1m" {
		t.Errorf("job_id did not decode: %q", got.JobID)
	}
	if got.StartTime.IsZero() {
		t.Error("start_time decoded to zero — the <nil> TIME column bug")
	}
	if got.DurationMs != 1955 {
		t.Errorf("duration_ms did not decode: %d", got.DurationMs)
	}
	if !got.Success {
		t.Error("success did not decode — the <nil> STATUS column bug")
	}
}

func TestFormatCronDuration(t *testing.T) {
	cases := map[int64]string{
		0:     "-",
		-1:    "-",
		1955:  "1.955s",
		340:   "340ms",
		60000: "1m0s",
	}
	for ms, want := range cases {
		if got := formatCronDuration(ms); got != want {
			t.Errorf("formatCronDuration(%d) = %q, want %q", ms, got, want)
		}
	}
}
