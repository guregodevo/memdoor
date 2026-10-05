package gateway

import (
	"context"
	"path/filepath"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
)

// A TURN LANDS WHERE ITS WORK IS.
//
// A turn that arrives without a directory — a channel message, a CLI
// dispatch — fell back to the CODER's scratch sandbox whatever the agent
// was. Live 2026-09-17: another agent landed in ~/memdoor-coder, read
// another job's notes, and ran `find` across the home directory looking for
// sources that were all in its own project.
func TestATurnWithNoDirectoryLandsWhereItsWorkIs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	coder := filepath.Join(home, "memdoor-coder")

	code := context.WithValue(context.Background(), sharedctx.BuddyToolsKey,
		[]string{"apply_patch", "bash", "locate"})
	if got := turnWorkdir(code); got != coder {
		t.Fatalf("a coder landed in %q, want its sandbox", got)
	}

	// An agent with no palette at all keeps the old default: the sandbox.
	if got := turnWorkdir(context.Background()); got != coder {
		t.Fatalf("an unknown agent landed in %q, want the sandbox", got)
	}

	// AND A DIRECTORY THE CLIENT SENT ALWAYS WINS — the app's window says
	// where it is, and that is never second-guessed.
	sent := filepath.Join(t.TempDir(), "somewhere")
	withDir := context.WithValue(code, sharedctx.WorkdirKey, sent)
	if got := turnWorkdir(withDir); got != sent {
		t.Fatalf("the client's own directory was overridden with %q", got)
	}
}
