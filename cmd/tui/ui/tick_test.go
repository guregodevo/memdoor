package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Tests drive ticks by hand; waiting the real delay only makes the suite
// slow (every runCmd of a tick slept it out).
func init() {
	tick = func(_ time.Duration, fn func(time.Time) tea.Msg) tea.Cmd {
		return tea.Tick(time.Millisecond, fn)
	}
}
