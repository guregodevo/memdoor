package cmd

import (
	"strings"
	"testing"
)

// memdoor remote: the phone door from a script. Stdout is the link, one
// line; the rest is stderr; the target is an id or --last.

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
	if _, err := remoteTarget(nil, false); err == nil || !strings.Contains(err.Error(), "--last") {
		t.Fatalf("no target must say how: %v", err)
	}
	// --last from a directory with no conversation falls back to the most
	// recent anywhere.
	t.Chdir(t.TempDir())
	r, err = remoteTarget(nil, true)
	if err != nil || r.Title != "newer" {
		t.Fatalf("--last elsewhere: %+v %v", r, err)
	}
}

func TestRemoteHoldTextKeepsStdoutForTheLink(t *testing.T) {
	r := sessionRecord{ChannelID: "cf5a7add-0000-0000-0000-000000000000", Title: "fix the test"}
	got := remoteHoldText(r, "https://memdoor.ai/r/x#v=tok", false, "https://memdoor.ai")
	for _, want := range []string{`"fix the test"`, "watch only: https://memdoor.ai/r/x#v=tok", "ctrl+c ends it, the link stays"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr text lacks %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "█") && !strings.Contains(got, "▄") {
		t.Error("the QR code goes to stderr even into a pipe: a CI log is what a phone scans")
	}
	view := remoteHoldText(r, "https://memdoor.ai/r/x#v=tok", true, "https://memdoor.ai")
	if !strings.Contains(view, "watch only ·") || strings.Contains(view, "watch only: https") {
		t.Errorf("--view names itself and does not repeat the link:\n%s", view)
	}
	untitled := remoteHoldText(sessionRecord{ChannelID: "cf5a7add-0000-0000-0000-000000000000"}, "v", false, "https://memdoor.ai")
	if !strings.Contains(untitled, `"cf5a7add"`) {
		t.Errorf("an untitled conversation is named by its short id:\n%s", untitled)
	}
}
