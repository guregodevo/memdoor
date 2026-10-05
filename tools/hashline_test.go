package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const hlSrc = `package calc

// Add adds.
func Add(a, b int) int {
	return a - b
}

// Old is unused.
func Old() {}
`

func hashlineOn(t *testing.T) {
	t.Helper()
	SetEditFormat(func() string { return EditFormatHashline })
	t.Cleanup(func() { SetEditFormat(nil) })
	patchVerify = false
	t.Cleanup(func() { patchVerify = true })
}

// readHL writes src to dir/calc.go and reads it the way read_file does.
func readHL(t *testing.T, dir, src string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, "calc.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, readableContent(path, []byte(src), "", true, 0, 0)
}

func applyHL(t *testing.T, dir, patch string) (string, error) {
	t.Helper()
	in, _ := json.Marshal(map[string]string{"input": patch, "cwd": dir})
	return ApplyPatch(json.RawMessage(in))
}

func TestFileTagIgnoresTrailingWhitespace(t *testing.T) {
	if FileTag("a\nb\n") != FileTag("a  \r\nb\t\n") {
		t.Fatal("trailing whitespace changed the tag")
	}
	if FileTag("a\nb\n") == FileTag("a\nc\n") {
		t.Fatal("different text, same tag")
	}
}

func TestHashlineReadNumbersAndTags(t *testing.T) {
	hashlineOn(t)
	path, out := readHL(t, t.TempDir(), hlSrc)
	want := "[" + path + "#" + FileTag(hlSrc) + "]\n1:package calc\n"
	if !strings.HasPrefix(out, want) || !strings.Contains(out, "\n5:\treturn a - b\n") {
		t.Fatalf("read:\n%s", out)
	}
	SetEditFormat(nil)
	if out := readableContent(path, []byte(hlSrc), "", true, 0, 0); out != hlSrc {
		t.Fatalf("patch format must read the file unchanged, got:\n%s", out)
	}
}

func TestHashlineReplaceInsertDelete(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	path, _ := readHL(t, dir, hlSrc)
	tag := FileTag(hlSrc)
	out, err := applyHL(t, dir, "[calc.go#"+tag+"]\n"+
		"replace 5-5 \"return a\":\n+\treturn a + b\n"+
		"insert after 6:\n+\n+// Sub subtracts.\n+func Sub(a, b int) int { return a - b }\n"+
		"delete 8-9 \"// Old\"\n")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := "package calc\n\n// Add adds.\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\n// Sub subtracts.\nfunc Sub(a, b int) int { return a - b }\n\n"
	if string(got) != want {
		t.Fatalf("file:\n%q\nwant\n%q", got, want)
	}
	// The answer carries the new tag and numbered lines around the change.
	if !strings.Contains(out, "[calc.go#"+FileTag(want)+"]") || !strings.Contains(out, "5:\treturn a + b") {
		t.Fatalf("answer lacks fresh anchors:\n%s", out)
	}
}

func TestHashlineGuardCatchesOffByOne(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	path, _ := readHL(t, dir, hlSrc)
	_, err := applyHL(t, dir, "[calc.go#"+FileTag(hlSrc)+"]\nreplace 4-4 \"return a\":\n+\treturn a + b\n")
	if err == nil || !strings.Contains(err.Error(), `line starts "func Add`) {
		t.Fatalf("want a guard refusal, got %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != hlSrc {
		t.Fatal("a refused edit wrote the file")
	}
	// The refusal shows the current numbered lines, so the retry needs no read.
	if !strings.Contains(err.Error(), "5:\treturn a - b") {
		t.Fatalf("refusal lacks the window:\n%v", err)
	}
}

// Two edits in one turn from one read: the second patch still names the
// first read's tag and numbers, and lands through the snapshot's line map.
func TestHashlineStaleTagRemapsThroughSnapshot(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	path, _ := readHL(t, dir, hlSrc)
	tag := FileTag(hlSrc)
	if _, err := applyHL(t, dir, "[calc.go#"+tag+"]\ninsert before 1:\n+// Package calc.\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := applyHL(t, dir, "[calc.go#"+tag+"]\nreplace 5-5 \"return\":\n+\treturn a + b\n"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "// Package calc.\npackage calc") || !strings.Contains(string(got), "\treturn a + b\n") {
		t.Fatalf("file:\n%s", got)
	}
	// A line the first edit changed cannot be edited through the old read.
	_, err := applyHL(t, dir, "[calc.go#"+tag+"]\nreplace 5-5 \"return\":\n+\treturn 0\n")
	if err == nil || !strings.Contains(err.Error(), "not all lines of that read unchanged") {
		t.Fatalf("want a refusal for a changed line, got %v", err)
	}
}

func TestHashlineUnknownTagRefused(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "calc.go"), []byte(hlSrc), 0o644)
	_, err := applyHL(t, dir, "[calc.go#0000]\nreplace 5-5 \"return\":\n+\treturn a + b\n")
	if err == nil || !strings.Contains(err.Error(), "that read is not known") || !strings.Contains(err.Error(), "#"+FileTag(hlSrc)+"]") {
		t.Fatalf("want a stale refusal naming the current tag, got %v", err)
	}
}

