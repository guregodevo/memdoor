package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// THE TAB SAYS WHETHER IT IS RUNNING (Greg, 2026-09-27: "is running state for
// cmux terminal so that the tab show it is running").
//
// A turn can take minutes. Someone who switched tabs has no way to tell whether
// it is still going, and the answer is already known inside the window — the
// footer's activity bar and the page strip both show it. The terminal title is
// the one place a multiplexer, a tab bar and a dock icon can all read, and it
// costs one escape sequence per state change.
//
// The state, in the words the title uses:
//
//	◐ proj          a turn is running (the circle turns: ◐ ◓ ◑ ◒)
//	? proj          the agent asked a question and is waiting for the answer
//	✓ proj          a turn finished and you have not looked yet
//	proj            idle
//
// THE END OF A TURN IS A SIGNAL, NOT JUST THE ABSENCE OF ●  (Greg, 2026-09-27:
// "signal the end of a turn from tab"). A dot that quietly disappears is easy
// to miss across a row of tabs; a turn that took four minutes deserves to be
// noticed the moment it lands. So the title keeps a ✓ until the next key or the
// window regaining focus, and — only when the terminal has told us the window
// is NOT in focus — the turn's end also rings: a BEL, which tmux, iTerm2,
// Ghostty/cmux and most terminals turn into a marked tab, and an OSC 9
// notification, which the ones that show notifications turn into one. In the
// window you are looking at, nothing rings: you can see it.
//
// The directory comes first because that is what tells two tabs apart. The
// marks are one character so a narrow tab still shows the name.

// windowTitle is the title for the current state. Kept separate from the
// command so both paths are testable without a terminal.
func (m Model) windowTitle() string {
	name := titleDirName()
	switch {
	case m.pendingQuestion != nil:
		return "? " + name
	case m.busy():
		return spinFrames[m.spinFrame%len(spinFrames)] + " " + name
	case m.doneUnseen:
		return "✓ " + name
	default:
		return name
	}
}

// spinFrames turn the running mark (Greg, 2026-09-27: "it should be a spinning
// circle when it's running"): a still dot reads the same as a stuck one.
var spinFrames = []string{"◐", "◓", "◑", "◒"}

// spinEvery is the title's own clock. The window's animation tick runs only
// while a tool executes, so it would freeze the circle through every model
// call; a quarter second turns it without making a tab bar blink.
const spinEvery = 250 * time.Millisecond

type titleSpinMsg struct{}

// spin keeps the circle turning while a turn runs: it starts the clock when a
// turn begins, advances a frame on each tick, and lets the clock stop when the
// turn is over. One clock at a time — spinning says one is already queued.
func (m *Model) spin(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(titleSpinMsg); ok {
		m.spinning = false
		if m.busy() {
			m.spinFrame++
		}
	}
	if !m.busy() || m.spinning {
		return nil
	}
	m.spinning = true
	return tick(spinEvery, func(time.Time) tea.Msg { return titleSpinMsg{} })
}

// titleDirName is the working directory's base name, or "memdoor" when there
// is nothing useful to say (the root, or a directory that cannot be read).
func titleDirName() string {
	d, err := os.Getwd()
	if err != nil || d == "" {
		return "memdoor"
	}
	base := filepath.Base(d)
	if base == "/" || base == "." || strings.TrimSpace(base) == "" {
		return "memdoor"
	}
	return base
}

// titleCmd sets the terminal title when it has changed, and does nothing when
// it has not: a title written on every tick would make some terminals blink.
func (m *Model) titleCmd() tea.Cmd {
	next := m.windowTitle()
	if next == m.lastTitle {
		return nil
	}
	m.lastTitle = next
	return tea.SetWindowTitle(next)
}

// turnEnded records a finished turn and, when the window is out of focus, rings.
// Called once per busy→idle transition (model_update.go).
func (m *Model) turnEnded() tea.Cmd {
	m.doneUnseen = true
	status := cmuxTurnEnded()
	if !m.blurred {
		return status
	}
	text := "Memdoor: turn finished in " + titleDirName()
	if bin, _ := cmuxCLI(); bin != "" {
		// cmux's own notification, which the next turn can clear (cmux.go).
		return tea.Batch(status, cmuxRun("notify", "--title", text))
	}
	return ringTerminal(text)
}

// seen clears the ✓: the person is back.
func (m *Model) seen() { m.doneUnseen = false }

// ringTerminal writes a BEL and an OSC 9 notification straight to the terminal.
// Both are zero-width, and a terminal that knows neither ignores them. Inside
// tmux the OSC goes through its DCS passthrough (the same wrapping OSC 8 links
// use, model_render.go); the BEL needs none — tmux marks the window itself.
func ringTerminal(text string) tea.Cmd {
	return func() tea.Msg {
		osc := "\x1b]9;" + strings.ReplaceAll(text, "\x07", "") + "\x07"
		if os.Getenv("TMUX") != "" {
			osc = "\x1bPtmux;" + strings.ReplaceAll(osc, "\x1b", "\x1b\x1b") + "\x1b\\"
		}
		_, _ = os.Stdout.WriteString("\a" + osc)
		return nil
	}
}
