package cmd

import (
	"os"
	"strings"
	"testing"
)

// withStdin runs f with os.Stdin read from a pipe holding text: not a
// terminal, so nothing is asked.
func withStdin(t *testing.T, text string, f func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.WriteString(text)
	_ = w.Close()
	saved := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = saved; _ = r.Close() }()
	f()
}

// withTerminalStdin runs f as if a person at a keyboard typed text: the
// question is asked, and the answer read from a pipe holding it.
func withTerminalStdin(t *testing.T, text string, f func()) {
	t.Helper()
	saved := remoteAsks
	remoteAsks = func() bool { return true }
	defer func() { remoteAsks = saved }()
	withStdin(t, text, f)
}

func TestRemoteTargetByIDAndLast(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	recordSession(sessionRecord{ChannelID: "aaaa1111-0000-0000-0000-000000000001", ChannelName: "a", Title: "older", Dir: "/p/one", Workspace: "w"})
	recordSession(sessionRecord{ChannelID: "bbbb2222-0000-0000-0000-000000000002", ChannelName: "b", Title: "newer", Dir: "/p/two", Workspace: "w"})
	recs := loadSessionIndex()
	if len(recs) != 2 {
		t.Skipf("the index did not record under this HOME (%d records); the id path is covered below", len(recs))
	}

	r, err := remoteTarget([]string{"bbbb2222"}, false)
	if err != nil || r.Title != "newer" {
		t.Fatalf("by id prefix: %+v %v", r, err)
	}
	if _, err := remoteTarget([]string{"zzzz"}, false); err == nil {
		t.Fatal("an unknown id must fail")
	}
	// In a pipe nothing can be asked: the error says how to name one. At a
	// terminal the list is shown and a row or id read; an empty answer
	// picks nothing.
	withStdin(t, "", func() {
		saved := remoteAsks
		remoteAsks = func() bool { return false }
		defer func() { remoteAsks = saved }()
		if _, err := remoteTarget(nil, false); err == nil || !strings.Contains(err.Error(), "--last") {
			t.Fatalf("no target in a pipe must say how: %v", err)
		}
	})
	withTerminalStdin(t, "1\n", func() {
		r, err := remoteTarget(nil, false)
		if err != nil || r.Title != "newer" {
			t.Fatalf("row 1 from the list: %+v %v", r, err)
		}
	})
	withTerminalStdin(t, "\n", func() {
		if _, err := remoteTarget(nil, false); err == nil || !strings.Contains(err.Error(), "picked") {
			t.Fatalf("an empty answer picks nothing: %v", err)
		}
	})
	// --last from a directory with no conversation falls back to the most
	// recent anywhere.
	t.Chdir(t.TempDir())
	r, err = remoteTarget(nil, true)
	if err != nil || r.Title != "newer" {
		t.Fatalf("--last elsewhere: %+v %v", r, err)
	}
}

func TestGatewayRemoteTextKeepsStdoutForTheLink(t *testing.T) {
	r := sessionRecord{ChannelID: "cf5a7add-0000-0000-0000-000000000000", Title: "fix the test"}
	got := remoteLinkText(r, "https://memdoor.ai/r/x#v=tok", false, "https://memdoor.ai")
	for _, want := range []string{`"fix the test"`, "watch only: https://memdoor.ai/r/x#v=tok", "held by your gateway"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr text lacks %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "█") && !strings.Contains(got, "▄") {
		t.Error("the QR code goes to stderr even into a pipe: a job's log is what a phone scans")
	}
	view := remoteLinkText(r, "https://memdoor.ai/r/x#v=tok", true, "https://memdoor.ai")
	if !strings.Contains(view, "watch only ·") || strings.Contains(view, "watch only: https") {
		t.Errorf("--view names itself and does not repeat the link:\n%s", view)
	}
	untitled := remoteLinkText(sessionRecord{ChannelID: "cf5a7add-0000-0000-0000-000000000000"}, "v", false, "https://memdoor.ai")
	if !strings.Contains(untitled, `"cf5a7add"`) {
		t.Errorf("an untitled conversation is named by its short id:\n%s", untitled)
	}
}
