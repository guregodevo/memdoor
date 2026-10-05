package cmd

import "testing"

// A coding agent's binary lists a coding agent's commands (Greg, 2026-09-27:
// "this is coding agent only", "less is more"). Hidden is not removed: the
// command must still be there to run.
func TestOnlyCodingCommandsAreListed(t *testing.T) {
	hideNonCodingCommands()
	byName := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		byName[c.Name()] = c.Hidden
	}
	for _, n := range []string{"tui", "model", "savings", "resume", "workflow", "providers", "connect", "logs", "upgrade"} {
		hidden, ok := byName[n]
		if !ok {
			t.Errorf("%q must exist: it is the coding agent", n)
			continue
		}
		if hidden {
			t.Errorf("%q must stay listed", n)
		}
	}
	for _, n := range []string{"billing", "channels", "chrome"} {
		hidden, ok := byName[n]
		if !ok {
			continue // a command this build does not register
		}
		if !hidden {
			t.Errorf("%q is not what someone installed a coding agent for", n)
		}
	}

}
