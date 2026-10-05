package ui

import (
	"errors"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// freshModel is a TUI with one exchange behind it, a poster that records the
// turns it sends, and a stub for the gateway's fresh endpoint.
func freshModel(t *testing.T, freshErr error) (*Model, *[]string, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var posted, wiped []string
	m := NewModel("http://x", "ws", "chan", "", func(agent, w, text, mode string) error {
		mu.Lock()
		posted = append(posted, text)
		mu.Unlock()
		return nil
	})
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = nm.(Model)
	nm, _ = m.Update(websocketConnectedMsg{})
	m = nm.(Model)
	m.fresh = func(agent string) error {
		mu.Lock()
		wiped = append(wiped, agent)
		mu.Unlock()
		return freshErr
	}
	m.messages = append(m.messages,
		Message{Role: "user", Content: "fix the login bug"},
		Message{Role: "assistant", Content: "Read 41 files, still looking…"},
	)
	return &m, &posted, &wiped
}

// enter types text and presses Enter; it returns the command Enter produced.
func enter(t *testing.T, m *Model, text string) tea.Cmd {
	t.Helper()
	m.input.SetValue(text)
	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	*m = nm.(Model)
	return cmd
}

// feed runs cmd — opening batches, since stopping a turn also retitles the
// window — and hands the gateway's answer back to the model, as the program
// does. It returns what the model does next.
func feed(t *testing.T, m *Model, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command to run")
	}
	var next tea.Cmd
	var answered bool
	var walk func(c tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			for _, inner := range msg {
				walk(inner)
			}
		case freshMsg, clearedMsg:
			nm, n := m.Update(msg)
			*m = nm.(Model)
			next, answered = n, true
		}
	}
	walk(cmd)
	if !answered {
		t.Fatal("the gateway was not asked")
	}
	return next
}

// /fresh wipes the agent's memory of the conversation and sends nothing: the
// last request re-sent into a clean session can mean nothing there ("go
// ahead" sent the coder off on a task of its own making, live 2026-09-29).
func TestFreshWipesAndSendsNothing(t *testing.T) {
	m, posted, wiped := freshModel(t, nil)
	runCmd(feed(t, m, enter(t, m, "/fresh")))
	if len(*wiped) != 1 || (*wiped)[0] != m.codeAgent {
		t.Fatalf("the gateway must wipe the coder's memory once: %v", *wiped)
	}
	if len(*posted) != 0 {
		t.Fatalf("a bare /fresh sends nothing: %v", *posted)
	}
	if !strings.Contains(m.renderMessages(), "Fresh session") || !strings.Contains(m.renderMessages(), "fix the login bug") {
		t.Error("the screen says a fresh session started, and keeps the conversation")
	}
}

// /fresh <request> sends that request into the clean session.
func TestFreshWithARequestSendsIt(t *testing.T) {
	m, posted, wiped := freshModel(t, nil)
	next := feed(t, m, enter(t, m, "/fresh fix the login bug in auth.go"))
	runCmd(next)
	if len(*wiped) != 1 {
		t.Fatalf("the wipe comes first: %v", *wiped)
	}
	if len(*posted) != 1 || (*posted)[0] != "fix the login bug in auth.go" {
		t.Fatalf("the request must go into the clean session: %v", *posted)
	}
}

// Only after the wipe: if the gateway could not wipe (a turn would not
// stop), nothing is sent — sending into the old context is not starting over.
func TestFreshSendsNothingWhenTheWipeFails(t *testing.T) {
	m, posted, _ := freshModel(t, errors.New("the running turn did not stop in time; try again"))
	runCmd(feed(t, m, enter(t, m, "/fresh fix it")))
	if len(*posted) != 0 {
		t.Fatalf("a request went into the old context: %v", *posted)
	}
	if !strings.Contains(m.renderMessages(), "did not stop in time") {
		t.Error("the reason must be shown")
	}
}

