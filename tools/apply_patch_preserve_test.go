package tools

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A match that only lands through seekSequence's whitespace tiers (the model
// re-typed context with spaces where the file has tabs) must not write the
// model's whitespace over the file's: the file's own lines survive, the real
// change lands.
func TestPreserveContextKeepsFileWhitespace(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"),
		[]byte("func main() {\n\tif x {\n\t\tdoThing()\n\t}\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := beginPatchMarker + "\n" +
		updateFileMarker + " main.go\n" +
		"@@ func main() {\n" +
		"-  if x {\n" +
		"-    doThing()\n" +
		"+  if x {\n" +
		"+    doThing(2)\n" +
		"  }\n" +
		endPatchMarker + "\n"
	applyPatchIn(t, dir, patch)
	body, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(body)
	// Unchanged context lines keep the file's own whitespace; the EDITED line
	// (doThing -> doThing(2)) is the model's and may carry its own indentation.
	for _, want := range []string{"\tif x {", "\t}", "doThing(2)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("file whitespace not preserved; got:\n%s", out)
		}
	}
	if strings.Contains(out, "    if x {") {
		t.Fatalf("model's space indentation leaked into the file:\n%s", out)
	}
}

// The live case (2026-09-30): context re-typed one tab short of the file.
// The context keeps the file's two tabs and the added lines move down the
// same tab, so the new code sits inside the block, not beside it.
func TestPreserveContextShiftsAddsByTheContextOffset(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir)
	src := "package main\n\nfunc main() {\n\tif true {\n\t\tx := 1\n\t\t_ = x\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := beginPatchMarker + "\n" + updateFileMarker + " main.go\n" +
		"@@\n \tx := 1\n \t_ = x\n+\ty := 2\n+\t_ = y\n" + endPatchMarker + "\n"
	applyPatchIn(t, dir, patch)
	body, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	want := "package main\n\nfunc main() {\n\tif true {\n\t\tx := 1\n\t\t_ = x\n\t\ty := 2\n\t\t_ = y\n\t}\n}\n"
	if string(body) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", body, want)
	}
}

// Re-indenting IS an edit: lines the hunk removes and adds again one level
// deeper (wrapping them in an if) keep the new depth.
func TestPreserveContextKeepsAReindent(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir)
	src := "package main\n\nfunc main() {\n\tx := 1\n\tfoo(x)\n\tfoo(x)\n}\n\nfunc foo(int) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := beginPatchMarker + "\n" + updateFileMarker + " main.go\n" +
		"@@\n \tx := 1\n-\tfoo(x)\n-\tfoo(x)\n+\tif x > 0 {\n+\t\tfoo(x)\n+\t\tfoo(x)\n+\t}\n }\n" + endPatchMarker + "\n"
	applyPatchIn(t, dir, patch)
	body, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	want := "package main\n\nfunc main() {\n\tx := 1\n\tif x > 0 {\n\t\tfoo(x)\n\t\tfoo(x)\n\t}\n}\n\nfunc foo(int) {}\n"
	if string(body) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", body, want)
	}
}

// Context off by different amounts gives no single offset: the context is
// still the file's, the added lines stay as written.
func TestPreserveContextMixedOffsetLeavesAdds(t *testing.T) {
	ch := patchChunk{
		oldLines:   []string{"a {", "b"},
		newLines:   []string{"a {", "  new", "b"},
		contextOld: []int{0, -1, 1},
	}
	got := preserveContext([]string{"\ta {", "\t\t\tb"}, ch.newLines, ch)
	want := []string{"\ta {", "  new", "\t\t\tb"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