func TestHashlineOverlapRefused(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	path, _ := readHL(t, dir, hlSrc)
	_, err := applyHL(t, dir, "[calc.go#"+FileTag(hlSrc)+"]\nreplace 4-6 \"func\":\n+x\ndelete 5-5 \"return\"\n")
	if err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("want overlap refusal, got %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != hlSrc {
		t.Fatal("a refused edit wrote the file")
	}
}

func TestHashlineStripsPastedNumbers(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	path, _ := readHL(t, dir, hlSrc)
	if _, err := applyHL(t, dir, "[calc.go#"+FileTag(hlSrc)+"]\nreplace 5-5 \"return\":\n+5:\treturn a + b\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !strings.Contains(string(got), "\n\treturn a + b\n") {
		t.Fatalf("prefix kept:\n%s", got)
	}
}

// A line the read cut (judged reads cut long lines with …) was not seen:
// replacing it would drop what the model never read.
func TestHashlineUnseenLineRefused(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "calc.go")
	_ = os.WriteFile(path, []byte(hlSrc), 0o644)
	lines := addressableLines(hlSrc)
	seen := make([]bool, len(lines))
	for i := 0; i < 4; i++ {
		seen[i] = true
	}
	recordSnapshot(path, FileTag(hlSrc), lines, seen)
	_, err := applyHL(t, dir, "[calc.go#"+FileTag(hlSrc)+"]\nreplace 5-5 \"return\":\n+\treturn a + b\n")
	if err == nil || !strings.Contains(err.Error(), "was not shown in full") {
		t.Fatalf("want unseen refusal, got %v", err)
	}
}

func TestHashlineNoOpRefused(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	readHL(t, dir, hlSrc)
	_, err := applyHL(t, dir, "[calc.go#"+FileTag(hlSrc)+"]\nreplace 5-5 \"return\":\n+\treturn a - b\n")
	if err == nil || !strings.Contains(err.Error(), "change nothing") {
		t.Fatalf("want no-op refusal, got %v", err)
	}
}

func TestHashlineOutsideProjectRefused(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	_, err := applyHL(t, dir, "[../calc.go#ABCD]\ndelete 1-1 \"pac\"\n")
	if err == nil || !strings.Contains(err.Error(), "outside the project") {
		t.Fatalf("got %v", err)
	}
}

func TestPatchFormatStillAcceptedWithHashlineOn(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	got, err := runPatch(t, dir, "*** Begin Patch\n*** Add File: x.txt\n+hi\n*** End Patch", "x.txt")
	if err != nil || got != "hi\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestApplyPatchDescriptionFollowsEditFormat(t *testing.T) {
	desc := func() string {
		return *BuildToolUnionParams([]ToolDefinition{ApplyPatchDefinition})[0].OfTool.Description
	}
	if strings.Contains(desc(), "[path#TAG]") {
		t.Fatal("patch format described line-anchored edits")
	}
	hashlineOn(t)
	if !strings.Contains(desc(), "[path#TAG]") {
		t.Fatal("hashline on, description unchanged")
	}
}

// Live 2026-09-29 (m03): replace 4-6 with only the new first line would drop
// the body and the brace. The refusal names the short range and the fix; it
// never sends the model to rewrite the whole file.
func TestHashlineShortRangeRefusalNamesTheFix(t *testing.T) {
	hashlineOn(t)
	dir := t.TempDir()
	path, _ := readHL(t, dir, hlSrc)
	_, err := applyHL(t, dir, "[calc.go#"+FileTag(hlSrc)+"]\nreplace 4-6 \"func Add\":\n+func Add(a, b int) int { // sum\n")
	if err == nil {
		t.Fatal("a patch that breaks the parse was applied")
	}
	for _, want := range []string{"replace 4-6 removes 3 lines and gives 1", "use replace 4-4", "4:func Add(a, b int) int {"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "Add File") {
		t.Errorf("refusal sends the model to rewrite the whole file:\n%v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != hlSrc {
		t.Fatal("a refused edit wrote the file")
	}
}
