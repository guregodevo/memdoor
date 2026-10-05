package gateway

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"memdoor/pkg/llm"
)

// fakeTurn is a running turn that, like updateSession, saves its messages
// when it ends — after it has been cancelled — and deregisters after saving.
type fakeTurn struct {
	mu      sync.Mutex
	active  bool
	saved   []string
	stops   bool
	lateBy  time.Duration
	removes int
}

func (f *fakeTurn) steps() freshSteps {
	return freshSteps{
		cancel: func(string) bool {
			f.mu.Lock()
			was := f.active
			f.mu.Unlock()
			if was && f.stops {
				go func() {
					time.Sleep(f.lateBy)
					f.mu.Lock()
					f.saved = append(f.saved, "the cancelled turn's reads")
					f.active = false
					f.mu.Unlock()
				}()
			}
			return was
		},
		running: func(string) bool { f.mu.Lock(); defer f.mu.Unlock(); return f.active },
		forget:  func(string) {},
		remove: func(string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.saved = nil
			f.removes++
			return nil
		},
		wait: 2 * time.Second,
	}
}

// Right after Esc the cancelled turn is still winding down and saves when it
// ends. A fresh start waits for that save, then deletes: the transcript is
// empty afterwards, not re-polluted a moment later.
func TestFreshWaitsForTheCancelledTurnToSave(t *testing.T) {
	f := &fakeTurn{active: true, stops: true, lateBy: 200 * time.Millisecond, saved: []string{"old reads"}}
	cancelled, err := startFresh(f.steps(), "k", "k:agent:coder")
	if err != nil || !cancelled {
		t.Fatalf("cancelled=%v err=%v", cancelled, err)
	}
	time.Sleep(300 * time.Millisecond) // anything late would have landed by now
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.saved) != 0 {
		t.Fatalf("the transcript was re-polluted after the wipe: %v", f.saved)
	}
}

func TestFreshWithNoTurnRunning(t *testing.T) {
	f := &fakeTurn{saved: []string{"old reads"}}
	cancelled, err := startFresh(f.steps(), "k", "k:agent:coder")
	if err != nil || cancelled || len(f.saved) != 0 || f.removes != 1 {
		t.Fatalf("cancelled=%v err=%v saved=%v removes=%d", cancelled, err, f.saved, f.removes)
	}
}

// A turn that does not stop is not wiped under: the caller is told, and
// nothing is deleted.
func TestFreshRefusesWhileATurnWillNotStop(t *testing.T) {
	f := &fakeTurn{active: true, stops: false, saved: []string{"old reads"}}
	s := f.steps()
	s.wait = 200 * time.Millisecond
	if _, err := startFresh(s, "k", "k:agent:coder"); !errors.Is(err, errFreshTurnStillRunning) {
		t.Fatalf("err=%v", err)
	}
	if f.removes != 0 {
		t.Fatal("a transcript was deleted under a running turn")
	}
}

// The real store: the agent's transcript for the conversation is deleted,
// the conversation's shared log and another agent's transcript are not.
func TestFreshDeletesOnlyTheAgentsTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	key := "workspace:ws1:channel:ch1"
	msg := llm.MessageParam{Role: llm.MessageParamRoleUser, Content: []llm.ContentBlockParamUnion{llm.NewTextBlock("hi")}}
	for _, k := range []string{key, transcriptKey(key, "coder"), transcriptKey(key, "planner")} {
		if err := sp.SaveMessages(k, []llm.MessageParam{msg}); err != nil {
			t.Fatal(err)
		}
	}
	if err := sp.DeleteSession(transcriptKey(key, "coder")); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(home, ".memdoor", "workspaces", "ws1", "channels", "ch1")
	if _, err := os.Stat(filepath.Join(base, "agents", "coder", "sessions.json")); !os.IsNotExist(err) {
		t.Fatalf("the coder's transcript must be gone: %v", err)
	}
	for _, keep := range []string{filepath.Join(base, "sessions.json"), filepath.Join(base, "agents", "planner", "sessions.json")} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("%s must stay: %v", keep, err)
		}
	}
}
