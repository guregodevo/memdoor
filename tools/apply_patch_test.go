package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run applies a patch under a temp dir (as the confinement would) and returns the
// resulting content of `file` (or "" if absent).
func runPatch(t *testing.T, dir, patch, file string) (string, error) {
	t.Helper()
	in, _ := json.Marshal(map[string]string{"input": patch, "cwd": dir})
	out, err := ApplyPatch(json.RawMessage(in))
	_ = out
	if err != nil {
		return "", err
	}
	b, rerr := os.ReadFile(filepath.Join(dir, file))
	if rerr != nil {
		return "", nil
	}
	return string(b), nil
}

func TestApplyPatchAddCreatesFile(t *testing.T) {
	dir := t.TempDir()
	patch := `*** Begin Patch
*** Add File: hello.go
+package main
+
+import "fmt"
+
+func main() { fmt.Println("hi") }
*** End Patch`
	got, err := runPatch(t, dir, patch, "hello.go")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n"
	if got != want {
		t.Fatalf("add mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// A small model often writes Add File content RAW (no "+" prefix, after a blank line).
func TestApplyPatchAddRawContent(t *testing.T) {
	dir := t.TempDir()
	patch := "*** Begin Patch\n*** Add File: r.go\n\npackage main\n\nfunc main() {}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "r.go")
	if err != nil {
		t.Fatalf("raw add should parse: %v", err)
	}
	want := "package main\n\nfunc main() {}\n"
	if got != want {
		t.Fatalf("raw-add mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestApplyPatchUpdateEditsFile(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := `*** Begin Patch
*** Update File: m.go
@@ func main() {
 	fmt.Println("hi")
*** End Patch`
	// This hunk only has context (no +/-), so it's a no-op locator; use a real change:
	patch = "*** Begin Patch\n*** Update File: m.go\n@@ func main() {\n-\tfmt.Println(\"hi\")\n+\tfmt.Println(\"hello, world\")\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello, world\")\n}\n"
	if got != want {
		t.Fatalf("update mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// The exact scenario from the flow: fix a missing import via a small patch.
func TestApplyPatchFixImport(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(strings.Repeat(\".\", 10))\n}\n"
	os.WriteFile(filepath.Join(dir, "t.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: t.go\n-import \"fmt\"\n+import (\n+\t\"fmt\"\n+\t\"strings\"\n+)\n*** End Patch"
	got, err := runPatch(t, dir, patch, "t.go")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main() {\n\tfmt.Println(strings.Repeat(\".\", 10))\n}\n"
	if got != want {
		t.Fatalf("fix-import mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// Fuzzy: the model's context line has a trailing space that the file doesn't; the
// progressive normalization still locates it.
func TestApplyPatchFuzzyWhitespace(t *testing.T) {
	dir := t.TempDir()
	orig := "line one\nline two\nline three\n"
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte(orig), 0o644)
	// old line has a trailing space not present in the file.
	patch := "*** Begin Patch\n*** Update File: f.txt\n-line two \n+line 2\n*** End Patch"
	got, err := runPatch(t, dir, patch, "f.txt")
	if err != nil {
		t.Fatalf("apply (fuzzy should still match): %v", err)
	}
	want := "line one\nline 2\nline three\n"
	if got != want {
		t.Fatalf("fuzzy mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// A small model routinely drops the leading space on unchanged context lines (a bare "}").
// The lenient parser must treat those as context, not fail the whole patch.
func TestApplyPatchUnmarkedContextLine(t *testing.T) {
	dir := t.TempDir()
	orig := "func main() {\n\tx := 1\n\tprintln(x)\n}\n"
	os.WriteFile(filepath.Join(dir, "u.go"), []byte(orig), 0o644)
	// The trailing "}" context line has no leading space (bare) — must be tolerated.
	patch := "*** Begin Patch\n*** Update File: u.go\n@@ func main() {\n-\tx := 1\n+\tx := 2\n\tprintln(x)\n}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "u.go")
	if err != nil {
		t.Fatalf("lenient parse should not fail: %v", err)
	}
	want := "func main() {\n\tx := 2\n\tprintln(x)\n}\n"
	if got != want {
		t.Fatalf("unmarked-context mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// The exact dialect a small model emits: every line stamped with "@@", change lines as
// "-@@"/"+@@". The forgiving parser must strip the noise and apply the real change.
func TestApplyPatchAtAtDialect(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {\n    for _, cell := range row {\n        print(cell)\n    }\n}\n"
	os.WriteFile(filepath.Join(dir, "d.go"), []byte(orig), 0o644)
	// OpenClaw design: "@@ func main() {" ANCHORS the hunk (forward-seek), then the
	// change lines carry a stray "@@" stamp (small-model habit) that gets stripped.
	patch := "*** Begin Patch\n*** Update File: d.go\n@@ func main() {\n-@@     for _, cell := range row {\n+@@     for _, c := range row {\n*** End Patch"
	got, err := runPatch(t, dir, patch, "d.go")
	if err != nil {
		t.Fatalf("@@-anchor should parse+apply: %v", err)
	}
	want := "package main\n\nfunc main() {\n    for _, c := range row {\n        print(cell)\n    }\n}\n"
	if got != want {
		t.Fatalf("@@-dialect mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// Multiple SEARCH/REPLACE blocks in one Update apply in order.
func TestApplyPatchSearchReplaceMultiple(t *testing.T) {
	dir := t.TempDir()
	orig := "a := 1\nb := 2\nc := 3\n"
	os.WriteFile(filepath.Join(dir, "m.txt"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: m.txt\n<<<<<<< SEARCH\na := 1\n=======\na := 10\n>>>>>>> REPLACE\n<<<<<<< SEARCH\nc := 3\n=======\nc := 30\n>>>>>>> REPLACE\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.txt")
	if err != nil {
		t.Fatalf("multi search/replace: %v", err)
	}
	want := "a := 10\nb := 2\nc := 30\n"
	if got != want {
		t.Fatalf("multi mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// Fuzzy locate still works under SEARCH/REPLACE: the SEARCH text's indentation
// differs from the file (spaces vs tab), but it's still found.
func TestApplyPatchSearchReplaceFuzzy(t *testing.T) {
	dir := t.TempDir()
	orig := "func main() {\n\treturn x\n}\n"
	os.WriteFile(filepath.Join(dir, "f.go"), []byte(orig), 0o644)
	// SEARCH uses 4 spaces where the file uses a tab.
	patch := "*** Begin Patch\n*** Update File: f.go\n<<<<<<< SEARCH\n    return x\n=======\n\treturn x + 1\n>>>>>>> REPLACE\n*** End Patch"
	got, err := runPatch(t, dir, patch, "f.go")
	if err != nil {
		t.Fatalf("fuzzy search/replace: %v", err)
	}
	want := "func main() {\n\treturn x + 1\n}\n"
	if got != want {
		t.Fatalf("fuzzy s/r mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// Loose markers: a small model mangles the fence punctuation. Still parses.
func TestApplyPatchSearchReplaceLooseMarkers(t *testing.T) {
	dir := t.TempDir()
	orig := "x := 1\n"
	os.WriteFile(filepath.Join(dir, "l.txt"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: l.txt\n<<<< SEARCH\nx := 1\n====\nx := 2\n>>>> REPLACE\n*** End Patch"
	got, err := runPatch(t, dir, patch, "l.txt")
	if err != nil {
		t.Fatalf("loose markers: %v", err)
	}
	if got != "x := 2\n" {
		t.Fatalf("loose-marker mismatch: got=%q", got)
	}
}

// A small model carries its diff habit into SEARCH/REPLACE, prefixing every line with "+".
// The tool strips the uniform marker so the SEARCH still matches the file.
func TestApplyPatchSearchReplacePlusPrefixed(t *testing.T) {
	dir := t.TempDir()
	orig := "func main() {\n        for _, cell := range row {\n            fmt.Print(\".\")\n        }\n}\n"
	os.WriteFile(filepath.Join(dir, "p.go"), []byte(orig), 0o644)
	// Both SEARCH and REPLACE lines are "+"-stamped (the exact live failure).
	patch := "*** Begin Patch\n*** Update File: p.go\n<<<<<<< SEARCH\n+        for _, cell := range row {\n=======\n+        for range row {\n>>>>>>> REPLACE\n*** End Patch"
	got, err := runPatch(t, dir, patch, "p.go")
	if err != nil {
		t.Fatalf("+-prefixed search/replace should apply: %v", err)
	}
	want := "func main() {\n        for range row {\n            fmt.Print(\".\")\n        }\n}\n"
	if got != want {
		t.Fatalf("plus-prefix mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// apply_patch echoes what changed, as it now reads — the changed lines ±3,
// numbered — so the next edit's "-" lines come from the real text. Not the
// whole file (30% of the new content entering a coder conversation), and not
// an added file, which holds exactly what the patch said (2026-09-30).
func TestApplyPatchEchoesTheChangedRegion(t *testing.T) {
	patchVerify = false
	t.Cleanup(func() { patchVerify = true })
	dir := t.TempDir()
	var src strings.Builder
	src.WriteString("package main\n\nfunc main() {\n")
	for i := 4; i <= 39; i++ {
		fmt.Fprintf(&src, "\t_ = %d\n", i)
	}
	src.WriteString("}\n")
	_ = os.WriteFile(filepath.Join(dir, "e.go"), []byte(src.String()), 0o644)
	apply := func(patch string) string {
		in, _ := json.Marshal(map[string]string{"input": patch, "cwd": dir})
		out, err := ApplyPatch(json.RawMessage(in))
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		return out
	}
	out := apply("*** Begin Patch\n*** Update File: e.go\n@@\n-\t_ = 20\n+\t_ = 2000\n*** End Patch")
	for _, want := range []string{"e.go now (40 lines), around the change:", "17:\t_ = 17", "20:\t_ = 2000", "23:\t_ = 23"} {
		if !strings.Contains(out, want) {
			t.Errorf("echo lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "package main") || strings.Contains(out, "_ = 30") {
		t.Errorf("echo carries more than the changed region:\n%s", out)
	}
	if out := apply("*** Begin Patch\n*** Add File: n.go\n+package main\n*** End Patch"); strings.Contains(out, "package main") {
		t.Errorf("an added file was echoed:\n%s", out)
	}
}

// A bare Delete File of an existing file is refused (see
// TestApplyPatchRefusesBareDeleteFile); deleting a file that does NOT exist stays a
// tolerated no-op, so a duplicate/stale delete doesn't fail the whole patch.
func TestApplyPatchDeleteNonexistentTolerated(t *testing.T) {
	dir := t.TempDir()
	patch := "*** Begin Patch\n*** Delete File: gone.go\n*** End Patch"
	if _, err := runPatch(t, dir, patch, "gone.go"); err != nil {
		t.Fatalf("delete of a nonexistent file should be a no-op: %v", err)
	}
}

// A patch missing the Begin/End markers (small-model slip) still parses.
func TestApplyPatchTolerateMissingMarkers(t *testing.T) {
	dir := t.TempDir()
	patch := "*** Add File: a.txt\n+hello\n"
	got, err := runPatch(t, dir, patch, "a.txt")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got != "hello\n" {
		t.Fatalf("got %q", got)
	}
}

// A SEARCH/REPLACE block whose lines legitimately start with "-" (a Markdown
// bullet, a YAML sequence item) must edit in place — not be mistaken for the
// diff-marker habit and stripped, which used to make the SEARCH fail to match
// and could corrupt the written content. Regression for the uniform-"-" bug.
func TestApplyPatchUpdateBulletLineNotStripped(t *testing.T) {
	dir := t.TempDir()
	orig := "# Fruit\n- apples\n- oranges\n- bananas\n"
	os.WriteFile(filepath.Join(dir, "list.md"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: list.md\n<<<<<<< SEARCH\n- oranges\n=======\n- grapes\n>>>>>>> REPLACE\n*** End Patch"
	got, err := runPatch(t, dir, patch, "list.md")
	if err != nil {
		t.Fatalf("bullet edit failed: %v", err)
	}
	want := "# Fruit\n- apples\n- grapes\n- bananas\n"
	if got != want {
		t.Fatalf("bullet edit mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// The diff-marker habit — a small model stamping every SEARCH/REPLACE line with a
// uniform leading "+" — must still apply: the raw block fails to match, so the
// marker is stripped as a fallback and the edit lands. Guards the fallback that
// the bullet fix must not regress.
func TestApplyPatchUpdateStripsDiffHabit(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: m.go\n<<<<<<< SEARCH\n+\tprintln(\"hi\")\n=======\n+\tprintln(\"bye\")\n>>>>>>> REPLACE\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("diff-habit edit failed: %v", err)
	}
	want := "package main\n\nfunc main() {\n\tprintln(\"bye\")\n}\n"
	if got != want {
		t.Fatalf("diff-habit edit mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// normalizePatchInput cases (ported from main's transport fixes) — these complement
// the branch's match-time strip-defer.

// A small model reconstructs a region from memory and DROPS a blank line in the
// context, so the contiguous match fails. The blank-tolerant fallback must absorb
// the extra file blank and still apply the edit. This is the exact stack.go/Peek
// failure from the live coder run (context skips the blank between Push(3) and for).
func TestApplyPatchToleratesDroppedBlankLine(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {\n    a := 1\n    b := 2\n\n    println(a + b)\n}\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	// The patch's context runs "b := 2" straight into "println" — omitting the blank
	// line the real file has between them — while inserting a new statement.
	patch := "*** Begin Patch\n*** Update File: m.go\n@@ func main() {\n" +
		"     a := 1\n     b := 2\n+    c := 3\n     println(a + b)\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("dropped-blank patch should apply via blank tolerance: %v", err)
	}
	if !strings.Contains(got, "c := 3") || !strings.Contains(got, "println(a + b)") {
		t.Fatalf("edit not applied: got=%q", got)
	}
}

// A small model adds a new method by anchoring "@@" on the method's OWN signature
// (which is not in the file yet) with the body as "+" lines. The anchor can't match;
// recover by appending the reconstructed declaration. This is the exact stack.go/Peek
// failure — the model emitted this identical hunk on every retry.
func TestApplyPatchAppendsSelfAnchoredNewDeclaration(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\ntype IntStack []int\n\nfunc (s IntStack) Len() int {\n    return len(s)\n}\n\nfunc main() {\n    _ = IntStack{}\n}\n"
	os.WriteFile(filepath.Join(dir, "stack.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: stack.go\n" +
		"@@ func (s IntStack) Peek() (int, bool) {\n" +
		"+    if len(s) == 0 {\n" +
		"+        return 0, false\n" +
		"+    }\n" +
		"+    return s[len(s)-1], true\n" +
		" }\n*** End Patch"
	got, err := runPatch(t, dir, patch, "stack.go")
	if err != nil {
		t.Fatalf("self-anchored new method should append, not fail: %v", err)
	}
	if !strings.Contains(got, "func (s IntStack) Peek() (int, bool) {") ||
		!strings.Contains(got, "return s[len(s)-1], true") {
		t.Fatalf("Peek method not appended: got=%q", got)
	}
	// The original content must be untouched.
	if !strings.Contains(got, "func (s IntStack) Len() int {") || !strings.Contains(got, "func main() {") {
		t.Fatalf("original content damaged: got=%q", got)
	}
}

// The full real-world failure: a compound patch a small model emits for "add a method AND
// call it" — hunks OUT of file order (main edited before Pop, which is earlier) and
// the new method self-anchored on its own signature. Must apply cleanly: call added
// to main, Peek method appended, and the whole thing compiles-shaped.
func TestApplyPatchCompoundOutOfOrderWithNewMethod(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\ntype IntStack []int\n\n" +
		"func (s *IntStack) Pop() (int, bool) {\n    if len(*s) == 0 {\n        return 0, false\n    }\n    return (*s)[len(*s)-1], true\n}\n\n" +
		"func (s IntStack) Len() int {\n    return len(s)\n}\n\n" +
		"func main() {\n    stack := &IntStack{}\n    stack.Push(1)\n\n    for stack.Len() > 0 {\n        fmt.Println(stack.Pop())\n    }\n}\n"
	os.WriteFile(filepath.Join(dir, "stack.go"), []byte(orig), 0o644)
	// hunk order: main (add call), then Pop (out-of-order no-op context), then the
	// new Peek method self-anchored on its own signature.
	patch := "*** Begin Patch\n*** Update File: stack.go\n" +
		"@@ func main() {\n     stack := &IntStack{}\n     stack.Push(1)\n+    top, ok := stack.Peek()\n+    _ = top\n+    _ = ok\n" +
		"@@ func (s *IntStack) Pop() (int, bool) {\n     if len(*s) == 0 {\n         return 0, false\n     }\n" +
		"@@ func (s IntStack) Peek() (int, bool) {\n+    if len(s) == 0 {\n+        return 0, false\n+    }\n+    return s[len(s)-1], true\n }\n" +
		"*** End Patch"
	got, err := runPatch(t, dir, patch, "stack.go")
	if err != nil {
		t.Fatalf("compound out-of-order patch should apply: %v", err)
	}
	if !strings.Contains(got, "top, ok := stack.Peek()") {
		t.Fatalf("call not inserted into main: got=%q", got)
	}
	if !strings.Contains(got, "func (s IntStack) Peek() (int, bool) {") || !strings.Contains(got, "return s[len(s)-1], true") {
		t.Fatalf("Peek method not appended: got=%q", got)
	}
	if !strings.Contains(got, "func (s *IntStack) Pop()") || !strings.Contains(got, "func (s IntStack) Len()") {
		t.Fatalf("original funcs damaged: got=%q", got)
	}
}

// The model wraps the patch in a JSON tool-call envelope under "arguments"/"input"
// (sometimes nested twice) instead of passing the bare patch. Without unwrapping,
// apply_patch parses the JSON as the patch ("invalid hunk header") and the coder
// loops — this is the exact fibonacci/main.go hang. Note the Add File form has NO
// *** Begin Patch marker, so the old Begin-slice recovery could not catch it.
func TestApplyPatchUnwrapsJSONEnvelope(t *testing.T) {
	patch := "*** Add File: main.go\n+package main\n+\n+import \"fmt\"\n+\n+func main() { fmt.Println(\"hi\") }\n"
	enc, _ := json.Marshal(patch) // properly-escaped JSON string value (quotes + newlines)
	cases := map[string]string{
		"input wrap":          `{"input":` + string(enc) + `}`,
		"arguments wrap":      `{"arguments":{"input":` + string(enc) + `}}`,
		"name+arguments wrap": `{"name":"apply_patch","arguments":{"input":` + string(enc) + `}}`,
		"tool_call tag wrap":  "<tool_call>\n  {\"name\": \"apply_patch\", \"arguments\": {\"input\": " + string(enc) + "}}\n</tool_call>",
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			got, err := runPatch(t, dir, env, "main.go")
			if err != nil {
				t.Fatalf("enveloped patch should unwrap and apply: %v", err)
			}
			if !strings.Contains(got, "package main") || !strings.Contains(got, `fmt.Println("hi")`) {
				t.Fatalf("file not created from envelope: got=%q", got)
			}
		})
	}
}

// A self-anchored new function where the model also puts a no-op "-x/+x" in the body
// (restating a body line as both removed and added). The anchor ("@@ func … {") still
// isn't in the file, and the intent is still "add this function", so it must append —
// the no-op removal must not disqualify the recovery.
func TestApplyPatchAppendsSelfAnchoredWithNoOpRemoval(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc greet(name string) string {\n\treturn \"Hello, \" + name\n}\n\nfunc main() {\n\tfmt.Println(greet(\"Ada\"))\n\tfmt.Println(farewell(\"Ada\"))\n}\n"
	os.WriteFile(filepath.Join(dir, "greet.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: greet.go\n" +
		"@@ func farewell(name string) string {\n" +
		"-    return \"Goodbye, \" + name\n" +
		"+    return \"Goodbye, \" + name\n" +
		" }\n*** End Patch"
	got, err := runPatch(t, dir, patch, "greet.go")
	if err != nil {
		t.Fatalf("self-anchored func with no-op removal should append: %v", err)
	}
	if !strings.Contains(got, "func farewell(name string) string {") || !strings.Contains(got, "return \"Goodbye, \" + name") {
		t.Fatalf("farewell not appended: got=%q", got)
	}
}

// Fixing a build error, the model re-sends the ALREADY-applied hunk (the call)
// bundled with the new one (the function). The call hunk's context no longer matches
// (the file already has the call), but its result is present, so it must be SKIPPED —
// not fail the whole patch and lose the function. This is the exact greet.go/farewell
// second-patch shape.
func TestApplyPatchSkipsAlreadyAppliedHunk(t *testing.T) {
	dir := t.TempDir()
	// The call is already in main (applied by a prior patch); farewell is NOT defined.
	orig := "package main\n\nimport \"fmt\"\n\nfunc greet(name string) string {\n\treturn \"Hello, \" + name\n}\n\nfunc main() {\n\tfmt.Println(greet(\"Ada\"))\n\tfmt.Println(farewell(\"Alice\"))\n}\n"
	os.WriteFile(filepath.Join(dir, "greet.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: greet.go\n" +
		"@@ func main() {\n\tfmt.Println(greet(\"Ada\"))\n+\tfmt.Println(farewell(\"Alice\"))\n }\n" +
		"@@ func farewell(name string) string {\n+    return \"Goodbye, \" + name\n }\n*** End Patch"
	got, err := runPatch(t, dir, patch, "greet.go")
	if err != nil {
		t.Fatalf("patch with an already-applied hunk should still apply the new one: %v", err)
	}
	if !strings.Contains(got, "func farewell(name string) string {") || !strings.Contains(got, "return \"Goodbye, \" + name") {
		t.Fatalf("farewell not appended: got=%q", got)
	}
	// The call must appear exactly once (not duplicated by re-applying the hunk).
	if n := strings.Count(got, "farewell(\"Alice\")"); n != 1 {
		t.Fatalf("call should appear once, got %d:\n%s", n, got)
	}
}

// The model emits a git-style unified-diff header ("@@ -1,1 +1,1 @@") instead of a
// code anchor. The line-number ranges match nothing, so it must be treated as NO
// anchor and the hunk located by its content — not fail "could not find 1,1 @@".
func TestApplyPatchIgnoresGitDiffHeader(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: m.go\n" +
		"@@ -1,1 +1,1 @@\n\tfmt.Println(\"hello\")\n-\tfmt.Println(\"hello\")\n+\tfmt.Println(\"hi\")\n*** End Patch"
	// The above uses the bogus header AND repeats a context line the model often does;
	// simpler real shape: header then remove/add.
	patch = "*** Begin Patch\n*** Update File: m.go\n" +
		"@@ -1,1 +1,1 @@\n-\tfmt.Println(\"hello\")\n+\tfmt.Println(\"hi\")\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("git-diff-header patch should apply by content: %v", err)
	}
	if !strings.Contains(got, `fmt.Println("hi")`) || strings.Contains(got, `Println("hello")`) {
		t.Fatalf("edit not applied: got=%q", got)
	}
}

// isDiffLineHeader must flag git line-number headers but NEVER a real code anchor —
// otherwise a legitimate "@@ }" or "@@ func main() {" would be dropped and the edit
// would misfire. Direct classifier guard.
func TestIsDiffLineHeaderClassifier(t *testing.T) {
	headers := []string{"-1,1 +1,1 @@", "1,1 @@", "-3,7 +3,8", "@@ -10 +10 @@", "1 @@", "+5,2"}
	for _, h := range headers {
		if !isDiffLineHeader(h) {
			t.Errorf("%q should be classified as a git diff header", h)
		}
	}
	anchors := []string{"}", ")", "func main() {", "\tx := 1", "return n", "if n <= 1 {", "case 1:", ""}
	for _, a := range anchors {
		if isDiffLineHeader(a) {
			t.Errorf("%q is a real code anchor, must NOT be flagged as a diff header", a)
		}
	}
}

// isFileLineRef must flag compiler-style "file.go:12" anchors but NEVER real code —
// a flagged anchor is dropped (content match), so a false positive would unanchor a
// legitimate hunk. Direct classifier guard.
func TestIsFileLineRefClassifier(t *testing.T) {
	refs := []string{"tetris.go:10", "pkg/game/board.py:42", "a/b/c.rs:7", "main.go:1"}
	for _, r := range refs {
		if !isFileLineRef(r) {
			t.Errorf("%q should be classified as a file:line reference", r)
		}
	}
	anchors := []string{"case 1:", "loop:", "func main() {", "default:", "\"key.ext\": 5,",
		"m[x.Name]: y", "}", "", "x := time.Second"}
	for _, a := range anchors {
		if isFileLineRef(a) {
			t.Errorf("%q is a real code line, must NOT be flagged as a file:line ref", a)
		}
	}
}

// The live tetris loop: the coder anchors every hunk on "@@ tetris.go:10" (a
// compiler-style file:line ref that matches no code line) and the patch failed
// every retry. The ref must be dropped so the hunk lands by content.
func TestApplyPatchFileLineRefAnchor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tetris.go")
	os.WriteFile(path, []byte("package main\n\nfunc main() {\n\tprintln(\"start\")\n}\n"), 0o644)
	patch := "*** Begin Patch\n*** Update File: " + path + "\n@@ tetris.go:10\n-\tprintln(\"start\")\n+\tprintln(\"tetris\")\n*** End Patch"
	in, _ := json.Marshal(map[string]string{"input": patch})
	if _, err := ApplyPatch(json.RawMessage(in)); err != nil {
		t.Fatalf("file:line anchored patch should apply, got: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `println("tetris")`) {
		t.Fatalf("edit not applied: %q", got)
	}
}

// The exact real-world failure from the TUI logs: the coder's fix arrives as a
// triple-nested tool-call envelope, and inside is a git-style "@@" header plus the
// edit. Locks that the envelope unwrap + git-header handling apply it end to end.
func TestApplyPatchRealWorldNestedEnvelopeGitHeader(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "m.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"old\")\n}\n"), 0o644)
	// Build the inner patch, then wrap it twice under {"arguments":{"input":…}}.
	inner := "*** Begin Patch\n*** Update File: m.go\n@@ -1,1 +1,1 @@\n-\tprintln(\"old\")\n+\tprintln(\"new\")\n*** End Patch"
	enc, _ := json.Marshal(inner)
	once := `{"arguments":{"input":` + string(enc) + `},"name":"apply_patch"}`
	encOnce, _ := json.Marshal(once)
	twice := `{"arguments":{"input":` + string(encOnce) + `},"name":"apply_patch"}`
	got, err := runPatch(t, dir, twice, "m.go")
	if err != nil {
		t.Fatalf("nested-envelope + git-header patch should apply: %v", err)
	}
	if !strings.Contains(got, `println("new")`) || strings.Contains(got, `println("old")`) {
		t.Fatalf("edit not applied: got=%q", got)
	}
}

// The exact NEG-1 failure: add a missing import, but the model anchors on "import ("
// while the file has a single-line `import "fmt"`. Merge the import into the real
// section (single-line → block) instead of failing/looping.
func TestApplyPatchImportAddSingleLine(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(strings.ToUpper(\"hi\"))\n}\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: m.go\n@@ import (\n     \"fmt\"\n+    \"strings\"\n)\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("import-add should merge into the single-line import: %v", err)
	}
	if !strings.Contains(got, "import (") || !strings.Contains(got, "\"strings\"") || !strings.Contains(got, "\"fmt\"") {
		t.Fatalf("import not merged to a block: got=%q", got)
	}
	// The merged file must compile (both imports present, one block).
	if strings.Count(got, "import ") != 1 {
		t.Fatalf("expected exactly one import keyword: got=%q", got)
	}
}

// An import-add where the file ALREADY has a block import — the new spec joins it,
// deduped, no duplicate.
func TestApplyPatchImportAddToBlock(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport (\n\t\"fmt\"\n)\n\nfunc main() { fmt.Println(os.Args) }\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: m.go\n@@ import (\n\t\"fmt\"\n+\t\"os\"\n)\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("import-add to block should apply: %v", err)
	}
	if !strings.Contains(got, "\"os\"") || strings.Count(got, "\"fmt\"") != 1 {
		t.Fatalf("import merge wrong: got=%q", got)
	}
}

// The whole tool-call JSON ended up inside `input`.
func TestApplyPatchDoubleWrappedJSON(t *testing.T) {
	dir := t.TempDir()
	patch := "{\"name\": \"apply_patch\", \"arguments\": {\"input\": \"*** Begin Patch\\n*** Add File: hi.go\\n+package main\\n*** End Patch\"}}"
	got, err := runPatch(t, dir, patch, "hi.go")
	if err != nil {
		t.Fatalf("double-wrapped patch should parse: %v", err)
	}
	if got != "package main\n" {
		t.Fatalf("double-wrap mismatch: got=%q", got)
	}
}

// json.Marshal HTML-escaped the SEARCH markers to <; must decode.
func TestApplyPatchDecodesHTMLUnicodeEscapes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "u.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	bs := string(rune(92))
	lt := bs + "u003c"
	gt := bs + "u003e"
	patch := "*** Begin Patch\n*** Update File: u.go\n" +
		lt + lt + lt + lt + lt + lt + lt + " SEARCH\n" +
		"func main() {}\n=======\nfunc main() { println(1) }\n" +
		gt + gt + gt + gt + gt + gt + gt + " REPLACE\n*** End Patch"
	got, err := runPatch(t, dir, patch, "u.go")
	if err != nil {
		t.Fatalf("html-escaped markers should decode+apply: %v", err)
	}
	if got != "package main\n\nfunc main() { println(1) }\n" {
		t.Fatalf("html-escape mismatch: got=%q", got)
	}
}

// OpenClaw multi-chunk: two "@@" hunks in ONE Update edit two NON-CONTIGUOUS
// locations, each anchored + applied independently. memdoor's old single-chunk
// parser could not do this (it collapsed everything into one contiguous old-block).
func TestApplyPatchUpdateMultiChunk(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc a() int {\n\treturn 1\n}\n\nfunc b() int {\n\treturn 2\n}\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	// Two @@ hunks anchored to func a and func b; each changes its own return line.
	patch := "*** Begin Patch\n*** Update File: m.go\n" +
		"@@ func a() int {\n-\treturn 1\n+\treturn 10\n" +
		"@@ func b() int {\n-\treturn 2\n+\treturn 20\n" +
		"*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("multi-chunk update should apply: %v", err)
	}
	want := "package main\n\nfunc a() int {\n\treturn 10\n}\n\nfunc b() int {\n\treturn 20\n}\n"
	if got != want {
		t.Fatalf("multi-chunk mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// The model stamps the +/- prefix onto the end marker too ("+*** End Patch"); the
// Add File parser must strip the "+" BEFORE checking for "***", or the end marker
// lands in the created file as content (a syntax error). Exact live bug from vibing.
func TestApplyPatchAddPlusPrefixedEndMarker(t *testing.T) {
	dir := t.TempDir()
	patch := "*** Begin Patch\n*** Add File: s.go\n+package main\n+\n+func main() {}\n+*** End Patch"
	got, err := runPatch(t, dir, patch, "s.go")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := "package main\n\nfunc main() {}\n"
	if got != want {
		t.Fatalf("+-prefixed end marker leaked into file:\n got=%q\nwant=%q", got, want)
	}
}

// A leaked ```golang fence tag as the first content line of a new file must be
// stripped so the created file is valid source (was: greet.go started with "golang").
func TestApplyPatchStripsLeakedLangTag(t *testing.T) {
	dir := t.TempDir()
	patch := "*** Begin Patch\n*** Add File: g.go\n+golang\n+package main\n+\n+func main() {}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "g.go")
	if err != nil {
		t.Fatalf("add with lang tag should apply: %v", err)
	}
	if strings.HasPrefix(got, "golang") {
		t.Fatalf("leaked lang tag not stripped: got=%q", got)
	}
	if !strings.HasPrefix(got, "package main") {
		t.Fatalf("file should start with package main: got=%q", got)
	}
}

// The model anchored a "add a function" hunk on a partial "@@ main() {" (no func
// keyword) that already exists in the file, with an over-indented context line, a
// spurious "+}", and a bare unmarked "func greeting()" line — the shape a small model
// reliably emits for "add a function". apply_patch must NOT duplicate main(); it should
// APPEND the new function (absorbing the malformation), yielding compilable code.
func TestApplyPatchMangledAnchorAppendsFunc(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(greeting())\n}\n"
	os.WriteFile(filepath.Join(dir, "ed.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: ed.go\n@@ main() {\n         fmt.Println(greeting())\n+}\n\nfunc greeting() string {\n+    return \"hi\"\n+}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "ed.go")
	if err != nil {
		t.Fatalf("expected the mangled add-func patch to be absorbed, got error: %v", err)
	}
	if strings.Count(got, "main() {") != 1 {
		t.Fatalf("main() must appear exactly once (no duplicate/clone):\n%s", got)
	}
	if !strings.Contains(got, "func greeting() string {") || !strings.Contains(got, `return "hi"`) {
		t.Fatalf("new greeting func was not appended:\n%s", got)
	}
	// It must compile: exactly one main, one greeting, balanced.
	if strings.Count(got, "func greeting") != 1 {
		t.Fatalf("greeting must be added exactly once:\n%s", got)
	}
}

// The same mangled-anchor shape must NOT re-append a function the file already declares
// (dedupe) — that would be a redeclaration. It should fail cleanly instead.
func TestApplyPatchMangledAnchorNoDuplicateFunc(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(greeting())\n}\n\nfunc greeting() string {\n\treturn \"hi\"\n}\n"
	os.WriteFile(filepath.Join(dir, "ed.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: ed.go\n@@ main() {\n         fmt.Println(greeting())\n+}\n\nfunc greeting() string {\n+    return \"bye\"\n+}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "ed.go")
	if err == nil && strings.Count(got, "func greeting") > 1 {
		t.Fatalf("greeting was duplicated (redeclaration):\n%s", got)
	}
}

// The model buried its real change under spurious context: it echoed the body and "}"
// as context, then put the actual "-return w + h / +return w * h" AFTER, so the full
// old-block ("return w+h", "}", "return w+h") isn't in the file. apply_patch must fall
// back to the MINIMAL change — find the removed line, swap in the added — and land it.
func TestApplyPatchContextThenChangeMinimalFallback(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc area(w, h int) int {\n\treturn w + h\n}\n\nfunc main() {\n\tfmt.Println(area(3, 4))\n}\n"
	os.WriteFile(filepath.Join(dir, "area.go"), []byte(orig), 0o644)
	// context (body + "}") first, THEN the -/+ change — the exact malformation seen live.
	patch := "*** Begin Patch\n*** Update File: area.go\n@@ func area(w, h int) int {\n     return w + h\n}\n-    return w + h\n+    return w * h\n*** End Patch"
	got, err := runPatch(t, dir, patch, "area.go")
	if err != nil {
		t.Fatalf("context-then-change hunk should be absorbed, got error: %v", err)
	}
	if !strings.Contains(got, "return w * h") || strings.Contains(got, "return w + h") {
		t.Fatalf("minimal change did not apply:\n%s", got)
	}
	// Structure preserved: one area, one main, no duplicated/garbled lines.
	if strings.Count(got, "func area") != 1 || strings.Count(got, "func main") != 1 {
		t.Fatalf("file structure corrupted:\n%s", got)
	}
}

// A hunk whose only removed line is a bare "}" must NOT minimal-match some arbitrary
// brace elsewhere — that would corrupt the file. It should fail cleanly instead.
func TestApplyPatchMinimalFallbackSkipsTrivialRemoval(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc a() {\n\tx := 1\n}\n\nfunc b() {\n\ty := 2\n}\n"
	os.WriteFile(filepath.Join(dir, "z.go"), []byte(orig), 0o644)
	// nonsense context that won't match, and the only "-" is a bare "}".
	patch := "*** Begin Patch\n*** Update File: z.go\n@@ nonexistent anchor line\n-}\n+} // end\n*** End Patch"
	got, err := runPatch(t, dir, patch, "z.go")
	// Either a clean error, or unchanged — but NEVER a stray "} // end" spliced in.
	if err == nil && strings.Contains(got, "} // end") {
		t.Fatalf("trivial "+"}"+" removal must not minimal-match arbitrarily:\n%s", got)
	}
}

// The model expressed "change return a+b to a*b" as pure context + a bare "+" add:
// context "return a + b", context "}", then "+return a * b". Applied literally that
// splices a stray "return a * b" AFTER the "}" (a non-compiling orphaned statement).
// apply_patch must recognize the added line as the alnum-twin of a context line and
// REPLACE it, yielding a clean single-return function.
func TestApplyPatchContextPlusBareAddIsReplacement(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc multiply(a, b int) int {\n\treturn a + b\n}\n"
	os.WriteFile(filepath.Join(dir, "ops.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: ops.go\n@@ func multiply(a, b int) int {\n         return a + b\n     }\n+    return a * b\n*** End Patch"
	got, err := runPatch(t, dir, patch, "ops.go")
	if err != nil {
		t.Fatalf("context+bare-add should be absorbed as a replacement, got: %v", err)
	}
	if strings.Contains(got, "return a + b") {
		t.Fatalf("original + line should be replaced, not kept:\n%s", got)
	}
	if strings.Count(got, "return a") != 1 || !strings.Contains(got, "return a * b") {
		t.Fatalf("expected exactly one return (the a*b fix):\n%s", got)
	}
	// Must compile: no statement after the closing brace.
	if strings.Contains(got, "}\n    return") || strings.Contains(got, "}\n\treturn") {
		t.Fatalf("stray statement after brace:\n%s", got)
	}
}

// A genuine pure-add (added line NOT an alnum-twin of any context line) must still be
// inserted normally, not swallowed by the near-duplicate path.
func TestApplyPatchGenuineAddStillInserts(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc f() {\n\tx := 1\n}\n"
	os.WriteFile(filepath.Join(dir, "g.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: g.go\n@@ func f() {\n \tx := 1\n+\ty := 2\n*** End Patch"
	got, err := runPatch(t, dir, patch, "g.go")
	if err != nil {
		t.Fatalf("genuine add should apply: %v", err)
	}
	if !strings.Contains(got, "x := 1") || !strings.Contains(got, "y := 2") {
		t.Fatalf("both lines should be present:\n%s", got)
	}
}

// The exact multi-file live failure: the model wrapped the patch in a JSON envelope
// {"name":"apply_patch","arguments":{"input":"*** Update File...\n..."}} with REAL
// newlines (invalid JSON). unwrapPatchEnvelope must repair the control chars, unwrap it,
// and then the context+bare-add gets absorbed as a replacement — a clean w*h fix.
func TestApplyPatchEnvelopeWithRealNewlines(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc multiply(a, b int) int {\n\treturn a + b\n}\n"
	os.WriteFile(filepath.Join(dir, "ops.go"), []byte(orig), 0o644)
	// Envelope with literal newlines inside the "input" string (as the model emitted).
	inner := "*** Update File: ops.go\n@@ func multiply(a, b int) int {\n     return a + b\n}\n+    return a * b\n"
	envelope := `{"name":"apply_patch","arguments":{"input": "` + inner + `"}}`
	got, err := runPatch(t, dir, envelope, "ops.go")
	if err != nil {
		t.Fatalf("envelope-wrapped patch with real newlines should unwrap+apply, got: %v", err)
	}
	if strings.Contains(got, "return a + b") || !strings.Contains(got, "return a * b") {
		t.Fatalf("expected the a*b fix, no leftover a+b:\n%s", got)
	}
	if strings.Count(got, "return a") != 1 {
		t.Fatalf("expected exactly one return (no orphaned duplicate):\n%s", got)
	}
}

// A clean, bare patch (marker at the front, no JSON wrapper) must be untouched by the
// embedded-slice path.
func TestApplyPatchBareUpdateStillWorks(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {\n\tprintln(\"a\")\n}\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: m.go\n@@ func main() {\n-\tprintln(\"a\")\n+\tprintln(\"b\")\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("bare patch should apply: %v", err)
	}
	if !strings.Contains(got, `println("b")`) || strings.Contains(got, `println("a")`) {
		t.Fatalf("bare update mismatch:\n%s", got)
	}
}

// Git-diff column habit (live rect.go corruption): the model wrote its add-lines as
// " +func …" — a SPACE before the "+", so the parser read them as context lines whose
// content begins with "+", and literal "+func…" lines landed in the file. Space-then-plus
// inside an @@ hunk must be an ADD; no "+" may ever appear in the written file.
func TestApplyPatchSpaceBeforePlusIsAdd(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc area(w, h int) int {\n\treturn w * h\n}\n"
	os.WriteFile(filepath.Join(dir, "rect.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: rect.go\n@@ func area(w, h int) int {\n     return w * h\n }\n +func triangle(b, h int) int {\n +    return b * h / 2\n +}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "rect.go")
	if err != nil {
		t.Fatalf("space-before-plus hunk should apply: %v", err)
	}
	for _, ln := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimLeft(ln, " \t"), "+") {
			t.Fatalf("literal '+' marker leaked into the file:\n%s", got)
		}
	}
	if !strings.Contains(got, "func triangle(b, h int) int {") {
		t.Fatalf("triangle func not added:\n%s", got)
	}
	if strings.Count(got, "func area") != 1 {
		t.Fatalf("area duplicated:\n%s", got)
	}
}

// A patch that would turn a PARSEABLE Go file into an unparseable one must be refused
// with the file unchanged — this is the corruption that strands a small model (it cannot write
// the "-"-only deletion hunks needed to repair). The error teaches the rewrite move.
func TestApplyPatchRefusesSyntaxRegression(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {\n\tprintln(1)\n}\n"
	os.WriteFile(filepath.Join(dir, "g.go"), []byte(orig), 0o644)
	// Replaces a good line with mid-file garbage — unparseable and beyond any
	// mechanical repair (not trailing junk, not a misplaced import).
	patch := "*** Begin Patch\n*** Update File: g.go\n@@ func main() {\n-\tprintln(1)\n+\tif { nope((\n*** End Patch"
	_, err := runPatch(t, dir, patch, "g.go")
	if err == nil {
		t.Fatal("syntax-breaking patch should be refused")
	}
	if !strings.Contains(err.Error(), "Add File") {
		t.Fatalf("error should teach the rewrite move, got: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "g.go"))
	if string(b) != orig {
		t.Fatalf("file must be unchanged after refusal:\n%s", b)
	}
}

// An ALREADY-broken Go file accepts patches (repair attempts must not be blocked),
// and an Add File rewrite with clean content recovers it.
func TestApplyPatchAllowsRepairOfBrokenFile(t *testing.T) {
	dir := t.TempDir()
	broken := "package main\n\nfunc main() {\n\tprintln(1)\n}\n+func junk() {\n"
	os.WriteFile(filepath.Join(dir, "b.go"), []byte(broken), 0o644)
	// Rewrite via Add File overwrite — the recovery move.
	patch := "*** Begin Patch\n*** Add File: b.go\n+package main\n+\n+func main() {\n+\tprintln(1)\n+}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "b.go")
	if err != nil {
		t.Fatalf("rewrite of a broken file must be allowed: %v", err)
	}
	if goParseErr("b.go", got) != nil {
		t.Fatalf("recovered file should parse:\n%s", got)
	}
}

// A valid→valid update still applies (the guard is invisible on the happy path), and
// non-Go files are never parse-checked.
func TestApplyPatchGuardHappyPathAndNonGo(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {\n\tprintln(1)\n}\n"
	os.WriteFile(filepath.Join(dir, "h.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: h.go\n-\tprintln(1)\n+\tprintln(2)\n*** End Patch"
	got, err := runPatch(t, dir, patch, "h.go")
	if err != nil || !strings.Contains(got, "println(2)") {
		t.Fatalf("valid update should apply: err=%v got=%s", err, got)
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello\n"), 0o644)
	patch2 := "*** Begin Patch\n*** Update File: notes.txt\n-hello\n+{{{ not go at all\n*** End Patch"
	if _, err := runPatch(t, dir, patch2, "notes.txt"); err != nil {
		t.Fatalf("non-Go file must not be parse-checked: %v", err)
	}
}

// "Delete those two methods" made a small model emit a bare *** Delete File — destroying the
// file it was asked to edit. A bare whole-file delete of an existing file is refused
// with the file untouched and an error teaching the rewrite move; a delete+add pair
// for the same path (a legitimate rewrite) still applies.
func TestApplyPatchRefusesBareDeleteFile(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {}\n"
	os.WriteFile(filepath.Join(dir, "d.go"), []byte(orig), 0o644)
	_, err := runPatch(t, dir, "*** Begin Patch\n*** Delete File: d.go\n*** End Patch", "d.go")
	if err == nil {
		t.Fatal("bare Delete File of an existing file should be refused")
	}
	if !strings.Contains(err.Error(), "Add File") {
		t.Fatalf("error should teach the rewrite move, got: %v", err)
	}
	if b, rerr := os.ReadFile(filepath.Join(dir, "d.go")); rerr != nil || string(b) != orig {
		t.Fatalf("file must be untouched after refusal: %v %q", rerr, b)
	}
	// delete + add same path = rewrite → allowed.
	patch := "*** Begin Patch\n*** Delete File: d.go\n*** Add File: d.go\n+package main\n+\n+func main() { println(9) }\n*** End Patch"
	got, err := runPatch(t, dir, patch, "d.go")
	if err != nil {
		t.Fatalf("delete+add rewrite should apply: %v", err)
	}
	if !strings.Contains(got, "println(9)") {
		t.Fatalf("rewrite content missing:\n%s", got)
	}
}

// An anchored PURE-INSERT hunk (`@@ <line>` + only "+" lines) must insert right AFTER
// the anchor — not at EOF, where a statement lands outside every function and the
// syntax guard refuses it. This blocked the tetris "add a println after the existing
// one" edit four times in a row.
func TestApplyPatchAnchoredInsertGoesAfterAnchor(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Starting\")\n}\n"
	os.WriteFile(filepath.Join(dir, "t.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: t.go\n@@ fmt.Println(\"Starting\")\n+\tfmt.Println(\"Welcome\")\n*** End Patch"
	got, err := runPatch(t, dir, patch, "t.go")
	if err != nil {
		t.Fatalf("anchored insert should land: %v", err)
	}
	want := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Starting\")\n\tfmt.Println(\"Welcome\")\n}\n"
	if got != want {
		t.Fatalf("insert position wrong:\n got=%q\nwant=%q", got, want)
	}
	if goParseErr("t.go", got) != nil {
		t.Fatalf("result must parse:\n%s", got)
	}
}

// A triple-nested envelope can half-decode into "*** Update File: x.go\n@@ …" with a
// LITERAL backslash-n gluing the path and hunk on one line (the T6 ascii_art failure:
// "failed to read file ascii_art.go\n@@ -9,1 +9,2 …"). The header-glued escape must
// trigger a full literal-escape decode so the patch parses and applies.
func TestApplyPatchHeaderGluedLiteralNewlines(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() {\n\tprintln(\"-----\")\n}\n"
	os.WriteFile(filepath.Join(dir, "a.go"), []byte(orig), 0o644)
	patch := "*** Update File: a.go\\n@@ func main() {\\n-\tprintln(\"-----\")\\n+\tprintln(\"/\\\\\")\n"
	got, err := runPatch(t, dir, patch, "a.go")
	if err != nil {
		t.Fatalf("header-glued literal-\\n patch should decode and apply: %v", err)
	}
	if strings.Contains(got, "-----") {
		t.Fatalf("old line should be replaced:\n%s", got)
	}
}

// Updating a guessed, nonexistent filename must list the directory's real files —
// same absorption as read_file's missing-file listing (seen live: patched greet.go
// while the workspace held sun.go).
func TestApplyPatchUpdateMissingListsFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sun.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	patch := "*** Begin Patch\n*** Update File: greet.go\n@@ func main() {\n+\tprintln(1)\n*** End Patch"
	_, err := runPatch(t, dir, patch, "greet.go")
	if err == nil {
		t.Fatal("update of a missing file should error")
	}
	if !strings.Contains(err.Error(), "sun.go") {
		t.Fatalf("error should list the real files, got: %v", err)
	}
}

// The model sometimes DOUBLES the anchor marker ("@@ @@ main() {"): the anchor must
// still resolve to bare code, the insert must land around the context, and no literal
// "@" may reach the file (live: the syntax guard had to refuse the whole edit).
func TestApplyPatchDoubledAnchorMarker(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"~~~~\")\n}\n"
	os.WriteFile(filepath.Join(dir, "wave.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: wave.go\n@@ @@ main() {\n+    log.Println(\"start\")\n     fmt.Println(\"~~~~\")\n+    log.Println(\"end\")\n}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "wave.go")
	if err != nil {
		t.Fatalf("doubled @@ anchor should still apply: %v", err)
	}
	if strings.Contains(got, "@") {
		t.Fatalf("literal @ leaked into the file:\n%s", got)
	}
	if !strings.Contains(got, `log.Println("start")`) || !strings.Contains(got, `log.Println("end")`) {
		t.Fatalf("log lines not inserted:\n%s", got)
	}
	if goParseErr("wave.go", got) != nil {
		t.Fatalf("result must parse:\n%s", got)
	}
}

// A matched hunk that ADDS a complete func whose name the file already declares must
// SUPERSEDE the old one (delete it), not leave two ("half redeclared" — live E5).
func TestApplyPatchAddedDeclSupersedesOld(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc half(n int) int {\n\treturn n * 2\n}\n\nfunc main() {\n\tfmt.Println(half(50))\n}\n"
	os.WriteFile(filepath.Join(dir, "h.go"), []byte(orig), 0o644)
	// Anchored on main, adds a NEW half definition after it (the live shape).
	patch := "*** Begin Patch\n*** Update File: h.go\n@@ func main() {\n \tfmt.Println(half(50))\n }\n+func half(n int) int {\n+\treturn n / 2\n+}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "h.go")
	if err != nil {
		t.Fatalf("supersede patch should apply: %v", err)
	}
	if strings.Count(got, "func half") != 1 {
		t.Fatalf("old half must be superseded, got:\n%s", got)
	}
	if !strings.Contains(got, "return n / 2") || strings.Contains(got, "return n * 2") {
		t.Fatalf("new definition should win:\n%s", got)
	}
	if goParseErr("h.go", got) != nil {
		t.Fatalf("result must parse:\n%s", got)
	}
}

// A hunk that only ECHOES the existing decl as context (not a new definition) must
// not trigger a supersede deletion.
func TestApplyPatchContextEchoNotSuperseded(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc half(n int) int {\n\treturn n * 2\n}\n"
	os.WriteFile(filepath.Join(dir, "e.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: e.go\n@@ func half(n int) int {\n-\treturn n * 2\n+\treturn n / 2\n*** End Patch"
	got, err := runPatch(t, dir, patch, "e.go")
	if err != nil {
		t.Fatalf("plain change should apply: %v", err)
	}
	if strings.Count(got, "func half") != 1 || !strings.Contains(got, "return n / 2") {
		t.Fatalf("simple in-place change broken:\n%s", got)
	}
}

// A context-only (no-op) patch must be REFUSED with a teaching error — "Patch
// applied" on a no-op reads as done and the model stops without making the change
// (live: a context-only hunk left third.go multiplying, model reported success).
func TestApplyPatchRefusesNoOp(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc third(n int) int {\n\treturn n * 3\n}\n"
	os.WriteFile(filepath.Join(dir, "t3.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: t3.go\n@@ func third(n int) int {\n     return n * 3\n}\n*** End Patch"
	_, err := runPatch(t, dir, patch, "t3.go")
	if err == nil {
		t.Fatal("no-op patch should be refused")
	}
	if !strings.Contains(err.Error(), "-old line") {
		t.Fatalf("error should teach the -/+ hunk shape, got: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "t3.go"))
	if string(b) != orig {
		t.Fatalf("file must be unchanged: %q", b)
	}
}

// The live V1 shape: a context-only hunk that ECHOES the function with a RENAMED
// parameter (x for n). It must not re-declare, supersede, or "apply" — it is a
// locator with no changes, and the whole patch gets the no-op refusal.
func TestApplyPatchContextEchoRenamedParamIsNoOp(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc third(n int) int {\n\treturn n * 3\n}\n\nfunc main() {\n\tfmt.Println(third(9))\n}\n"
	os.WriteFile(filepath.Join(dir, "t4.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: t4.go\n@@ func third(x int) int {\n     return x * 3\n}\n*** End Patch"
	_, err := runPatch(t, dir, patch, "t4.go")
	if err == nil {
		t.Fatal("context-echo patch should be refused as a no-op")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "t4.go"))
	if string(b) != orig {
		t.Fatalf("file must be untouched:\n%s", b)
	}
}

// Live misfire: adding func combine (body "a + b") after context echoing blend's body
// ("a - b") triggered the operator-twin replacement — blend's BODY got changed and
// combine was never added. A hunk whose adds contain a DECLARATION is new code: apply
// it as written (blend untouched, combine appended — supersede rules still apply).
func TestApplyPatchAddFuncNotTwinReplaced(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc blend(a, b int) int {\n\treturn a - b\n}\n\nfunc main() {\n\tfmt.Println(blend(10, 4))\n}\n"
	os.WriteFile(filepath.Join(dir, "m.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: m.go\n@@ func blend(a, b int) int {\n     return a - b\n}\n+func combine(a, b int) int {\n+    return a + b\n+}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "m.go")
	if err != nil {
		t.Fatalf("add-func hunk should apply: %v", err)
	}
	if !strings.Contains(got, "return a - b") {
		t.Fatalf("blend's body must be untouched:\n%s", got)
	}
	if !strings.Contains(got, "func combine(a, b int) int {") || !strings.Contains(got, "return a + b") {
		t.Fatalf("combine must be added:\n%s", got)
	}
	if goParseErr("m.go", got) != nil {
		t.Fatalf("result must parse:\n%s", got)
	}
}

// A whole-file rewrite (Add File over an existing file) is refused when the harness
// says the file was NOT read this turn — rewriting from memory drifts details (live:
// argument values changed across a rewrite). Reading first (turn_reads carries the
// name) allows it; absent turn_reads (CLI/tests) means no enforcement.
func TestApplyPatchBlindRewriteGuard(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nfunc main() { println(1) }\n"
	os.WriteFile(filepath.Join(dir, "r.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Add File: r.go\n+package main\n+\n+func main() { println(2) }\n*** End Patch"

	// Enforced + not read → refused, file untouched.
	in, _ := json.Marshal(map[string]any{"input": patch, "cwd": dir, "turn_reads": []string{}})
	if _, err := ApplyPatch(json.RawMessage(in)); err == nil || !strings.Contains(err.Error(), "read_file r.go first") {
		t.Fatalf("blind rewrite should be refused with the read-first teaching, got: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "r.go")); string(b) != orig {
		t.Fatal("file must be untouched after refusal")
	}

	// Read this turn → allowed.
	in2, _ := json.Marshal(map[string]any{"input": patch, "cwd": dir, "turn_reads": []string{"r.go"}})
	if _, err := ApplyPatch(json.RawMessage(in2)); err != nil {
		t.Fatalf("rewrite after reading should apply: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "r.go")); !strings.Contains(string(b), "println(2)") {
		t.Fatal("rewrite content should land")
	}

	// No turn_reads at all (CLI/tests) → no enforcement.
	os.WriteFile(filepath.Join(dir, "r.go"), []byte(orig), 0o644)
	in3, _ := json.Marshal(map[string]string{"input": patch, "cwd": dir})
	if _, err := ApplyPatch(json.RawMessage(in3)); err != nil {
		t.Fatalf("absent turn_reads must not enforce: %v", err)
	}
}

// A hunk adding BOTH a new func and a new main must supersede the OLD main too —
// the first-decl-only break left "main redeclared" live (stats.go).
func TestApplyPatchSupersedesEveryAddedDecl(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"skeleton\")\n}\n"
	os.WriteFile(filepath.Join(dir, "st.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: st.go\n@@ fmt.Println(\"skeleton\")\n+func sum() int {\n+\treturn 10\n+}\n+\n+func main() {\n+\tfmt.Println(sum())\n+}\n*** End Patch"
	got, err := runPatch(t, dir, patch, "st.go")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if strings.Count(got, "func main") != 1 {
		t.Fatalf("old main must be superseded, got:\n%s", got)
	}
	if !strings.Contains(got, "func sum") || !strings.Contains(got, "fmt.Println(sum())") {
		t.Fatalf("new code missing:\n%s", got)
	}
	if goParseErr("st.go", got) != nil {
		t.Fatalf("must parse:\n%s", got)
	}
}

// Live stars.go shape: BOTH markers space-indented ("    -    fmt…", "    +    fmt…").
// With indented "+" present, indented "-" is a removal; the hunk applies as a diff.
// A lone " - item" (no indented "+") stays context — YAML/markdown safety.
func TestApplyPatchIndentedMinusWithPlus(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"*\")\n\tfmt.Println(\"***\")\n}\n"
	os.WriteFile(filepath.Join(dir, "st.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: st.go\n@@ func main() {\n     fmt.Println(\"*\")\n    -    fmt.Println(\"***\")\n    +    fmt.Println(\"**\")\n    +    fmt.Println(\"***\")\n*** End Patch"
	got, err := runPatch(t, dir, patch, "st.go")
	if err != nil {
		t.Fatalf("indented-marker diff should apply: %v", err)
	}
	want := []string{`fmt.Println("*")`, `fmt.Println("**")`, `fmt.Println("***")`}
	pos := -1
	for _, w := range want {
		p := strings.Index(got, w)
		if p < 0 || p < pos {
			t.Fatalf("lines missing or out of order (%s):\n%s", w, got)
		}
		pos = p
	}
	if strings.Count(got, `("***")`) != 1 {
		t.Fatalf("*** line should appear once:\n%s", got)
	}

	// YAML safety: indented "-" with NO indented "+" in the hunk stays content.
	orig2 := "items:\n  - alpha\n  - beta\n"
	os.WriteFile(filepath.Join(dir, "y.yaml"), []byte(orig2), 0o644)
	patch2 := "*** Begin Patch\n*** Update File: y.yaml\n@@ items:\n  - alpha\n+  - gamma\n*** End Patch"
	got2, err := runPatch(t, dir, patch2, "y.yaml")
	if err != nil {
		t.Fatalf("yaml add should apply: %v", err)
	}
	if !strings.Contains(got2, "- alpha") || !strings.Contains(got2, "- gamma") {
		t.Fatalf("yaml list items mangled:\n%s", got2)
	}
}

// The model's #1 recurring hallucination: imports that cannot exist — placeholder
// paths ("your-project/sharedctx") or undeclared deps (testify). Ground-truth
// verification: stdlib (GOROOT/src), this module's own packages (go.mod name),
// or declared requires. The refusal teaches the REAL module name.
func TestApplyPatchRefusesHallucinatedImports(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module realmod\n\ngo 1.22\n\nrequire (\n\tgithub.com/gorilla/websocket v1.5.0 // indirect\n)\n"), 0o644)

	// Placeholder path + undeclared dep → refused, module name taught.
	bad := "*** Begin Patch\n*** Add File: t.go\n+package main\n+\n+import (\n+\t\"your-project/sharedctx\"\n+\t\"github.com/stretchr/testify/assert\"\n+)\n+\n+func main() { _ = sharedctx.X; _ = assert.Y }\n*** End Patch"
	_, err := runPatch(t, dir, bad, "t.go")
	if err == nil {
		t.Fatal("hallucinated imports should be refused")
	}
	if !strings.Contains(err.Error(), "realmod") || !strings.Contains(err.Error(), "your-project/sharedctx") {
		t.Fatalf("refusal should name the bad import and teach the real module, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "t.go")); statErr == nil {
		t.Fatal("file must not be written")
	}

	// stdlib + module-local + declared dep → all fine (the local pkg must EXIST).
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "sub.go"), []byte("package sub\n\nvar X = 1\n"), 0o644)
	good := "*** Begin Patch\n*** Add File: ok.go\n+package main\n+\n+import (\n+\t\"fmt\"\n+\t\"go/parser\"\n+\t\"realmod/sub\"\n+\t\"github.com/gorilla/websocket\"\n+)\n+\n+func main() { fmt.Println(parser.Mode(0), sub.X, websocket.TextMessage) }\n*** End Patch"
	if _, err := runPatch(t, dir, good, "ok.go"); err != nil {
		t.Fatalf("resolvable imports should pass: %v", err)
	}

	// No go.mod: stdlib fine, anything else refused.
	dir2 := t.TempDir()
	solo := "*** Begin Patch\n*** Add File: s.go\n+package main\n+\n+import \"github.com/stretchr/testify/assert\"\n+\n+func main() { _ = assert.Y }\n*** End Patch"
	if _, err := runPatch(t, dir2, solo, "s.go"); err == nil {
		t.Fatal("third-party import without go.mod should be refused")
	}

	// Hallucinated module-LOCAL subpackage (prefix matches, dir doesn't exist) →
	// refused, and the teaching lists the packages that DO exist.
	ghost := "*** Begin Patch\n*** Add File: g.go\n+package main\n+\n+import \"realmod/assert\"\n+\n+func main() { _ = assert.Y }\n*** End Patch"
	_, err = runPatch(t, dir, ghost, "g.go")
	if err == nil {
		t.Fatal("nonexistent local subpackage should be refused")
	}
	if !strings.Contains(err.Error(), "realmod/sub") {
		t.Fatalf("teaching should list the real packages, got: %v", err)
	}
}

// TestApplyPatchAbsorbsModulePathEcho: live failure — inside module "demo" the
// A small model targeted "demo/mathx/add_test.go", creating a spurious nested demo/ dir
// whose package (demo/demo/mathx) go can't resolve. The redundant module-name
// segment is stripped so the file lands in the real package dir.
func TestApplyPatchAbsorbsModulePathEcho(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "mathx"), 0o755)
	os.WriteFile(filepath.Join(dir, "mathx", "mathx.go"), []byte("package mathx\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644)

	patch := "*** Begin Patch\n*** Add File: demo/mathx/add_test.go\n+package mathx\n+\n+import \"testing\"\n+\n+func TestAdd(t *testing.T) {\n+\tif Add(2, 3) != 5 {\n+\t\tt.Fatal(\"nope\")\n+\t}\n+}\n*** End Patch"
	if _, err := runPatch(t, dir, patch, "demo/mathx/add_test.go"); err != nil {
		t.Fatalf("echoed-module-path add should absorb, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mathx", "add_test.go")); err != nil {
		t.Fatal("file should land in the REAL package dir mathx/")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo")); err == nil {
		t.Fatal("no spurious nested demo/ dir should be created")
	}

	// A project that really HAS a dir named like the module is left alone.
	os.MkdirAll(filepath.Join(dir, "demo"), 0o755)
	os.WriteFile(filepath.Join(dir, "demo", "d.go"), []byte("package demo\n\nvar D = 1\n"), 0o644)
	patch2 := "*** Begin Patch\n*** Add File: demo/e.go\n+package demo\n+\n+var E = 2\n*** End Patch"
	if _, err := runPatch(t, dir, patch2, "demo/e.go"); err != nil {
		t.Fatalf("legit demo/ dir add failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo", "e.go")); err != nil {
		t.Fatal("legit demo/ dir must NOT be stripped")
	}
}

// TestApplyPatchRefusalQuotesOffendingLine: "main.go:25:90" is useless to a
// model that cannot count lines in its own patch — the refusal must QUOTE the
// broken line text.
func TestApplyPatchRefusalQuotesOffendingLine(t *testing.T) {
	dir := t.TempDir()
	patch := "*** Begin Patch\n*** Add File: bad.go\n" +
		"+package main\n+\n+func main() {\n+\tx := Value: addFlag},\n+}\n*** End Patch"
	_, err := runPatch(t, dir, patch, "bad.go")
	if err == nil {
		t.Fatal("broken content should refuse")
	}
	if !strings.Contains(err.Error(), "Value: addFlag") {
		t.Fatalf("refusal should quote the offending line, got: %v", err)
	}
}

// TestApplyPatchAtomicMultiFile: live failure — a two-file patch wrote
// store/store.go, then main.go's hunk was refused (bad import); the error said
// "NOT written" while half the patch HAD landed, cornering the model (its own
// file then tripped the overwrite guard). A patch with ANY invalid add hunk
// must write NOTHING. And a package added by the same patch counts as existing
// for its sibling's import check.
func TestApplyPatchAtomicMultiFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module todoapp\n\ngo 1.22\n"), 0o644)

	// Hunk 1 valid, hunk 2 has a hallucinated import → NOTHING written.
	bad := "*** Begin Patch\n*** Add File: store/store.go\n+package store\n+\n+var X = 1\n" +
		"*** Add File: main.go\n+package main\n+\n+import \"github.com/ghost/pkg\"\n+\n+func main() { _ = pkg.Y }\n*** End Patch"
	if _, err := runPatch(t, dir, bad, "main.go"); err == nil {
		t.Fatal("patch with an invalid hunk should refuse")
	}
	if _, err := os.Stat(filepath.Join(dir, "store", "store.go")); err == nil {
		t.Fatal("atomicity violated: sibling hunk was written despite the refusal")
	}

	// Same shape but main.go imports the package the SAME patch creates → both write.
	good := "*** Begin Patch\n*** Add File: store/store.go\n+package store\n+\n+var X = 1\n" +
		"*** Add File: main.go\n+package main\n+\n+import (\n+\t\"fmt\"\n+\t\"todoapp/store\"\n+)\n+\n+func main() { fmt.Println(store.X) }\n*** End Patch"
	if _, err := runPatch(t, dir, good, "main.go"); err != nil {
		t.Fatalf("same-patch package import should pass: %v", err)
	}
	for _, f := range []string{"store/store.go", "main.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s not written", f)
		}
	}
}

// TestApplyPatchTeachesLocalPackageSpelling: a bare single-segment import
// ("store") is the model reaching for a project-local package — the refusal
// must teach the exact import path AND the create-in-the-same-patch move
// (live: without this, the model flailed at go.mod then shipped a hello-world).
func TestApplyPatchTeachesLocalPackageSpelling(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module todoapp\n\ngo 1.22\n"), 0o644)
	patch := "*** Begin Patch\n*** Add File: main.go\n+package main\n+\n+import \"store\"\n+\n+func main() { _ = store.X }\n*** End Patch"
	_, err := runPatch(t, dir, patch, "main.go")
	if err == nil {
		t.Fatal("bare local import should refuse")
	}
	for _, want := range []string{`"todoapp/store"`, "SAME patch", "store/store.go"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("teach missing %q, got: %v", want, err)
		}
	}
}

// TestApplyPatchContextRewrite: live failure — the model "patched" Mean by
// showing the CORRECTED function as bare context lines (no +/- at all) and got
// three identical "changed NOTHING" refusals before giving up. A context-only
// hunk that almost-matches one region is an implicit replacement: matching
// lines anchor it, differing lines are the edit.
func TestApplyPatchContextRewrite(t *testing.T) {
	dir := t.TempDir()
	orig := "package stats\n\nfunc Mean(xs []float64) float64 {\n\tif len(xs) == 0 {\n\t\treturn 0\n\t}\n\tsum := 0.0\n\tfor _, x := range xs {\n\t\tsum += x\n\t}\n\treturn sum / float64(len(xs)+1)\n}\n"
	os.WriteFile(filepath.Join(dir, "stats.go"), []byte(orig), 0o644)

	// The corrected body as pure context — exactly what the model emitted live.
	patch := "*** Begin Patch\n*** Update File: stats.go\n@@ func Mean(xs []float64) float64 {\n" +
		" \tif len(xs) == 0 {\n \t\treturn 0\n \t}\n \tsum := 0.0\n \tfor _, x := range xs {\n \t\tsum += x\n \t}\n \treturn sum / float64(len(xs))\n }\n*** End Patch"
	if _, err := runPatch(t, dir, patch, "stats.go"); err != nil {
		t.Fatalf("context-rewrite should absorb: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "stats.go"))
	if !strings.Contains(string(got), "float64(len(xs))\n}") || strings.Contains(string(got), "len(xs)+1") {
		t.Fatalf("bug not fixed via context rewrite:\n%s", got)
	}

	// A context-only hunk IDENTICAL to the file is still a no-op → refused.
	noop := "*** Begin Patch\n*** Update File: stats.go\n@@ func Mean(xs []float64) float64 {\n \tif len(xs) == 0 {\n \t\treturn 0\n \t}\n*** End Patch"
	if _, err := runPatch(t, dir, noop, "stats.go"); err == nil {
		t.Fatal("truly-unchanged context hunk should still refuse")
	}
}

// TestApplyPatchContextRewriteEnvelopeTail: the VERBATIM live input that beat
// the first context-rewrite: a double-wrapped envelope whose tail glues JSON
// junk onto the last context line ('}"}},"name":"apply_patch'). The junk line
// "almost matched" the Max region, got spliced in, failed the parse-gate, and
// the WHOLE absorption (including the good Mean fix) was discarded. Junk-aware
// matching keeps the file's own lines for everything that matches.
func TestApplyPatchContextRewriteEnvelopeTail(t *testing.T) {
	dir := t.TempDir()
	orig := "package stats\n\n// Mean returns the arithmetic mean of xs.\nfunc Mean(xs []float64) float64 {\n\tif len(xs) == 0 {\n\t\treturn 0\n\t}\n\tsum := 0.0\n\tfor _, x := range xs {\n\t\tsum += x\n\t}\n\treturn sum / float64(len(xs)+1)\n}\n\n// Max returns the largest value in xs.\nfunc Max(xs []float64) float64 {\n\tif len(xs) == 0 {\n\t\treturn 0\n\t}\n\tm := xs[0]\n\tfor _, x := range xs {\n\t\tif x < m {\n\t\t\tm = x\n\t\t}\n\t}\n\treturn m\n}\n"
	os.MkdirAll(filepath.Join(dir, "stats"), 0o755)
	os.WriteFile(filepath.Join(dir, "stats", "stats.go"), []byte(orig), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixit\n\ngo 1.22\n"), 0o644)

	liveInput := "{\"arguments\":{\"input\":\"*** Update File: stats/stats.go\n@@ func Mean(xs []float64) float64 {\n     if len(xs) == 0 {\n         return 0\n     }\n     sum := 0.0\n     for _, x := range xs {\n         sum += x\n     }\n     return sum / float64(len(xs))\n}\n@@ func Max(xs []float64) float64 {\n     if len(xs) == 0 {\n         return 0\n     }\n     m := xs[0]\n     for _, x := range xs {\n         if x \\u003c m {\n             m = x\n         }\n     }\n     return m\n}\"}},\"name\":\"apply_patch\"}"
	payload, _ := json.Marshal(map[string]string{"input": liveInput, "cwd": dir})
	if _, err := ApplyPatch(json.RawMessage(payload)); err != nil {
		t.Fatalf("verbatim live input should absorb: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "stats", "stats.go"))
	s := string(got)
	if strings.Contains(s, "len(xs)+1") {
		t.Fatalf("Mean bug not fixed:\n%s", s)
	}
	if strings.Contains(s, "apply_patch") || strings.Contains(s, "\"}}") {
		t.Fatalf("envelope junk leaked into file:\n%s", s)
	}
	if goParseErr("stats.go", s) != nil {
		t.Fatalf("result does not parse:\n%s", s)
	}
}

// TestApplyPatchAnchoredDeclAddRoutesToEOF: live failure (tetris turn 2) — a
// context+add hunk anchored "@@ BoardWidth = 10 / BoardHeight = 20" (INSIDE
// the const block) with added lines declaring const/type/func. The decls were
// spliced into the const group ("expected 'IDENT', found 'const'") and three
// retries died identically. Declarations in a no-removal hunk route to EOF no
// matter where the context anchors.
func TestApplyPatchAnchoredDeclAddRoutesToEOF(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\nconst (\n\tBoardWidth  = 10\n\tBoardHeight = 20\n)\n\ntype Board [BoardHeight][BoardWidth]bool\n"
	os.WriteFile(filepath.Join(dir, "tetris.go"), []byte(orig), 0o644)

	patch := "*** Begin Patch\n*** Update File: tetris.go\n@@ BoardWidth  = 10\n     BoardHeight = 20\n" +
		"+const PieceSize = 3\n+\n+type Piece struct {\n+\tShape [PieceSize][PieceSize]bool\n+}\n*** End Patch"
	if _, err := runPatch(t, dir, patch, "tetris.go"); err != nil {
		t.Fatalf("anchored decl-add should route to EOF: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "tetris.go"))
	s := string(got)
	if goParseErr("tetris.go", s) != nil {
		t.Fatalf("result does not parse:\n%s", s)
	}
	if !strings.Contains(s, "type Piece struct") {
		t.Fatalf("decl not added:\n%s", s)
	}
	if strings.Index(s, "type Piece") < strings.Index(s, ")") {
		t.Fatalf("decl landed inside the const block:\n%s", s)
	}
}

// TestApplyPatchProtectsGoMod: live failure — steered at go.mod, the model
// wrote a placeholder require plus envelope junk (')","name":"apply_patch')
// TWICE, and every subsequent go command died on the corrupt file. Junk lines
// are sheared universally; what remains must satisfy go's OWN modfile parser.
func TestApplyPatchProtectsGoMod(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tetris\n\ngo 1.22\n"), 0o644)

	// The live corruption shape: junk glued after a require block.
	patch := "*** Begin Patch\n*** Add File: go.mod\n+module tetris\n+\n+go 1.22\n+\n+require (\n+    github.com/example/package v1.0.0\n+)\",\"name\":\"apply_patch\n*** End Patch"
	// blind-rewrite guard needs a read first — simulate read via turn_reads
	payload, _ := json.Marshal(map[string]interface{}{"input": patch, "cwd": dir, "turn_reads": []string{"go.mod"}})
	_, err := ApplyPatch(json.RawMessage(payload))
	if err != nil {
		// refusal is acceptable IF go.mod is untouched
		b, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
		if !strings.Contains(string(b), "module tetris") || strings.Contains(string(b), "apply_patch") {
			t.Fatalf("go.mod corrupted after refusal:\n%s", b)
		}
		return
	}
	// applied: junk must be sheared and the file must parse as a modfile
	b, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	if strings.Contains(string(b), "apply_patch") {
		t.Fatalf("junk leaked into go.mod:\n%s", b)
	}

	// Outright garbage go.mod content → refused, file untouched.
	garbage := "*** Begin Patch\n*** Add File: go.mod\n+module tetris\n+\n+go 1.22\n+\n+require (((\n*** End Patch"
	payload2, _ := json.Marshal(map[string]interface{}{"input": garbage, "cwd": dir, "turn_reads": []string{"go.mod"}})
	if _, err := ApplyPatch(json.RawMessage(payload2)); err == nil {
		t.Fatal("unparseable go.mod should refuse")
	}
	b2, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	if strings.Contains(string(b2), "((") {
		t.Fatal("garbage reached go.mod")
	}
}

// TestApplyPatchStripsUnifiedDiffCoords: live failure — the model emitted a
// git-style hunk header "@@ -10,7 +10,8 @@ type Piece struct {" and the
// coordinates were parsed as CONTENT ("expected declaration, found '-'").
// Coordinates strip; the trailing context anchors as usual.
func TestApplyPatchStripsUnifiedDiffCoords(t *testing.T) {
	dir := t.TempDir()
	orig := "package main\n\ntype Piece struct {\n\tShape [][]bool\n}\n\nfunc main() {}\n"
	os.WriteFile(filepath.Join(dir, "p.go"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: p.go\n@@ -3,3 +3,4 @@ type Piece struct {\n \tShape [][]bool\n+\tX int\n }\n*** End Patch"
	if _, err := runPatch(t, dir, patch, "p.go"); err != nil {
		t.Fatalf("unified-coords hunk should absorb: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "p.go"))
	s := string(got)
	if !strings.Contains(s, "X int") {
		t.Fatalf("field not added:\n%s", s)
	}
	if goParseErr("p.go", s) != nil || strings.Contains(s, "10,7") {
		t.Fatalf("coords leaked or broke the file:\n%s", s)
	}
}

// Live (qant, 2026-10-10): a patch that re-indents one Python line — "-" the
// eight-space line, "+" the four-space one — was refused three times as
// "changed NOTHING" and apply_patch locked out; a sed with the same edit
// worked. Whitespace is the change in Python, YAML and make: an explicit
// -/+ pair that differs only in indentation is an edit, not a no-op.
func TestApplyPatchAppliesAnIndentationOnlyChange(t *testing.T) {
	dir := t.TempDir()
	orig := "def main():\n    x = 1\n        print(x)\n"
	os.WriteFile(filepath.Join(dir, "goal.py"), []byte(orig), 0o644)
	patch := "*** Begin Patch\n*** Update File: goal.py\n@@ def main():\n     x = 1\n-        print(x)\n+    print(x)\n*** End Patch"
	if _, err := runPatch(t, dir, patch, "goal.py"); err != nil {
		t.Fatalf("an indentation-only -/+ hunk must apply: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "goal.py"))
	if string(b) != "def main():\n    x = 1\n    print(x)\n" {
		t.Fatalf("file after patch:\n%s", b)
	}
}
