package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func goModDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["go.mod"] = "module m\n\ngo 1.25\n"
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readIn(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const atomicA = "package m\n\nimport \"fmt\"\n\nfunc A() { fmt.Println(\"a\") }\n"

// A patch is all or nothing. Live 2026-09-28: a three-section patch to one
// file wrote its first sections, a later section was refused, and the error
// said "the file was NOT written" — the file had doubled. The model patched
// a file whose state it no longer knew, and mangled its imports.
func TestAFailedSectionLeavesEveryFileUntouched(t *testing.T) {
	dir := goModDir(t, map[string]string{"a.go": atomicA, "b.go": "package m\n\nimport nope \"example.com/nope\"\n\nvar _ = nope.Y\n\nfunc B() {}\n"})
	patch := "*** Begin Patch\n" +
		"*** Update File: a.go\n@@\n func A() { fmt.Println(\"a\") }\n+\n+func A2() {}\n" +
		"*** Update File: b.go\n@@\n func B() {}\n+\n+func B2() { nope.X() }\n" +
		"*** End Patch\n"
	if _, err := runPatch(t, dir, patch, "a.go"); err == nil {
		t.Fatal("the second section cannot apply: the patch must fail")
	}
	if got := readIn(t, dir, "a.go"); got != atomicA {
		t.Fatalf("a failed patch wrote its first section:\n%s", got)
	}
}

// The live patch itself, against the file it was written for.
func TestTheLiveThreeSectionPatchIsAllOrNothing(t *testing.T) {
	orig, err := os.ReadFile("testdata/apply_patch_live_0928/tui_remote.go.orig")
	if err != nil {
		t.Fatal(err)
	}
	patch, err := os.ReadFile("testdata/apply_patch_live_0928/patch.txt")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "cmd/cli/cmd/tui_remote.go")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0o644)
	_ = os.WriteFile(p, orig, 0o644)
	in, _ := json.Marshal(map[string]string{"input": string(patch), "cwd": dir})
	_, perr := ApplyPatch(in)
	after, _ := os.ReadFile(p)
	if perr != nil && string(after) != string(orig) {
		t.Fatalf("the patch failed (%v) yet changed the file: %d -> %d bytes", perr, len(orig), len(after))
	}
}

// By default a patch is written as sent, whatever the language: no import is
// added or pruned, nothing is reformatted. Live 2026-09-28 the rewriters built
// for small models added a second import block and deleted a used import
// three patches running, and the coder patched a file it did not know.
func TestAPatchIsWrittenExactlyAsSent(t *testing.T) {
	t.Setenv("MEMDOOR_PATCH_REWRITE", "")
	dir := goModDir(t, map[string]string{"a.go": "package m\n\nimport (\n\t\"fmt\"\n)\n\nfunc A() { fmt.Println(\"a\") }\n"})
	patch := "*** Begin Patch\n*** Update File: a.go\n@@\n import (\n \t\"fmt\"\n+\t\"sort\"\n )\n@@\n func A() { fmt.Println(\"a\") }\n+\n+func Up(s string) string { return strings.ToUpper(s) }\n*** End Patch\n"
	got, err := runPatch(t, dir, patch, "a.go")
	if err != nil {
		t.Fatal(err)
	}
	want := "package m\n\nimport (\n\t\"fmt\"\n\t\"sort\"\n)\n\nfunc A() { fmt.Println(\"a\") }\n\nfunc Up(s string) string { return strings.ToUpper(s) }\n"
	if got != want {
		t.Fatalf("the patch was rewritten (an unused import pruned, a missing one added?):\n%s", got)
	}
}

// A refused multi-file patch names every file it did not write: live
// 2026-09-28 a tui.go + test patch was refused for the test alone, the error
// said "the file was NOT modified", and the model took tui.go as written.
func TestARefusedPatchNamesEveryFile(t *testing.T) {
	dir := goModDir(t, map[string]string{"a.go": atomicA, "b.go": "package m\n\nfunc B() {}\n"})
	patch := "*** Begin Patch\n" +
		"*** Update File: a.go\n@@\n func A() { fmt.Println(\"a\") }\n+\n+func A2() {}\n" +
		"*** Update File: b.go\n@@\n func B() {}\n+\n+func B2() { if x := T{1}.f(); x { }\n" +
		"*** End Patch\n"
	_, err := runPatch(t, dir, patch, "a.go")
	if err == nil || !strings.Contains(err.Error(), "Nothing in this patch was written — not a.go, b.go") {
		t.Fatalf("the error must name both files: %v", err)
	}
	if got := readIn(t, dir, "a.go"); got != atomicA {
		t.Fatalf("a.go was written:\n%s", got)
	}
	one := "*** Begin Patch\n*** Update File: b.go\n@@\n func B() {}\n+func B2() { if x := T{1}.f(); x { }\n*** End Patch\n"
	if _, err := runPatch(t, dir, one, "b.go"); err == nil || strings.Contains(err.Error(), "Nothing in this patch") {
		t.Fatalf("a one-file refusal keeps its own message: %v", err)
	}
}
