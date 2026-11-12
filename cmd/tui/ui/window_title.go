package ui

import (
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// THE TAB SAYS WHETHER IT IS RUNNING (the maintainer, 2026-09-27: "is running state for
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
//	◐ fix the bug    a turn is running (the circle turns: ◐ ◓ ◑ ◒)
//	? fix the bug    the agent asked a question and is waiting for the answer
//	✓ fix the bug    a turn finished and you have not looked yet
//	fix the bug      idle
//
// THE END OF A TURN IS A SIGNAL, NOT JUST THE ABSENCE OF ●  (the maintainer, 2026-09-27:
// "signal the end of a turn from tab"). A dot that quietly disappears is easy
// to miss across a row of tabs; a turn that took four minutes deserves to be
// noticed the moment it lands. So the title keeps a ✓ until the next key or the
// window regaining focus, and — only when the terminal has told us the window
// is NOT in focus — the turn's end also rings: a BEL, which tmux, iTerm2,
// Ghostty/cmux and most terminals turn into a marked tab, and an OSC 9
// notification, which the ones that show notifications turn into one. In the
// window you are looking at, nothing rings: you can see it.
//
// The session title (the first thing asked, model_view.go setTitle) is what
// tells two tabs apart; a window that has not been asked anything yet says
// "memdoor". The marks are one character so a narrow tab still shows the name.

// windowTitle is the title for the current state. Kept separate from the
// command so both paths are testable without a terminal.
func (m Model) windowTitle() string {
	switch name := m.sessionTitle(); {
	case m.pendingQuestion != nil:
		return "? " + name
	case m.working():
		return spinFrames[m.spinFrame%len(spinFrames)] + " " + name
	case m.doneUnseen:
		return "✓ " + name
	default:
		return name
	}
}

// sessionTitle is what the tab calls this conversation: the session's title
// when it has one (setTitle names it after the first thing asked), else
// "memdoor".
func (m Model) sessionTitle() string {
	if m.title != "" {
		return m.title
	}
	return "memdoor"
}

// spinFrames turn the running mark (the maintainer, 2026-09-27: "it should be a spinning
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
		if m.working() {
			m.spinFrame++
		}
	}
	if !m.working() || m.spinning {
		return nil
	}
	m.spinning = true
	return tick(spinEvery, func(time.Time) tea.Msg { return titleSpinMsg{} })
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
// Called once per busy→idle transition (model_update.go). A run that failed
// keeps its failure as the outcome; the ring says what the end is: finished,
// waiting at a gate, or failed.
func (m *Model) turnEnded() tea.Cmd {
	m.doneUnseen = true
	if m.lastOutcome.state == "" {
		m.lastOutcome = windowStatus{state: statusDone}
	}
	if !m.blurred {
		return nil
	}
	return ring(m.windowStatus().endNote(m.sessionTitle()))
}

// ring tells a person who is not looking: cmux's own notification, which the
// next turn can clear (cmux.go), else a BEL and an OSC 9.
func ring(text string) tea.Cmd {
	if bin, _ := cmuxCLI(); bin != "" {
		return cmuxRun("notify", "--title", text)
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
		osc := tmuxPassthrough("\x1b]9;" + strings.ReplaceAll(text, "\x07", "") + "\x07")
		_, _ = os.Stdout.WriteString("\a" + osc)
		return nil
	}
}

// tmuxPassthrough wraps an escape in tmux's DCS passthrough when $TMUX is set,
// with every ESC in the payload doubled, per the tmux protocol. tmux drops a
// bare OSC otherwise. The pane needs allow-passthrough on, which the TUI
// enables at startup.
func tmuxPassthrough(seq string) string {
	if os.Getenv("TMUX") == "" {
		return seq
	}
	return "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
}
