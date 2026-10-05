package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"memdoor/pkg/notes"
)

// turnTask is what the gateway injects as turn_task: the request the
// judged read weighs the sections against.
var turnTask string

func withTask(t *testing.T, task string) {
	t.Helper()
	prev := turnTask
	turnTask = task
	t.Cleanup(func() { turnTask = prev })
}

// notesFile writes n sections; the ones in about are about sourdough.
func notesFile(t *testing.T, n int, about map[int]bool) notes.ConversationKey {
	t.Helper()
	key := withNotes(t)
	var b strings.Builder
	for i := 0; i < n; i++ {
		topic := "a cat video reel, unrelated job"
		if about[i] {
			topic = "talk.mp4 sourdough starter and cold proof"
		}
		fmt.Fprintf(&b, "## note — section %d\n\n%s %s\n\n", i, topic, strings.Repeat("padding ", 60))
	}
	if err := notesRepo.Append(key, b.String()); err != nil {
		t.Fatal(err)
	}
	return key
}

func readNotes(t *testing.T, key notes.ConversationKey, all bool) string {
	t.Helper()
	in, _ := json.Marshal(map[string]any{"read": true, "all": all, "conversation": key.String(), "turn_task": turnTask})
	out, err := Notes(in)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestNotesReadJudged(t *testing.T) {
	withDecisions(t, &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "sourdough") {
			return 0.9
		}
		return 0.1
	}})
	withTask(t, "Make a short from talk.mp4 about the starter and the cold proof.")
	dir := notesFile(t, 40, map[int]bool{3: true, 10: true})
	out := readNotes(t, dir, false)
	if !strings.Contains(out, "the 5 of 40 sections that help") {
		t.Fatalf("header: %s", out[:200])
	}
	for _, want := range []string{"section 3\n", "section 10\n", "section 37\n", "section 38\n", "section 39\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "section 4\n") || strings.Contains(out, "section 36\n") {
		t.Errorf("kept an unrelated older section:\n%s", out)
	}
	// all: the unfiltered tail.
	if full := readNotes(t, dir, true); strings.Contains(full, "sections that help") || !strings.Contains(full, "section 36") {
		t.Errorf("all=true must return the unfiltered tail")
	}
}

func TestNotesReadFallsBack(t *testing.T) {
	dir := notesFile(t, 40, map[int]bool{3: true})
	// No task: unfiltered.
	withDecisions(t, &scriptedDecisions{p: func(string) float64 { return 0.1 }})
	withTask(t, "")
	if out := readNotes(t, dir, false); strings.Contains(out, "sections that help") {
		t.Error("judged without a task")
	}
	// No decision model: unfiltered.
	withTask(t, "Make a short from talk.mp4.")
	withDecisions(t, nil)
	if out := readNotes(t, dir, false); strings.Contains(out, "sections that help") {
		t.Error("judged without a model")
	}
	// Short notes: unfiltered even with both.
	withDecisions(t, &scriptedDecisions{p: func(string) float64 { return 0.1 }})
	small := notesFile(t, 3, nil)
	if out := readNotes(t, small, false); strings.Contains(out, "sections that help") {
		t.Error("judged short notes")
	}
}
