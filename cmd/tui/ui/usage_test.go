package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
)

// stripANSI is used by several tests in this package to read a rendered
// view as plain text.

// THE MONEY LEFT THE WINDOW (2026-09-05). Credits and top-ups are
// operator business behind admin auth on the broker, not
// commands in a window a creator uses — and the model, the rate and the
// balance are not shown at all. What the window offers must be what the
// person in front of it can actually act on.
func TestTheWindowOffersNoMoneyCommands(t *testing.T) {
	m := NewModel("ws://localhost:0/ws", "demo", "chan", "", func(string, string, string, string) error { return nil })
	offered := strings.Join(m.slashCommands, " ")
	for _, gone := range []string{"/topup", "/pause", "/resume"} {
		if strings.Contains(offered, gone) {
			t.Errorf("%s is still offered: %s", gone, offered)
		}
	}
	// USAGE IS NOT MONEY. A subscription may show what has been used —
	// brain time this month, seats — which is what /usage answers; what it
	// cost us stays with the operator (2026-09-05).
	if !strings.Contains(offered, "/usage") {
		t.Errorf("/usage went missing: %s", offered)
	}
	// The commands a person can still act on are untouched.
	for _, kept := range []string{"/model", "/clear"} {
		if !strings.Contains(offered, kept) {
			t.Errorf("%s went missing: %s", kept, offered)
		}
	}
	// /llm is gone; /model pins a model.
	if strings.Contains(offered, "/llm") {
		t.Errorf("/llm is gone: %s", offered)
	}
	if m.runNetworkCmd([]string{"/llm", "x"}) != nil {
		t.Error("/llm must not start any work")
	}
}

// And the status line names the model and no money, whichever front end is
// drawing it (the model came back on 2026-09-26; the money did not).
func TestTheStatusLineNamesTheModelAndNoMoney(t *testing.T) {
	st := Status{Branch: "main",
		Model: "qwen3.8-omni-flash"}
	line := st.line()
	for _, leak := range []string{"0.40", "18.80"} {
		if strings.Contains(line, leak) {
			t.Fatalf("leaked %q: %q", leak, line)
		}
	}
	if !strings.Contains(line, "qwen3.8-omni-flash") {
		t.Fatalf("dropped the model: %q", line)
	}
}

// The footer names the rung and why, next to the model; a one-rung ladder
// says nothing about rungs.
func TestFooterSaysTheRungAndWhy(t *testing.T) {
	s := Status{Model: "deepseek/deepseek-v4.1-flash", Rung: 1, Rungs: 4, Reason: "first rung · held for this session"}
	if line := s.line(); !strings.Contains(line, "rung 1/4") || !strings.Contains(line, "held for this session") {
		t.Fatalf("rung and reason: %q", line)
	}
	s = Status{Model: "z-ai/glm-5.3", Rung: 3, Rungs: 3, Reason: "pinned by you", Pinned: true}
	if line := s.line(); !strings.Contains(line, "rung 3/3 · pinned by you") {
		t.Fatalf("pin: %q", line)
	}
	s = Status{Model: "qwen/qwen3.8-omni-flash", Rung: 1, Rungs: 1}
	if line := s.line(); strings.Contains(line, "rung") {
		t.Fatalf("one rung says nothing: %q", line)
	}
}

// A model pinned by id has no rung to name: the footer says the model and
// that it is pinned.
func TestFooterForAModelPinnedById(t *testing.T) {
	s := Status{Model: "z-ai/glm-5.3", Rung: 0, Rungs: 3, Reason: "pinned by you", Pinned: true}
	line := s.line()
	if !strings.Contains(line, "z-ai/glm-5.3") || !strings.Contains(line, "pinned by you") || strings.Contains(line, "rung") {
		t.Fatalf("%q", line)
	}
}

// A published newer build is named in the footer; /update's success quits
// the window with the restart flag set.
func TestUpdateNoticeAndRestart(t *testing.T) {
	if line := (Status{Model: "m", Update: "memdoor v1.305 is out — /update installs it"}).line(); !strings.Contains(line, "/update installs it") {
		t.Fatalf("%q", line)
	}
	m := NewPage(PageConfig{Agent: "coder", Ops: Ops{Update: func() (string, error) { return "Installed v1.305.", nil }}})
	next, cmd := m.Update(updateResultMsg{summary: "Installed v1.305."})
	m = next.(Model)
	if !m.restartAfterUpdate || cmd == nil {
		t.Fatal("a successful update quits with the restart flag")
	}
	a := NewApp(NewPage(PageConfig{Agent: "coder", Paged: true}), func(string, int) (Model, error) { return Model{}, nil })
	a.pages[0].restartAfterUpdate = true
	if !a.RestartAfterUpdate() {
		t.Fatal("the app reports a page's restart request")
	}
}

