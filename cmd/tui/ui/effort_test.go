package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Shift+Tab flips the mode, auto and team (#86): each press is sent to the
// gateway and the footer says which; the effort is not touched (it has
// ctrl+t, #87).
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

// CTRL+T CYCLES THE EFFORT (#87): auto, low, medium, high, auto, each press
// sent to the gateway and shown in the footer; the mode is not touched. The
// keys go through the App, which used to take ctrl+t for a page switch even
// with one page. Mutation check: drop the len(pageAgents()) > 1 guard in
// pages.go, or the ctrl+t case in model_update.go, and this fails.
func TestCtrlTCyclesTheEffort(t *testing.T) {
	var efforts, modes []string
	ops := Ops{
		Effort: func(level string) (string, error) { efforts = append(efforts, level); return level, nil },
		Mode:   func(mode string) (string, error) { modes = append(modes, mode); return mode, nil },
	}
	var mm tea.Model = NewApp(NewPage(PageConfig{Agent: "coder", Paged: true, Ops: ops}), func(string, int) (Model, error) { return Model{}, nil })
	for i, want := range []string{"effort low (ctrl+t)", "effort medium (ctrl+t)", "effort high (ctrl+t)", "effort auto (ctrl+t)"} {
		var cmd tea.Cmd
		mm, cmd = mm.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
		if cmd == nil {
			t.Fatalf("press %d sent nothing", i+1)
		}
		runCmd(cmd)
		if line := mm.(App).Current().status.line(); !strings.Contains(line, want) {
			t.Fatalf("footer after press %d: %q, want %q", i+1, line, want)
		}
	}
	if strings.Join(efforts, ",") != "low,medium,high," || len(modes) != 0 {
		t.Fatalf("efforts sent = %q, modes sent = %q", efforts, modes)
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
		{Status{Effort: "low", EffortUsed: "low", EffortFrom: "chosen"}, "effort low (ctrl+t)"},
		{Status{Mode: "team"}, "team (shift+tab)"},
		{Status{}, "auto (shift+tab)"},
	} {
		if line := c.st.line(); !strings.Contains(line, c.want) {
			t.Errorf("%+v: footer %q lacks %q", c.st, line, c.want)
		}
	}
}

// A PICKER OWNS THE KEYBOARD: ctrl+t while a question waits for an answer
// changes nothing and sends nothing.
func TestCtrlTWaitsForTheQuestion(t *testing.T) {
	m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
	sent := 0
	m.effortOp = func(level string) (string, error) { sent++; return level, nil }
	m.pendingQuestion = &pendingQuestion{}
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	runCmd(cmd)
	if sent != 0 || mm.(Model).status.Effort != "" {
		t.Fatalf("a press under a question sent %d and set %q", sent, mm.(Model).status.Effort)
	}
}

// A REFUSED CHANGE IS NOT SHOWN AS MADE: when the gateway does not take a
// ctrl+t or shift+tab, the window says so and the footer is read back from
// the gateway, so it and the next press start from what the gateway holds.
// Mutation check: return noteResultMsg instead of settingRefusedMsg, or drop
// the status read in its case, and this fails.
func TestARefusedSettingIsReadBack(t *testing.T) {
	for _, c := range []struct {
		key   tea.KeyType
		field func(Status) string
		held  Status
		says  string
	}{
		{tea.KeyCtrlT, func(s Status) string { return s.Effort }, Status{Effort: "high", Mode: "auto"}, "effort not changed"},
		{tea.KeyShiftTab, func(s Status) string { return s.Mode }, Status{Effort: "high", Mode: "auto"}, "mode not changed"},
	} {
		m := NewModel("http://x", "ws/demo", "chan", "", func(a, w, tx, mode string) error { return nil })
		refused := func(string) (string, error) { return "", errors.New("gateway unreachable") }
		m.effortOp, m.modeOp = refused, refused
		m.statusOp = func() Status { return c.held }
		m.status = c.held
		var mm tea.Model = m
		mm, cmd := mm.Update(tea.KeyMsg{Type: c.key})
		if c.field(mm.(Model).status) == c.field(c.held) {
			t.Fatalf("%v: the footer moves at once", c.key)
		}
		mm = settle(mm, cmd, 3)
		if got := c.field(mm.(Model).status); got != c.field(c.held) {
			t.Fatalf("%v refused: the footer shows %q, the gateway holds %q", c.key, got, c.field(c.held))
		}
		if !strings.Contains(mm.(Model).renderMessages(), c.says) {
			t.Fatalf("%v refused: the window does not say %q", c.key, c.says)
		}
	}
}

// settle feeds a command's messages back into the model, depth levels deep,
// opening batches; the status ticker's next tick is a timer and is not fed.
func settle(mm tea.Model, cmd tea.Cmd, depth int) tea.Model {
	if cmd == nil || depth == 0 {
		return mm
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			mm = settle(mm, c, depth)
		}
		return mm
	}
	if _, ok := msg.(statusTickMsg); ok {
		mm, _ = mm.Update(msg)
		return mm
	}
	mm, next := mm.Update(msg)
	return settle(mm, next, depth-1)
}
