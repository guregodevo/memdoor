package providers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"memdoor/gateway/logs"
)

func TestMeterSkipsTrailingGarbage(t *testing.T) {
	logs.InitGlobalLogger(t.TempDir(), false)
	home := t.TempDir()
	t.Setenv("HOME", home)
	RecordMeter(MeterEntry{TS: time.Now(), Engine: "e1", InputTokens: 1, OutputTokens: 2})
	p := filepath.Join(home, ".memdoor", "meter.jsonl")
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"ts":"broken`)
	f.Close()
	entries, err := ReadMeter()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("half-written trailing line must not hide history: got %d entries", len(entries))
	}
}
