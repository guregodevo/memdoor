package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// messageText pulls the concatenated text of a MessageParam's text blocks.
func messageText(m llm.MessageParam) string {
	var b strings.Builder
	for _, blk := range m.Content {
		if blk.OfText != nil {
			b.WriteString(blk.OfText.Text)
		}
	}
	return b.String()
}

// TestCompactSessionFile verifies an oversized shared-session JSONL is trimmed
// to its most-recent sessionKeepMessages records, preserving order and the
// newest content (not the oldest).
func TestCompactSessionFile(t *testing.T) {
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")

	// Write enough fat records to clear both the 1 MiB trigger and the keep
	// count. Each record carries ~3 KB of text; the last one is uniquely
	// marked so we can prove the NEWEST survived.
	total := sessionKeepMessages + 300
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < total; i++ {
		text := fmt.Sprintf("msg-%04d %s", i, strings.Repeat("x", 3000))
		if i == total-1 {
			text = "NEWEST-RECORD " + text
		}
		rec := MessageRecord{Message: llm.NewUserMessage(llm.NewTextBlock(text)), Timestamp: int64(i)}
		data, _ := json.Marshal(rec)
		f.Write(append(data, '\n'))
	}
	f.Close()

	if fi, _ := os.Stat(path); fi.Size() < sessionCompactTriggerBytes {
		t.Fatalf("setup: file %d bytes, expected > trigger %d", fi.Size(), sessionCompactTriggerBytes)
	}

	if err := sp.compactSessionFile(path, "ws:test:channel:test"); err != nil {
		t.Fatal(err)
	}

	records, err := sp.readAllRecords(path, "ws:test:channel:test")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != sessionKeepMessages {
		t.Errorf("kept %d records, want %d", len(records), sessionKeepMessages)
	}
	// The newest record must survive; the oldest must be gone.
	last := records[len(records)-1]
	if !strings.Contains(messageText(last.Message), "NEWEST-RECORD") {
		t.Error("compaction dropped the newest record (kept the wrong tail)")
	}
	if strings.Contains(messageText(records[0].Message), "msg-0000") {
		t.Error("compaction kept the oldest record (did not trim the head)")
	}
}

// TestCompactSessionFileNoOpWhenSmall: a small file is left untouched.
func TestCompactSessionFileNoOpWhenSmall(t *testing.T) {
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "small.jsonl")
	rec := MessageRecord{Message: llm.NewUserMessage(llm.NewTextBlock("hi")), Timestamp: 1}
	data, _ := json.Marshal(rec)
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := sp.compactSessionFile(path, "ws:test:channel:small"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("small file was modified; compaction should be a no-op under the trigger")
	}
}
