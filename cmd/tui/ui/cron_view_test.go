package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The header shows the schedule and the bound; the number travels as JSON
// number and must still show.
func TestCronViewShowsScheduleAndBound(t *testing.T) {
	v := viewFor("cron")
	got := v.Label(`{"action":"every","every":"30s","task":"check whether CI passed","times":5}`, 120)
	if got != "Cron(every 30s ×5: check whether CI passed)" {
		t.Fatalf("label = %q", got)
	}
	if got := v.Label(`{"action":"every","every":"2m","task":"watch the deploy"}`, 120); !strings.Contains(got, "×10:") {
		t.Fatalf("the default bound should show: %q", got)
	}
	if got := v.Label(`{"action":"stop","id":"poll-42"}`, 120); got != "Cron(stop poll-42)" {
		t.Fatalf("stop = %q", got)
	}
	if got := v.Label(`{"action":"list"}`, 120); got != "Cron(list)" {
		t.Fatalf("list = %q", got)
	}
	long := strings.Repeat("x", 200)
	if got := v.Label(`{"action":"every","every":"1h","task":"`+long+`"}`, 80); len(got) > 90 || !strings.HasSuffix(got, "…)") {
		t.Fatalf("a long task is cut to the width: %d chars, %q", len(got), got[len(got)-4:])
	}
}

// A scheduled check's answer lands in the transcript as a note, not as a turn.
func TestACronAnswerIsNotedWithoutATurn(t *testing.T) {
	m := NewModel("http://x", "ws", "chan", "", nil)
	step := func(msg tea.Msg) { nm, _ := m.Update(msg); m = nm.(Model) }
	step(tea.WindowSizeMsg{Width: 100, Height: 40})
	step(cronAnswerMsg{jobID: "poll-1", text: "⏱ poll-1 — CI is still running (run 2 of 10)"})
	last := m.messages[len(m.messages)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "poll-1") {
		t.Fatalf("want a system note with the answer, got %+v", last)
	}
	if m.turnRunning {
		t.Fatal("an answer must not start or mark a turn")
	}
}
