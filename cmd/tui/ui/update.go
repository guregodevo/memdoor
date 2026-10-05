package ui

// Update from the TUI (docs/roadmap/MUST.md, 2026-09-26): a seat holder never
// runs a shell. The footer's status carries "memdoor vX is out — /update
// installs it" when memdoor.ai/dl/VERSION is newer (cmd/cli checks once a
// day, off the UI loop); /update runs the same installer `memdoor upgrade`
// runs, and when it succeeds the window quits and cmd/cli re-executes the
// new binary into this conversation (memdoor resume --last).

// updateResultMsg is the installer's outcome.
type updateResultMsg struct {
	summary string
	err     error
}

// RestartAfterUpdate reports that the new binary is in place and the
// window quit to be re-executed into the same conversation.
func (m Model) RestartAfterUpdate() bool { return m.restartAfterUpdate }

// RestartAfterUpdate on the App: any page asked for it.
func (a App) RestartAfterUpdate() bool {
	for _, p := range a.pages {
		if p.restartAfterUpdate {
			return true
		}
	}
	return false
}