// A COMMAND THAT NO LONGER EXISTS MUST NOT REAPPEAR (Greg, 2026-09-27:
// "remove slash commands"): the deleted ones stay deleted.
func TestTheDeletedSlashCommandsAreGone(t *testing.T) {
	{
		offered := strings.Join(loadSlashCommands("coder"), " ")
		for _, gone := range []string{"/wiki ", "/wiki-clone", "/wiki-publish", "/wiki-open", "/wiki-reindex", "/wiki-discover"} {
			if strings.Contains(offered, gone) {
				t.Errorf("offered %s: %s", gone, offered)
			}
		}
		// What a coder came for is untouched.
		for _, kept := range []string{"/model", "/usage"} {
			if !strings.Contains(offered, kept) {
				t.Errorf("lost %s: %s", kept, offered)
			}
		}
	}
}

// An interrupt is something the person did; a failure is something that broke.
// Both paths, because renaming one must not swallow the other (battle test,
// 2026-09-27).
func TestInterruptIsNotReportedAsAFailure(t *testing.T) {
	interrupts := []string{
		`oai request failed: Post "https://openrouter.ai/api/v1/chat/completions": context canceled`,
		"context cancelled",
		"Request canceled by the client",
	}
	for _, e := range interrupts {
		if !wasInterrupted(e) {
			t.Errorf("a cancelled request must read as interrupted: %q", e)
		}
	}
	realFailures := []string{
		"oai request failed: HTTP 429 rate limited",
		"no brain is serving yet",
		"apply_patch: context lines did not match",
		"decision model unavailable: not-configured",
		"HTTP 401 unauthorized: check your OpenRouter key",
	}
	for _, e := range realFailures {
		if wasInterrupted(e) {
			t.Errorf("a real failure must still be reported as one: %q", e)
		}
	}

	// And the message the window shows, both ways.
	m := Model{}
	m.viewport.Height, m.viewport.Width = 10, 80
	next, _ := m.Update(executionFailedMsg{errText: `Post "https://openrouter.ai/…": context canceled`})
	shown := next.(Model).renderMessages()
	if !strings.Contains(shown, "Interrupted") || strings.Contains(shown, "context canceled") {
		t.Errorf("esc must read as an interrupt, with no URL or Go error: %q", shown)
	}
	m2 := Model{}
	m2.viewport.Height, m2.viewport.Width = 10, 80
	next2, _ := m2.Update(executionFailedMsg{errText: "HTTP 401 unauthorized"})
	if shown := next2.(Model).renderMessages(); !strings.Contains(shown, "401") {
		t.Errorf("a real failure keeps its detail: %q", shown)
	}
}

// A note must be on screen, not merely in the slice (battle test, 2026-09-27).
func TestANoteIsDrawnWithoutAnExplicitRefresh(t *testing.T) {
	m := NewModel("", "", "", "", nil)
	m.width, m.height = 80, 24
	m.viewport = viewport.New(80, 10)
	m.note("No model **nope/nope** with hosts that can serve a turn")
	if !strings.Contains(m.viewport.View(), "nope/nope") {
		t.Errorf("the note never reached the screen: %q", m.viewport.View())
	}
	// And a scrolled-up reader is still not yanked by one.
	m2 := NewModel("", "", "", "", nil)
	m2.width, m2.height = 80, 24
	m2.viewport = viewport.New(80, 5)
	m2.messages = []Message{{Role: "assistant", Content: strings.Repeat("line\n", 40)}}
	m2.refreshFollow()
	m2.viewport.GotoTop()
	at := m2.viewport.YOffset
	m2.note("something happened")
	if m2.viewport.YOffset != at {
		t.Errorf("a note moved a scrolled-up reader: %d → %d", at, m2.viewport.YOffset)
	}
}

// stripANSI is what the older tests call plainText.
func stripANSI(s string) string { return plainText(s) }
