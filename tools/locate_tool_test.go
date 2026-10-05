package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/logs"
)

// A small repository in three languages, plus the files locate must never
// return: a lock file and a minified bundle.
func locateFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("web/auth.ts", `export async function login(user: string, password: string) {
  const session = await fetch('/api/login', { method: 'POST', body: JSON.stringify({ user, password }) });
  return session.ok;
}

export function logout() {
  localStorage.removeItem('token');
}
`)
	write("billing/invoice.py", `def invoice_total(lines):
    """Sum an invoice's lines with tax."""
    return sum(l.price * l.quantity for l in lines) * 1.2


def refund(invoice):
    return -invoice_total(invoice.lines)
`)
	write("render.go", `package app

func RenderScreen() string { return "screen" }
`)
	write("package-lock.json", `{"login": "login login login password session"}`)
	write("web/app.min.js", `function login(){}login();login();password;session;`)
	return dir
}

func callLocate(t *testing.T, dir string, args map[string]string) string {
	t.Helper()
	args["cwd"] = dir
	in, _ := json.Marshal(args)
	out, err := Locate(json.RawMessage(in))
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	return out
}

// firstRanked is the file on the first ranked line.
func firstRanked(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "1. ") {
			return strings.SplitN(strings.TrimPrefix(l, "1. "), ":", 2)[0]
		}
	}
	return ""
}

// Any language: the query's words find the file that holds them, with its
// section and line numbers; lock files and minified bundles never come back.
func TestLocateFindsCodeInAnyLanguage(t *testing.T) {
	withDecisions(t, nil)
	dir := locateFixture(t)
	for query, want := range map[string]string{
		"the user login with a password fails": "web/auth.ts",
		"invoice total is wrong, tax":          "billing/invoice.py",
		"render the screen":                    "render.go",
	} {
		out := callLocate(t, dir, map[string]string{"query": query})
		if got := firstRanked(out); got != want {
			t.Errorf("%q: first ranked %q, want %q:\n%s", query, got, want, out)
		}
		if strings.Contains(out, "package-lock.json") || strings.Contains(out, "app.min.js") {
			t.Errorf("%q: a lock file or a minified bundle came back:\n%s", query, out)
		}
	}
	out := callLocate(t, dir, map[string]string{"query": "the user login with a password fails"})
	if !strings.Contains(out, "== web/auth.ts:1-") || !strings.Contains(out, "    1  export async function login") {
		t.Errorf("the top section must be shown with line numbers:\n%s", out)
	}
}

func TestLocateRequiresQuery(t *testing.T) {
	in, _ := json.Marshal(map[string]string{"query": "  ", "cwd": t.TempDir()})
	if _, err := Locate(json.RawMessage(in)); err == nil {
		t.Fatal("an empty query must be refused")
	}
}

// One file's sections, ranked: set explicitly, or named in the query.
func TestLocateRanksOneFilesSections(t *testing.T) {
	withDecisions(t, nil)
	dir := locateFixture(t)
	for _, args := range []map[string]string{
		{"query": "refund", "file": "billing/invoice.py"},
		{"query": "refund in billing/invoice.py"},
	} {
		out := callLocate(t, dir, args)
		if !strings.HasPrefix(out, "Sections of billing/invoice.py") || !strings.Contains(out, "def refund") {
			t.Errorf("%v:\n%s", args, out)
		}
		if strings.Contains(out, "auth.ts") {
			t.Errorf("%v: another file leaked in:\n%s", args, out)
		}
	}
}

// Nothing matches: say what the workspace holds, never that a file is missing.
func TestLocateListsFilesOnNoMatch(t *testing.T) {
	withDecisions(t, nil)
	dir := locateFixture(t)
	out := callLocate(t, dir, map[string]string{"query": "zzqx wibble frobnicate"})
	if !strings.Contains(out, "render.go") || !strings.Contains(out, "web/auth.ts") {
		t.Fatalf("no-match must list the workspace:\n%s", out)
	}
}

