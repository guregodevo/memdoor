package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func applyPatchIn(t *testing.T, dir, patch string) string {
	t.Helper()
	in, _ := json.Marshal(map[string]string{"input": patch, "cwd": dir})
	out, err := ApplyPatch(in)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	return out
}

func writeGoMod(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module scratch\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPatchVerifyPassInModule(t *testing.T) {
	if goRootDir() == "" {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	writeGoMod(t, dir)
	out := applyPatchIn(t, dir, `*** Begin Patch
*** Add File: main.go
+package main
+
+func main() { println("ok") }
*** End Patch`)
	if !strings.Contains(out, "verify: PASS") {
		t.Fatalf("want verify: PASS in result, got:\n%s", out)
	}
	if strings.Contains(out, "Next: run the build") {
		t.Fatalf("instructional fallback should be replaced by the verdict, got:\n%s", out)
	}
	// -o kept the binary out of the module dir.
	if _, err := os.Stat(filepath.Join(dir, "scratch")); err == nil {
		t.Fatal("go build wrote a binary into the module dir")
	}
}

// A library-only module has no main package, and `go build -o dir ./...`
// refuses it outright ("no main packages to build"). That is not a compile
// error: the build is retried plain, and a clean package reads PASS. Live
// 2026-10-03: a rate-limiter package was told "the code does not compile"
// after every patch.
func TestPatchVerifyPassInLibraryModule(t *testing.T) {
	if goRootDir() == "" {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	writeGoMod(t, dir)
	out := applyPatchIn(t, dir, `*** Begin Patch
*** Add File: lib/lib.go
+package lib
+
+func Add(a, b int) int { return a + b }
*** End Patch`)
	if !strings.Contains(out, "verify: PASS") || strings.Contains(out, "no main packages") {
		t.Fatalf("a clean library-only module must read PASS, got:\n%s", out)
	}
}

func TestPatchVerifyFailInModule(t *testing.T) {
	if goRootDir() == "" {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	writeGoMod(t, dir)
	out := applyPatchIn(t, dir, `*** Begin Patch
*** Add File: main.go
+package main
+
+func main() { undefinedThing() }
*** End Patch`)
	if !strings.Contains(out, "verify: FAIL") || !strings.Contains(out, "undefinedThing") {
		t.Fatalf("want verify: FAIL with compiler output, got:\n%s", out)
	}
	// No rollback: the patch stays applied so the fix loop can continue.
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Fatal("failing patch was rolled back; it must stay applied")
	}
}

func TestPatchVerifyLooseFileIgnoresStaleSiblings(t *testing.T) {
	if goRootDir() == "" {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	// Stale leftover with its own func main — a whole-dir build would collide.
	if err := os.WriteFile(filepath.Join(dir, "stale.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := applyPatchIn(t, dir, `*** Begin Patch
*** Add File: hello.go
+package main
+
+func main() { println("hello") }
*** End Patch`)
	if !strings.Contains(out, "verify: PASS") {
		t.Fatalf("loose-file verify must compile only the touched file, got:\n%s", out)
	}
}

func TestPatchVerifyOptOut(t *testing.T) {
	patchVerify = false
	t.Cleanup(func() { patchVerify = true })
	dir := t.TempDir()
	writeGoMod(t, dir)
	out := applyPatchIn(t, dir, `*** Begin Patch
*** Add File: main.go
+package main
+
+func main() {}
*** End Patch`)
	if strings.Contains(out, "verify:") {
		t.Fatalf("opt-out must skip verification, got:\n%s", out)
	}
	if !strings.Contains(out, "Next: run the build") {
		t.Fatalf("opt-out must keep the instructional fallback, got:\n%s", out)
	}
}

func TestPatchVerifySkipsNonGo(t *testing.T) {
	dir := t.TempDir()
	out := applyPatchIn(t, dir, fmt.Sprintf(`*** Begin Patch
*** Add File: %s
+hello
*** End Patch`, "note.txt"))
	if strings.Contains(out, "verify:") {
		t.Fatalf("non-Go patch must not verify, got:\n%s", out)
	}
}

// Reproduces the live 2026-08-12 TUI failure: a doubly-nested tool-call
// envelope whose mangled tail (`"`, ` }`, `}"},"name":"apply_patch`) leaked
// into the patch as unmatchable context lines, failing an otherwise-correct
// hunk four times until the loop-breaker killed the turn.
func TestNestedEnvelopeDebrisTailShorn(t *testing.T) {
	if goRootDir() == "" {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"),
		[]byte("package main\n\nfunc main() {\n\tprintln(\"start\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := "{\n  \"name\": \"apply_patch\",\n  \"arguments\": {\n    \"input\": \"*** Update File: main.go\\n@@ func main() {\\n-    println(\\\"start\\\")\\n+    println(\\\"hello-verify\\\")\\n}\\n\"\n    }\n  }\"},\"name\":\"apply_patch"
	out := applyPatchIn(t, dir, raw)
	body, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "hello-verify") {
		t.Fatalf("patch did not land, file:\n%s\nresult:\n%s", body, out)
	}
	if !strings.Contains(out, "verify: PASS") {
		t.Fatalf("want verify: PASS, got:\n%s", out)
	}
}

// Reproduces the live 2026-08-15 smoke failure: a triple-nested envelope whose
// inner patch body was WHOLESALE-INDENTED by four spaces — "    @@ func" and
// "    +    println(…)" parsed as unmatchable context and the turn died after
// four refusals. The exact raw input from the session transcript.
func TestWholesaleIndentedPatchBodyLands(t *testing.T) {
	if goRootDir() == "" {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"),
		[]byte("package main\n\nfunc main() {\n\tprintln(\"start\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := "{\"arguments\":{\"input\":\"{\"name\":\"apply_patch\",\"arguments\":{\"input\": \"*** Update File: main.go\\n    @@ func main() {\\n         println(\\\"start\\\")\\n    }\\n    +    println(\\\"smoke-ok\\\")\\n\"}}\"},\"name\":\"apply_patch\"}"
	out := applyPatchIn(t, dir, raw)
	body, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "smoke-ok") {
		t.Fatalf("patch did not land, file:\n%s\nresult:\n%s", body, out)
	}
	if !strings.Contains(out, "verify: PASS") {
		t.Fatalf("want verify: PASS, got:\n%s", out)
	}
}