// A turn still running is stopped on the screen first; the gateway's endpoint
// cancels it and waits for it to save before the wipe.
func TestFreshStopsARunningTurn(t *testing.T) {
	m, _, wiped := freshModel(t, nil)
	m.turnRunning = true
	m.queued = []string{"a queued prompt"}
	cmd := enter(t, m, "/fresh")
	if m.turnRunning || len(m.queued) != 0 || !m.interrupted {
		t.Fatalf("the running turn must stop on screen: running=%v queued=%v", m.turnRunning, m.queued)
	}
	runCmd(feed(t, m, cmd))
	if len(*wiped) != 1 {
		t.Fatal("the gateway was not asked to start over")
	}
}

// /clear wipes the agent's memory and starts the screen over — and says
// what it did, not that history is "permanently deleted" (it said so while
// deleting a file that did not exist, 2026-09-29).
func TestClearWipesThroughTheGateway(t *testing.T) {
	m, posted, wiped := freshModel(t, nil)
	m.printedThrough = 3
	feed(t, m, enter(t, m, "/clear"))
	if len(*wiped) != 1 || len(*posted) != 0 {
		t.Fatalf("/clear wipes and sends nothing: wiped=%v posted=%v", *wiped, *posted)
	}
	if m.printedThrough != 0 || len(m.messages) != 1 {
		t.Fatalf("the screen starts over: watermark %d, %d messages", m.printedThrough, len(m.messages))
	}
	shown := m.renderMessages()
	if !strings.Contains(shown, "Cleared") || strings.Contains(shown, "permanently deleted") {
		t.Fatalf("what /clear did:\n%s", shown)
	}
}

func TestClearKeepsTheScreenWhenTheWipeFails(t *testing.T) {
	m, _, _ := freshModel(t, errors.New("unauthorized"))
	before := len(m.messages)
	feed(t, m, enter(t, m, "/clear"))
	if len(m.messages) <= before || !strings.Contains(m.renderMessages(), "Could not wipe") {
		t.Fatal("a failed wipe keeps the conversation on screen and says so")
	}
}

// The line Esc leaves names the way out.
func TestInterruptOffersFresh(t *testing.T) {
	m, _, _ := freshModel(t, nil)
	m.turnRunning = true
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	*m = nm.(Model)
	if last := m.messages[len(m.messages)-1]; !strings.Contains(last.Content, "/fresh") {
		t.Fatalf("the interrupt note: %q", last.Content)
	}
}

// /compact [focus] asks the gateway to summarize now, with the focus; a
// turn that is running owns the conversation, so it is not interrupted.
func TestCompactSendsTheFocus(t *testing.T) {
	m, _, _ := freshModel(t, nil)
	var gotAgent, gotFocus string
	m.compact = func(agent, focus string) (string, error) {
		gotAgent, gotFocus = agent, focus
		return "Compacted: 18.2k → 6.1k tokens", nil
	}
	cmd := enter(t, m, "/compact keep the API decisions")
	if cmd == nil {
		t.Fatal("no request")
	}
	nm, _ := m.Update(cmd())
	*m = nm.(Model)
	if gotAgent != m.codeAgent || gotFocus != "keep the API decisions" {
		t.Fatalf("agent %q focus %q", gotAgent, gotFocus)
	}
	if !strings.Contains(m.renderMessages(), "Compacted: 18.2k") {
		t.Fatal("the result is shown")
	}
	m.turnRunning = true
	gotFocus = "unchanged"
	runCmd(enter(t, m, "/compact"))
	if gotFocus != "unchanged" || !strings.Contains(m.renderMessages(), "A turn is running") {
		t.Fatal("a running turn is not compacted under, and the screen says so")
	}
}

// /handoff shows the summary the next conversation starts from, then sends
// the request into it.
func TestHandoffShowsTheSummaryThenSendsTheRequest(t *testing.T) {
	m, posted, _ := freshModel(t, nil)
	m.handoff = func(agent string) (string, error) { return "Goal: login fix.\nNext steps: 1. test it.", nil }
	next := feed(t, m, enter(t, m, "/handoff write the test"))
	runCmd(next)
	shown := m.renderMessages()
	if !strings.Contains(shown, "Handed off") || !strings.Contains(shown, "Next steps: 1. test it.") {
		t.Fatalf("the handoff is on screen:\n%s", shown)
	}
	if len(*posted) != 1 || (*posted)[0] != "write the test" {
		t.Fatalf("the request goes into the new conversation: %v", *posted)
	}
}
