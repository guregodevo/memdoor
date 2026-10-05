package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// A TUI small enough to assert on: it shows its size and what was typed,
// and q quits it.
type ttyProbe struct {
	cols  int
	typed string
}

func (ttyProbe) Init() tea.Cmd { return nil }

func (m ttyProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.cols = msg.Width
	case tea.KeyMsg:
		if msg.String() == "q" {
			return m, tea.Quit
		}
		m.typed += msg.String()
	}
	return m, nil
}

func (m ttyProbe) View() string { return fmt.Sprintf("cols=%d typed=[%s]", m.cols, m.typed) }

// screenUntil reads the phone's frames until the screen holds want, or the
// program ends when ended is asked for.
func screenUntil(t *testing.T, r *remoteRelay, want string, ended bool) {
	t.Helper()
	var screen strings.Builder
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-r.ttyOut:
			var ev struct {
				Stream string `json:"stream"`
				Data   struct {
					Out   string `json:"out"`
					Ended bool   `json:"ended"`
				} `json:"data"`
			}
			if err := json.Unmarshal(b, &ev); err != nil || ev.Stream != remoteTTYStream {
				t.Fatalf("not a tty frame: %s", b)
			}
			out, err := base64.StdEncoding.DecodeString(ev.Data.Out)
			if err != nil {
				t.Fatalf("screen bytes are not base64: %v", err)
			}
			screen.Write(out)
			if ended && ev.Data.Ended {
				return
			}
			if !ended && strings.Contains(screen.String(), want) {
				return
			}
		case <-deadline:
			t.Fatalf("the phone never showed %q (ended=%v); screen so far: %q", want, ended, screen.String())
		}
	}
}

// The page's terminal view is a real program: it draws at the page's size,
// takes the page's keys, follows a resize, and says when it has quit.
func TestRemoteTTYRunsTheProgramForThePage(t *testing.T) {
	r := newRemoteRelay(remoteSession{TTY: func() tea.Model { return ttyProbe{} }}, remoteLink{})
	t.Cleanup(r.Close)

	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 60, Rows: 20, Fresh: true}}, true)
	screenUntil(t, r, "cols=60", false)

	r.handle(nil, remoteRelayTurn{TTYIn: "hi"}, true)
	screenUntil(t, r, "typed=[hi]", false)

	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 90, Rows: 20}}, true)
	screenUntil(t, r, "cols=90", false)

	// A size no terminal has is clamped, not obeyed.
	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 100000, Rows: 1}}, true)
	screenUntil(t, r, fmt.Sprintf("cols=%d", remoteTTYMaxCols), false)

	r.handle(nil, remoteRelayTurn{TTYIn: "q"}, true)
	screenUntil(t, r, "", true)
}

// replayFor reads frames until the one addressed to forID, and returns its
// screen bytes (or none when there is no program).
func replayFor(t *testing.T, r *remoteRelay, forID string) (screen string, none bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-r.ttyOut:
			var ev struct {
				Data struct {
					For    string `json:"for"`
					Replay string `json:"replay"`
					None   bool   `json:"none"`
				} `json:"data"`
			}
			if json.Unmarshal(b, &ev) != nil || ev.Data.For != forID {
				continue
			}
			out, _ := base64.StdEncoding.DecodeString(ev.Data.Replay)
			return string(out), ev.Data.None
		case <-deadline:
			t.Fatalf("no replay for %q", forID)
		}
	}
}

// A page that opens while another is using the screen joins it: the program
// is not restarted (what was typed stays), and the new page is replayed the
// screen so far, addressed to it alone.
func TestRemoteTTYLatePageJoinsTheScreen(t *testing.T) {
	r := newRemoteRelay(remoteSession{TTY: func() tea.Model { return ttyProbe{} }}, remoteLink{})
	t.Cleanup(r.Close)

	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 60, Rows: 20, Fresh: true}}, true)
	r.handle(nil, remoteRelayTurn{TTYIn: "old"}, true)
	screenUntil(t, r, "typed=[old]", false)

	// A second driving page opens, at its own size: a resize, not a restart.
	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 70, Rows: 20, Fresh: true}}, true)
	screenUntil(t, r, "cols=70 typed=[old]", false)

	r.handle(nil, remoteRelayTurn{Watch: "page-2"}, true)
	screen, none := replayFor(t, r, "page-2")
	if none || !strings.Contains(screen, "typed=[old]") {
		t.Fatalf("the late page must be replayed the screen so far: none=%v %q", none, screen)
	}
}

// A view-only page watches: it is replayed the screen and the conversation,
// and its keys, turns, stops and resizes are dropped.
func TestRemoteViewOnlyPageCannotDrive(t *testing.T) {
	var posted []string
	r := newRemoteRelay(remoteSession{
		TTY:  func() tea.Model { return ttyProbe{} },
		Post: func(text string) error { posted = append(posted, text); return nil },
	}, remoteLink{})
	t.Cleanup(r.Close)

	// No program yet: a watcher cannot start one, and is told so.
	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 40, Rows: 20, Fresh: true}}, false)
	r.handle(nil, remoteRelayTurn{Watch: "viewer"}, false)
	if _, none := replayFor(t, r, "viewer"); !none {
		t.Fatal("a view-only page must not start the screen")
	}

	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 60, Rows: 20, Fresh: true}}, true)
	screenUntil(t, r, "cols=60", false)
	r.handle(nil, remoteRelayTurn{TTYIn: "x"}, false)
	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 90, Rows: 20}}, false)
	r.handle(nil, remoteRelayTurn{Text: "delete everything"}, false)
	r.handle(nil, remoteRelayTurn{TTYIn: "y"}, true)
	screenUntil(t, r, "cols=60 typed=[y]", false)
	if len(posted) != 0 {
		t.Fatalf("a view-only page posted a turn: %v", posted)
	}

	r.handle(nil, remoteRelayTurn{Watch: "viewer"}, false)
	if screen, _ := replayFor(t, r, "viewer"); !strings.Contains(screen, "typed=[y]") {
		t.Fatalf("a view-only page is replayed the screen: %q", screen)
	}
}

// Without a TUI to run (a session that gives none), the request is ignored.
func TestRemoteTTYWithoutATUIIsIgnored(t *testing.T) {
	r := newRemoteRelay(remoteSession{}, remoteLink{})
	t.Cleanup(r.Close)
	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 60, Rows: 20, Fresh: true}}, true)
	r.handle(nil, remoteRelayTurn{TTYIn: "x"}, true)
	select {
	case b := <-r.ttyOut:
		t.Fatalf("a frame with no program: %s", b)
	case <-time.After(200 * time.Millisecond):
	}
}

// Keys arrive one frame each when someone types: they reach the program in
// the order they were typed (live 2026-09-28: "/help" arrived as "/hple").
func TestRemoteTTYKeepsTypedOrder(t *testing.T) {
	r := newRemoteRelay(remoteSession{TTY: func() tea.Model { return ttyProbe{} }}, remoteLink{})
	t.Cleanup(r.Close)
	r.handle(nil, remoteRelayTurn{TTY: &remoteTTYSize{Cols: 200, Rows: 20, Fresh: true}}, true)
	screenUntil(t, r, "cols=200", false)

	text := strings.Repeat("abcdefghijklmnop", 8)
	for _, c := range text {
		r.handle(nil, remoteRelayTurn{TTYIn: string(c)}, true)
	}
	screenUntil(t, r, "typed=["+text+"]", false)
}