// The index is reused while nothing changes and rebuilt after an edit, so
// the coder locates against its own changes.
func TestLocateRebuildsAfterAnEdit(t *testing.T) {
	withDecisions(t, nil)
	dir := locateFixture(t)
	first := indexFor(dir)
	if indexFor(dir) != first {
		t.Fatal("an unchanged repo must reuse its index")
	}
	later := time.Now().Add(time.Second)
	p := filepath.Join(dir, "billing/invoice.py")
	if err := os.WriteFile(p, []byte("def chargeback_window():\n    return 30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(p, later, later)
	if indexFor(dir) == first {
		t.Fatal("an edited file must rebuild the index")
	}
	if got := firstRanked(callLocate(t, dir, map[string]string{"query": "chargeback window"})); got != "billing/invoice.py" {
		t.Fatalf("the edit is not searchable: first ranked %q", got)
	}
}

// With a decision model, it decides: the section the words match best but the
// task does not need is dropped, and the one the task is about ranks first.
func TestLocateIsJudged(t *testing.T) {
	withDecisions(t, &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "RenderScreen") {
			return 0.95
		}
		return 0.05
	}})
	dir := locateFixture(t)
	out := callLocate(t, dir, map[string]string{"query": "the user login screen renders blank"})
	if !strings.Contains(out, "judged") || firstRanked(out) != "render.go" {
		t.Fatalf("not judged, or the judged section is not first:\n%s", out)
	}
	if strings.Contains(out, "auth.ts") {
		t.Fatalf("a section the judge rejected was returned:\n%s", out)
	}
}

// In a git repository, what .gitignore excludes is never searched.
func TestLocateSkipsIgnoredFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	withDecisions(t, nil)
	dir := locateFixture(t)
	_ = os.MkdirAll(filepath.Join(dir, "generated"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "generated", "login_gen.ts"), []byte("export const login = 'login password session user fails';\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("generated/\n"), 0o644)
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	out := callLocate(t, dir, map[string]string{"query": "the user login with a password fails"})
	if strings.Contains(out, "generated/") {
		t.Fatalf("an ignored file was searched:\n%s", out)
	}
	if firstRanked(out) != "web/auth.ts" {
		t.Fatalf("untracked, not-ignored files must still be found:\n%s", out)
	}
}

// A copy of the code under a test-data folder, or left behind by a patch,
// reads like the code: it must not come back for a question about the code
// (live 2026-09-29: tools/testdata/.../tui_remote.go.orig ranked first).
func TestLocateRanksCodeNotItsCopies(t *testing.T) {
	withDecisions(t, nil)
	dir := locateFixture(t)
	copyOf := func(name string) {
		body, err := os.ReadFile(filepath.Join(dir, "web/auth.ts"))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		// The copy says the words more often than the file it copies.
		if err := os.WriteFile(p, append(body, []byte("// login password user fails login password\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{
		"tools/testdata/live/auth.ts",
		"spec/fixtures/auth.ts",
		"web/__snapshots__/auth.ts",
		"web/auth.ts.orig",
		"web/auth.ts.rej",
		"web/auth.ts~",
	} {
		copyOf(name)
	}
	out := callLocate(t, dir, map[string]string{"query": "the user login with a password fails"})
	if got := firstRanked(out); got != "web/auth.ts" {
		t.Fatalf("first ranked %q, want the code:\n%s", got, out)
	}
	for _, copied := range []string{"testdata", "fixtures", "__snapshots__", ".orig", ".rej", "auth.ts~"} {
		if strings.Contains(out, copied) {
			t.Errorf("a copy (%s) came back beside the code:\n%s", copied, out)
		}
	}

	// Named, a test-data file is searched like any other.
	out = callLocate(t, dir, map[string]string{"query": "login", "file": "tools/testdata/live/auth.ts"})
	if !strings.HasPrefix(out, "Sections of tools/testdata/live/auth.ts") {
		t.Fatalf("a named test-data file must be searched:\n%s", out)
	}
}

// When test data is all that matches, it is what comes back.
func TestLocateFallsBackToTestData(t *testing.T) {
	withDecisions(t, nil)
	dir := locateFixture(t)
	p := filepath.Join(dir, "testdata/golden/report.txt")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte("quarterly zeppelin manifest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := firstRanked(callLocate(t, dir, map[string]string{"query": "zeppelin manifest"})); got != "testdata/golden/report.txt" {
		t.Fatalf("first ranked %q, want the only file that matches", got)
	}
}

// read_file of a GUESSED nonexistent name ("main.go" on a fresh task) must list the
// directory's real source files, so the model's next read targets one instead of flailing.
func TestReadFileMissingListsSiblings(t *testing.T) {
	if err := logs.InitGlobalLoggerDefault(false); err != nil {
		t.Fatal(err)
	}
	dir := locateFixture(t)
	in, _ := json.Marshal(map[string]string{"path": filepath.Join(dir, "main.go")})
	_, err := ReadFile(json.RawMessage(in))
	if err == nil {
		t.Fatal("missing file should error")
	}
	if !strings.Contains(err.Error(), "render.go") || !strings.Contains(err.Error(), "web/auth.ts") {
		t.Fatalf("error should list the real files, got: %v", err)
	}
}
