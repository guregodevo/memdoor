package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// What counts as a read through bash: every command in a chain or pipeline
// reads. Live 2026-09-28, 51 of 52 bash reads were chains — the note this
// replaced skipped every one of them.
func TestBashReadKind(t *testing.T) {
	for cmd, want := range map[string]string{
		"cat web/src/Landing.tsx":                     "file",
		"sed -n 1,400p gateway/server.go":             "file",
		"sed -n 260,340p a.go; sed -n 374,412p a.go":  "file",
		"grep -rn price web/src | head -200":          "matches",
		"grep -n X a.go | head; sed -n '44,80p' b.go": "matches",
		"cd gateway && grep -rn Relay *.go":           "matches",
		"ls web/src/pages 2>/dev/null | head":         "file",
		"cat a > b":                                   "",
		"go test ./...":                               "",
		"sed -i '' s/a/b/ x.go":                       "",
		"grep -n x a.go; go build ./...":              "",
		"cat $(ls *.go)":                              "",
		`ls -a; echo "---"; ls .agents 2>/dev/null`:   "listing",
		"ls > files.txt":                              "",
		`ls -la spec data; for f in a.md b.md; do echo "== $f"; head -5 $f; done; wc -l a.md`: "file",
		"for t in a b; do python3 $t; done":                                                   "",
		"while true; do curl x; done":                                                         "",
	} {
		if got := bashReadKind(cmd); got != want {
			t.Errorf("%q: got %q, want %q", cmd, got, want)
		}
	}
}

func bashListing(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		if i%10 == 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "func filler%d() { return } // padding padding padding\n", i)
	}
	b.WriteString("\nfunc resumeAfterRestart() { /* the one that matters */ }\n")
	return b.String()
}

// A large read through bash reaches the model judged: the section the task
// needs, not the listing.
func TestBashReadIsJudged(t *testing.T) {
	withDecisions(t, &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "resumeAfterRestart") {
			return 0.95
		}
		return 0.05
	}})
	out := bashListing(200)
	got := bashReadOutput("sed -n 1,400p a.go; sed -n 1,20p b.go", out, "where is a turn resumed after restart")
	if !strings.Contains(got, "judged against this turn's task") || !strings.Contains(got, "resumeAfterRestart") {
		t.Fatalf("not judged:\n%.400s", got)
	}
	if strings.Contains(got, "filler150") || len(got) > len(out)/4 {
		t.Fatalf("judged output kept the listing (%d of %d bytes)", len(got), len(out))
	}

	// No task: whole, with the note.
	if got := bashReadOutput("cat a.go", out, ""); !strings.Contains(got, "filler150") || !strings.Contains(got, "read_file or jread") {
		t.Fatalf("unjudged read must be whole with the note:\n%.200s", got)
	}
	// Small, or not a read: untouched.
	if got := bashReadOutput("cat a.go", "x", "task"); got != "x" {
		t.Fatalf("small read changed: %q", got)
	}
	if got := bashReadOutput("go test ./...", out, "task"); got != strings.TrimSpace(out) {
		t.Fatal("a build's output must never be judged")
	}
}

// A read-only command that exits non-zero is a result: live 2026-09-28 the
// coder's `grep …; echo ---; ls web/src/pages` ran as written, ls found no
// such folder, and the tool said "FAILED … fix IT".
func TestReadOnlyNonZeroExitIsAResult(t *testing.T) {
	dir := t.TempDir()
	in := func(cmd string) json.RawMessage {
		b, _ := json.Marshal(map[string]string{"command": cmd, "cwd": dir})
		return b
	}
	got, err := Bash(in(`grep -rln "relay\|/r/" . --include="*.tsx" --include="*.ts" 2>/dev/null | head; echo ---; ls pages 2>/dev/null`))
	if err != nil {
		t.Fatal(err)
	}
	// "exit status N": BSD ls answers 1 for a missing path, GNU ls 2 — the
	// number is the platform's, the sentence is ours.
	if strings.Contains(got, "FAILED") || strings.Contains(got, "fix IT") || !strings.Contains(got, "---") || !strings.Contains(got, "exit status ") {
		t.Fatalf("a read that found nothing is not a broken command:\n%s", got)
	}
	got, _ = Bash(in(`go build ./does-not-exist`))
	if !strings.Contains(got, "FAILED") {
		t.Fatalf("a build that fails is still a failure:\n%s", got)
	}
}
