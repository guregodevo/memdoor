package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Shift+Tab flips the mode, auto and team (#86): each press is sent to the
// gateway and the footer says which; the effort is not touched (#87 gives it
// its own key).
func TestShiftTabFlipsTheMode(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	var sent, efforts []string
	m.modeOp = func(mode string) (string, error) { sent = append(sent, mode); return mode, nil }
	m.effortOp = func(level string) (string, error) { efforts = append(efforts, level); return level, nil }
	var mm tea.Model = m
	for i, want := range []string{"team (shift+tab)", "auto (shift+tab)", "team (shift+tab)"} {
		var cmd tea.Cmd
		mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		if cmd == nil {
			t.Fatalf("press %d sent nothing", i+1)
		}
		runCmd(cmd)
		if line := mm.(Model).status.line(); !strings.Contains(line, want) {
			t.Fatalf("footer after press %d: %q, want %q", i+1, line, want)
		}
	}
	if strings.Join(sent, ",") != "team,auto,team" || len(efforts) != 0 {
		t.Fatalf("modes sent = %q, efforts sent = %q", sent, efforts)
	}
}

// The footer shows the effort in use, not only "auto".
func TestTheFooterShowsTheEffortInUse(t *testing.T) {
	for _, c := range []struct {
		st   Status
		want string
	}{
		{Status{}, "effort auto"},
		{Status{EffortUsed: "high", EffortFrom: "default"}, "effort high · auto"},
		{Status{EffortUsed: "medium", EffortFrom: "decision model"}, "effort medium · auto (Jev)"},
		{Status{Effort: "low", EffortUsed: "low", EffortFrom: "chosen"}, "effort low"},
		{Status{Mode: "team"}, "team (shift+tab)"},
		{Status{}, "auto (shift+tab)"},
	} {
		if line := c.st.line(); !strings.Contains(line, c.want) {
			t.Errorf("%+v: footer %q lacks %q", c.st, line, c.want)
		}
	}
}
