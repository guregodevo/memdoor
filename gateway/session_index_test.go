package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/pkg/llm"
)

// referenceSave is the save as it was before the index: append, then, for
// SaveMessages, compact by parsing the file.
func referenceSave(t *testing.T, sp *SessionPersistence, path string, recs []MessageRecord, compact bool) {
	t.Helper()
	if err := appendSessionBatch(path, func(w io.Writer) error {
		for _, rec := range recs {
			if _, err := marshalAndWriteRecord(w, rec); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if compact {
		if err := sp.compactSessionFile(path, "reference"); err != nil {
			t.Fatal(err)
		}
	}
}

func randomRecords(r *rand.Rand, n int, carried bool) []MessageRecord {
	recs := make([]MessageRecord, n)
	for i := range recs {
		text := strings.Repeat(string(rune('a'+r.Intn(26))), 100+r.Intn(12_000))
		var msg llm.MessageParam
		switch r.Intn(4) {
		case 0:
			msg = llm.NewUserMessage(llm.NewTextBlock(text))
		case 1:
			msg = llm.NewAssistantMessage(llm.NewTextBlock(text), llm.NewToolUseBlock(fmt.Sprintf("t%d", i), `{"cmd":"ls"}`, "bash"))
		default:
			msg = llm.NewUserMessage(llm.NewToolResultBlock(fmt.Sprintf("t%d", i), text, false))
		}
		recs[i] = MessageRecord{Message: msg, Timestamp: int64(i), Boundary: carried && i == 0, Carried: carried}
	}
	return recs
}

// The indexed save writes the same bytes as append-then-compact, through
// every case the trim rule has: files past 1 MiB, boundaries, a turn longer
// than the kept window, lines the reader skips, records an older writer
// wrote, and a gateway restart that drops the index.
func TestIndexedSaveWritesWhatAppendThenCompactWrote(t *testing.T) {
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(1); seed <= 3; seed++ {
		r := rand.New(rand.NewSource(seed))
		dir := t.TempDir()
		ref, got := filepath.Join(dir, "reference.jsonl"), filepath.Join(dir, "indexed.jsonl")
		crossed := 0
		for step := 0; step < 300; step++ {
			external := ""
			var recs []MessageRecord
			compact := true
			switch k := r.Intn(100); {
			case k < 70:
				recs = randomRecords(r, 1+r.Intn(4), false)
			case k < 80:
				recs, compact = randomRecords(r, 2+r.Intn(30), true), false
			case k < 85:
				recs = randomRecords(r, 1, false)
				recs[0].Message = llm.NewUserMessage(llm.NewToolResultBlock("long", strings.Repeat("z", 500), false))
				for i := 0; i < 40+r.Intn(220); i++ {
					recs = append(recs, recs[0])
				}
			case k < 88:
				external = "not json\n\n"
			case k < 91:
				external = `{"timestamp":7,"message":{"content":[{"text":"old","type":"text"}],"role":"user"},"extra":1}` + "\n"
			case k < 94:
				sp.forgetIndex(got)
				continue
			default:
				recs, compact = randomRecords(r, 1, false), false
			}
			if external != "" {
				for _, p := range []string{ref, got} {
					f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
					if err != nil {
						t.Fatal(err)
					}
					f.WriteString(external)
					f.Close()
				}
				continue
			}
			referenceSave(t, sp, ref, recs, compact)
			if _, err := sp.writeBatch(got, "indexed", recs, compact); err != nil {
				t.Fatal(err)
			}
			want, _ := os.ReadFile(ref)
			have, _ := os.ReadFile(got)
			if !bytes.Equal(want, have) {
				t.Fatalf("seed %d step %d: indexed save wrote %d bytes, append-then-compact %d", seed, step, len(have), len(want))
			}
			if len(want) >= sessionCompactTriggerBytes {
				crossed++
			}
		}
		if crossed == 0 {
			t.Fatalf("seed %d never passed the trigger: the case under test did not run", seed)
		}
	}
}

// A save past the trigger reads the transcript once, to build its index, and
// never parses it again while the index is current.
func TestIndexedSaveParsesTheTranscriptOnce(t *testing.T) {
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fat.jsonl")
	r := rand.New(rand.NewSource(1))
	if _, err := sp.writeBatch(path, "fat", randomRecords(r, 300, false), true); err != nil {
		t.Fatal(err)
	}
	first := sp.indexes[path]
	for i := 0; i < 20; i++ {
		if _, err := sp.writeBatch(path, "fat", randomRecords(r, 2, false), true); err != nil {
			t.Fatal(err)
		}
	}
	if fi, _ := os.Stat(path); fi.Size() < sessionCompactTriggerBytes {
		t.Fatalf("setup: the transcript must stay past the trigger, %d bytes", fi.Size())
	}
	idx := sp.indexes[path]
	if idx == nil || idx == first {
		t.Fatal("the index was not carried from save to save")
	}
	rebuilt, err := buildTranscriptIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.size != idx.size || len(rebuilt.entries) != len(idx.entries) {
		t.Fatalf("carried index drifted from the file: %d/%d bytes, %d/%d records", idx.size, rebuilt.size, len(idx.entries), len(rebuilt.entries))
	}
	for i := range idx.entries {
		if idx.entries[i].off != rebuilt.entries[i].off || idx.entries[i].mark != rebuilt.entries[i].mark {
			t.Fatalf("record %d: carried %+v, file %+v", i, idx.entries[i], rebuilt.entries[i])
		}
	}
}

// A file changed outside the store (rewind, an edit) is indexed again, and
// a file whose last line has no newline takes the old path.
func TestIndexedSaveFollowsAFileChangedUnderIt(t *testing.T) {
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ref, got := filepath.Join(dir, "reference.jsonl"), filepath.Join(dir, "indexed.jsonl")
	r := rand.New(rand.NewSource(2))
	first := randomRecords(r, 3, false)
	referenceSave(t, sp, ref, first, true)
	if _, err := sp.writeBatch(got, "indexed", first, true); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{ref, got} {
		f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0600)
		f.WriteString(`{"message":{"role":"assis`)
		f.Close()
	}
	next := randomRecords(r, 2, false)
	referenceSave(t, sp, ref, next, true)
	if _, err := sp.writeBatch(got, "indexed", next, true); err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(ref)
	have, _ := os.ReadFile(got)
	if !bytes.Equal(want, have) {
		t.Fatal("a torn last line was not saved the way the old path saves it")
	}
	if _, ok := sp.indexes[got]; ok {
		t.Fatal("a file the index cannot describe kept an index")
	}
}

// The trim starts at exactly 1 MiB, as compactSessionFile's size check does,
// and not a byte before.
func TestIndexedSaveTrimsAtExactlyTheTrigger(t *testing.T) {
	for _, short := range []int{0, 1} {
		sp, err := NewSessionPersistence(nil, false)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		ref, got := filepath.Join(dir, "reference.jsonl"), filepath.Join(dir, "indexed.jsonl")
		r := rand.New(rand.NewSource(3))
		var head []MessageRecord
		for _, rec := range randomRecords(r, 260, false) {
			rec.Message = llm.NewUserMessage(llm.NewTextBlock(strings.Repeat("h", 3000)))
			head = append(head, rec)
		}
		referenceSave(t, sp, ref, head, false)
		if _, err := sp.writeBatch(got, "indexed", head, false); err != nil {
			t.Fatal(err)
		}
		fi, _ := os.Stat(ref)
		last := MessageRecord{Message: llm.NewUserMessage(llm.NewTextBlock(""))}
		empty, _ := json.Marshal(last)
		pad := sessionCompactTriggerBytes - int(fi.Size()) - len(empty) - 1 - short
		last.Message = llm.NewUserMessage(llm.NewTextBlock(strings.Repeat("p", pad)))
		referenceSave(t, sp, ref, []MessageRecord{last}, true)
		if _, err := sp.writeBatch(got, "indexed", []MessageRecord{last}, true); err != nil {
			t.Fatal(err)
		}
		want, _ := os.ReadFile(ref)
		have, _ := os.ReadFile(got)
		trimmed := len(want) < sessionCompactTriggerBytes-short
		if !bytes.Equal(want, have) || trimmed != (short == 0) {
			t.Fatalf("%d bytes short of the trigger: trimmed=%v, indexed %d bytes, reference %d", short, trimmed, len(have), len(want))
		}
	}
}
