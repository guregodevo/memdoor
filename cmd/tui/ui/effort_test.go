package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Shift+Tab cycles the reasoning effort (omp's app.thinking.cycle): auto, low,
// medium, high, auto — each sent to the gateway, and the footer says which.
func TestShiftTabCyclesTheEffort(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	var sent []string
	m.effortOp = func(level string) (string, error) { sent = append(sent, level); return level, nil }
	var mm tea.Model = m
	for i := 0; i < 4; i++ {
		var cmd tea.Cmd
		mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		if cmd == nil {
			t.Fatalf("press %d sent nothing", i+1)
		}
		runCmd(cmd)
		if i == 0 {
			if line := mm.(Model).status.line(); !strings.Contains(line, "effort low") {
				t.Fatalf("footer after one press: %q", line)
			}
		}
	}
	if strings.Join(sent, ",") != "low,medium,high," {
		t.Fatalf("levels sent = %q, want low,medium,high, then auto", sent)
	}
	if line := mm.(Model).status.line(); !strings.Contains(line, "effort auto") {
		t.Fatalf("footer back on auto: %q", line)
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
		{Status{Effort: "low", EffortUsed: "low", EffortFrom: "chosen"}, "effort low (shift+tab)"},
	} {
		if line := c.st.line(); !strings.Contains(line, c.want) {
			t.Errorf("%+v: footer %q lacks %q", c.st, line, c.want)
		}
	}
}
