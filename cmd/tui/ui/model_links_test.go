package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Paths and URLs become OSC 8 links; slash commands, root fragments and text
// already inside a link do not.
func TestLinkifyLine(t *testing.T) {
	t.Setenv("TMUX", "") // plain OSC 8, not the tmux passthrough wrapper
	home, _ := os.UserHomeDir()

	linked := func(href, text string) string {
		return "\x1b]8;;" + href + "\x1b\\" + text + "\x1b]8;;\x1b\\"
	}
	cases := map[string]string{
		"/Users/g/app/main.go: 66.00s":    linked("file:///Users/g/app/main.go", "/Users/g/app/main.go") + ": 66.00s",
		"saved to ~/notes/a.txt.":         "saved to " + linked("file://"+home+"/notes/a.txt", "~/notes/a.txt") + ".",
		"see https://memdoor.ai/docs, ok": "see " + linked("https://memdoor.ai/docs", "https://memdoor.ai/docs") + ", ok",
		"(in /tmp/x/y.png)":               "(in " + linked("file:///tmp/x/y.png", "/tmp/x/y.png") + ")",
		// Styled text: the escape before the path is not part of it.
		"\x1b[1m/a/b/c.go\x1b[0m": "\x1b[1m" + linked("file:///a/b/c.go", "/a/b/c.go") + "\x1b[0m",
		// Left alone.
		"run /model to see it": "run /model to see it",
		"a/b relative":         "a/b relative",
		"just / slash":         "just / slash",
		"no paths here":        "no paths here",
	}
	for in, want := range cases {
		if got := linkifyLine(in); got != want {
			t.Errorf("linkifyLine(%q)\n got %q\nwant %q", in, got, want)
		}
	}

	// A line that already carries a link is untouched, even with a path in it.
	already := linked("https://x.y/p", "cite") + " /Users/g/a/b.txt"
	if got := linkifyLine(already); got != already {
		t.Errorf("re-linked a line that already had a link:\n%q", got)
	}
	if strings.Contains(linkifyLine("/exit"), "\x1b]8;;") {
		t.Error("a slash command became a link")
	}
}

// Under tmux a path link must stay a plain OSC 8: the DCS passthrough the
// citation helper uses moves the text out of tmux's screen grid, and the
// path vanished from the line (live 2026-09-02).
func TestLinkifyLineNoPassthroughUnderTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,1,0")
	got := linkifyLine("saved /Users/g/a/b.go ok")
	if strings.Contains(got, "\x1bPtmux;") {
		t.Fatalf("path link used tmux passthrough: %q", got)
	}
	if !strings.Contains(got, "\x1b]8;;file:///Users/g/a/b.go\x1b\\/Users/g/a/b.go\x1b]8;;\x1b\\") {
		t.Errorf("expected a plain OSC 8 around the path, got %q", got)
	}
}

// A file name the model wrote without a directory links when the file
// exists in the session directory, and stays plain when it does not.
func TestLinkifyBareFilesInSessionDir(t *testing.T) {
	t.Setenv("TMUX", "")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "report.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "talk.transcript.txt"), []byte("x"), 0o644)
	t.Chdir(dir)
	linked := func(name string) string {
		full := filepath.Join(dir, name)
		return "\x1b]8;;file://" + full + "\x1b\\" + name + "\x1b]8;;\x1b\\"
	}
	cases := map[string]string{
		"1. report.md (30s)":             "1. " + linked("report.md") + " (30s)",
		"cached at talk.transcript.txt.": "cached at " + linked("talk.transcript.txt") + ".",
		"see `report.md` now":            "see `" + linked("report.md") + "` now",
		"missing.md is not there":        "missing.md is not there",
		"edit app.js and main.go":        "edit app.js and main.go",
		"/Users/g/x/report.md: 30.00s":   "\x1b]8;;file:///Users/g/x/report.md\x1b\\/Users/g/x/report.md\x1b]8;;\x1b\\: 30.00s",
	}
	for in, want := range cases {
		if got := linkifyLine(in); got != want {
			t.Errorf("linkifyLine(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
