package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExec(t *testing.T, dir, rel, body string) string {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return abs
}

// The programs a target runs are the project files in command position,
// or the file an interpreter is given; the data a check reads is not one,
// and nothing outside the project is.
func TestTargetProgramsAreTheProjectFilesTheCheckRuns(t *testing.T) {
	dir := t.TempDir()
	writeExec(t, dir, "never.sh", "exit 1\n")
	writeExec(t, dir, "scripts/verify.sh", "exit 0\n")
	writeExec(t, dir, "grades/x.json", "{}")
	writeExec(t, dir, "rules/x.start", "")
	cases := map[string][]string{
		"./never.sh": {"never.sh"},
		"sh scripts/verify.sh && test -f grades/x.json": {"scripts/verify.sh"},
		`test "grades/x.json" -nt "rules/x.start"`:      nil,
		"make check":    nil,
		"go test ./...": nil,
		"../never.sh":   nil,
		"bash -e scripts/verify.sh | tee out; ./never.sh": {"never.sh", "scripts/verify.sh"},
		filepath.Join(dir, "never.sh"):                    {"never.sh"},
	}
	for cmd, want := range cases {
		got := targetPrograms(cmd, dir)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q: programs %v, want %v", cmd, got, want)
		}
	}
}

// A writing tool that names the check is refused; after any tool, a check
// that was changed anyway is put back, bytes and mode, and named.
func TestTheTaskCannotChangeItsOwnCheck(t *testing.T) {
	dir := t.TempDir()
	abs := writeExec(t, dir, "never.sh", "#!/bin/sh\nexit 1\n")
	s := &Session{Metadata: map[string]interface{}{}}
	if kept := snapshotTargetPrograms(s, "./never.sh", dir); len(kept) != 1 || kept[0] != "never.sh" {
		t.Fatalf("kept %v", kept)
	}
	for _, in := range []string{`{"path":"never.sh","content":"exit 0"}`, `{"file_path":"./never.sh"}`, `{"patch":"*** Update File: never.sh\n"}`} {
		if msg := targetGuardRefuses(s, "write_file", []byte(in)); !strings.Contains(msg, "never.sh is the check this task is done by") {
			t.Errorf("write %s: %q", in, msg)
		}
	}
	if msg := targetGuardRefuses(s, "write_file", []byte(`{"path":"ok.txt"}`)); msg != "" {
		t.Errorf("another file is the task's: %q", msg)
	}
	if msg := targetGuardRefuses(s, "write_file", []byte(`{"path":"`+abs+`","content":"exit 0"}`)); msg == "" {
		t.Error("the absolute path is the same file")
	}
	for _, in := range []string{
		`{"path":"mynever.sh","content":"exit 0"}`,
		`{"path":"notes.md","content":"the check is ./never.sh and it fails"}`,
		`{"path":"scripts/never.sh","content":"exit 0"}`,
		`{"file_path":"README.md","old_string":"never.sh","new_string":"never.sh (the check)"}`,
	} {
		if msg := targetGuardRefuses(s, "write_file", []byte(in)); msg != "" {
			t.Errorf("not the check, refused: %s: %q", in, msg)
		}
	}
	if msg := targetGuardRefuses(s, "bash", []byte(`{"command":"./never.sh"}`)); msg != "" {
		t.Errorf("running the check is allowed: %q", msg)
	}
	if changed := restoreTargetPrograms(s, dir); len(changed) != 0 {
		t.Fatalf("nothing changed yet: %v", changed)
	}
	// A shell redirect got past the tools: put back.
	if err := os.WriteFile(abs, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed := restoreTargetPrograms(s, dir); len(changed) != 1 || changed[0] != "never.sh" {
		t.Fatalf("restored %v", changed)
	}
	body, _ := os.ReadFile(abs)
	st, _ := os.Stat(abs)
	if string(body) != "#!/bin/sh\nexit 1\n" || st.Mode().Perm() != 0o755 {
		t.Fatalf("as it was: %q %v", body, st.Mode())
	}
	// Deleted: put back too.
	_ = os.Remove(abs)
	if changed := restoreTargetPrograms(s, dir); len(changed) != 1 {
		t.Fatalf("a removed check comes back: %v", changed)
	}
	// A file target keeps nothing.
	snapshotTargetPrograms(s, "", dir)
	if len(targetProgramsOf(s)) != 0 {
		t.Fatal("cleared")
	}
}
