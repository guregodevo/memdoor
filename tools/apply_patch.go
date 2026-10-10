package tools

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/pmezard/go-difflib/difflib"

	"golang.org/x/mod/modfile"
)

// apply_patch — the single file tool, ported from OpenClaw (src/agents/apply-patch.ts
// + apply-patch-update.ts), which uses the Codex "*** Begin Patch" diff format. One
// tool creates, edits, and deletes files; edits are small context-located diff hunks
// (not a whole-file rewrite, not an exact full-string match), and the hunk is located
// with a progressive fuzzy match so a whitespace slip doesn't fail the edit.
//
// Format:
//
//	*** Begin Patch
//	*** Add File: path/to/new.go
//	+package main
//	+
//	+func main() {}
//	*** Update File: path/to/existing.go
//	@@ func main() {
//	 	x := 1
//	-	return x
//	+	return x + 1
//	*** Delete File: path/to/old.go
//	*** End Patch
//
// In an Update hunk: a leading " " line is unchanged context, "-" removes a line, "+"
// adds one; "@@ text" narrows where the hunk applies. In an Add hunk every line is "+".

const (
	beginPatchMarker    = "*** Begin Patch"
	endPatchMarker      = "*** End Patch"
	addFileMarker       = "*** Add File: "
	deleteFileMarker    = "*** Delete File: "
	updateFileMarker    = "*** Update File: "
	moveToMarker        = "*** Move to: "
	eofMarker           = "*** End of File"
	changeContextMarker = "@@ "
	emptyContextMarker  = "@@"
	openToolCallTag     = "<tool_call>"
	closeToolCallTag    = "</tool_call>"
)

type patchChunk struct {
	changeContext string
	hasContext    bool
	oldLines      []string
	newLines      []string
	isEndOfFile   bool
	// hasRemoval records that the hunk deletes at least one line ("-"). A hunk with
	// no removals is pure ADDITION — used to recover a new-declaration hunk whose
	// "@@" anchor names a signature not yet in the file (see computeReplacements).
	hasRemoval bool
	// removedLines / addedLines are just the "-" and "+" lines, in order — the
	// hunk's ACTUAL change, apart from context. A small model routinely places the
	// change AFTER spurious context (echoing the body then the real -/+), so the full
	// old-block won't match; these let computeReplacements retry as a minimal
	// search/replace (find the removed lines, swap in the added ones).
	removedLines []string
	addedLines   []string
	// contextOld pairs each newLines entry with the oldLines entry it repeats:
	// the index of that context line in oldLines, -1 for an added line. Only
	// parsed diff hunks carry it (see preserveContext).
	contextOld []int
	// markerStripCandidate marks a SEARCH/REPLACE chunk whose lines may carry a
	// small model's diff-marker habit (a uniform leading "+"/"-"). Stripping is
	// tried only as a MATCH-TIME fallback (see computeReplacements) so a genuine
	// uniform "-"/"+" block — a Markdown bullet list, a YAML sequence — is left
	// intact when it already matches the file.
	markerStripCandidate bool
}

type patchHunk struct {
	kind     string // "add" | "delete" | "update"
	path     string
	movePath string
	contents string       // add
	chunks   []patchChunk // update
}

// ApplyPatchInput is the tool input. Cwd is internal — set by the coder-workdir
// confinement so relative patch paths resolve inside the sandbox, never sent by the model.
type ApplyPatchInput struct {
	Input string `json:"input" jsonschema_description:"A patch in the *** Begin Patch / *** End Patch format. Use *** Add File: <path> to create a file (every content line prefixed with +); *** Update File: <path> then a hunk — removed lines with -, added lines with +, and unchanged lines (leading space) only where needed to make the hunk unique, optionally preceded by @@ <the enclosing function's first line>; *** Delete File: <path> to remove. Read the file first so the context lines are exact."`
	Cwd   string `json:"-"`
}

var ApplyPatchInputSchema = GenerateSchema[ApplyPatchInput]()

var ApplyPatchDefinition = ToolDefinition{
	Name: "apply_patch",
	Description: `The single tool for changing files — it CREATES, EDITS, and DELETES via a patch.

The blocks below show the FORMAT ONLY. Never copy their filenames or lines — always
patch the ACTUAL file and the ACTUAL lines named in your task. Wrap everything in
*** Begin Patch / *** End Patch. Inside, one or more file sections:

• CREATE a file — every content line prefixed with "+". Write the REAL program the
  task asks for, in full; do NOT emit a placeholder or a "hello world" stub. Shape:
    *** Add File: <your-file>.<ext>
    +<first real line of your program>
    +<... every remaining line, each prefixed with +, complete and runnable ...>

• EDIT a file — mark removed lines with "-" and added lines with "+", exactly as in
  the file. Do not copy unchanged lines around them unless the "-" lines appear more
  than once in the file: then pin the hunk with "@@ <the enclosing function's first
  line>" (cheapest) or one unchanged line (leading space). An ambiguous hunk is
  refused, never guessed. Shape:
    *** Update File: <the-file-you-are-editing>
    @@ <the first line of the function you are editing>
    -    <the exact current line to remove>
    +    <the new line>
  Use several hunks for several changes. To fix a compile error, put the exact broken
  line (from the error) as a "-" and the corrected line as a "+".

• DELETE a file:
    *** Delete File: <the-file>

Prefer small edits (Update) over rewriting whole files.`,
	InputSchema: ApplyPatchInputSchema,
	Function:    ApplyPatch,
}

// ApplyPatch parses and applies a patch, resolving relative paths under Cwd.
// patchRecreates reports whether the patch carries an Add File for path — the
// delete+add pair that makes a Delete File a legitimate rewrite instead of data loss.
func patchRecreates(hunks []patchHunk, path string) bool {
	for _, h := range hunks {
		if h.kind == "add" && h.path == path {
			return true
		}
	}
	return false
}

// goRootDir resolves GOROOT once (exec "go env GOROOT"); empty when go is absent,
// which disables stdlib verification (the import guard then stands down).
var goRootDir = sync.OnceValue(func() string {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
})

// moduleInfo is the go.mod ground truth an import must resolve against.
type moduleInfo struct {
	name     string   // module path (import prefix for project-local packages)
	root     string   // directory containing go.mod
	requires []string // declared dependency module paths
}

// findGoMod walks up from dir to locate go.mod and parses the module name and
// require paths. Returns nil when the file tree has no module (standalone dir).
func findGoMod(dir string) *moduleInfo {
	for d := dir; ; {
		if b, err := os.ReadFile(filepath.Join(d, "go.mod")); err == nil {
			mi := &moduleInfo{root: d}
			for _, l := range strings.Split(string(b), "\n") {
				if i := strings.Index(l, "//"); i >= 0 {
					l = l[:i] // "// indirect" comments break field counting
				}
				t := strings.TrimSpace(l)
				if strings.HasPrefix(t, "module ") {
					mi.name = strings.TrimSpace(strings.TrimPrefix(t, "module "))
				} else if strings.HasPrefix(t, "require ") && !strings.Contains(t, "(") {
					if f := strings.Fields(t); len(f) >= 2 {
						mi.requires = append(mi.requires, f[1])
					}
				} else if f := strings.Fields(t); len(f) == 2 && strings.Contains(f[0], "/") {
					// inside a require ( ... ) block: "path version"
					mi.requires = append(mi.requires, f[0])
				}
			}
			return mi
		}
		parent := filepath.Dir(d)
		if parent == d {
			return nil
		}
		d = parent
	}
}

// guardUnresolvableImports refuses a .go write whose imports cannot exist here —
// the model's #1 recurring hallucination (placeholder paths like
// "your-project/sharedctx", or deps like testify that go.mod never declares).
// Ground truth, not heuristics: an import must be stdlib (present under
// GOROOT/src), a package of THIS module (go.mod's module path prefix), or a
// declared dependency (require list prefix). The error teaches the real module
// name so the retry writes a resolvable path.
func guardUnresolvableImports(target, content string, pendingDirs map[string]bool) error {
	groot := goRootDir()
	if groot == "" {
		return nil // no toolchain visible — cannot verify, stand down
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", content, parser.ImportsOnly)
	if err != nil {
		return nil // syntax guard owns parse errors
	}
	mod := findGoMod(filepath.Dir(target))
	var bad []string
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		if fi, err := os.Stat(filepath.Join(groot, "src", filepath.FromSlash(p))); err == nil && fi.IsDir() {
			continue // stdlib
		}
		if mod != nil {
			if mod.name != "" && (p == mod.name || strings.HasPrefix(p, mod.name+"/")) {
				// Module-local: the package DIRECTORY must actually exist — a prefix
				// match alone lets a hallucinated subpackage ("demo/assert") through,
				// which then fails the build anyway.
				rel := strings.TrimPrefix(strings.TrimPrefix(p, mod.name), "/")
				pkgDir := filepath.Join(mod.root, filepath.FromSlash(rel))
				// A package this same patch is creating counts as existing —
				// atomic multi-file patches add the package and its importer together.
				if dirHasGoFiles(pkgDir) || pendingDirs[pkgDir] {
					continue
				}
				bad = append(bad, p)
				continue
			}
			declared := false
			for _, r := range mod.requires {
				if p == r || strings.HasPrefix(p, r+"/") {
					declared = true
					break
				}
			}
			if declared {
				continue
			}
		}
		bad = append(bad, p)
	}
	if len(bad) == 0 {
		return nil
	}
	hint := "only standard-library imports are available here (no go.mod)"
	if mod != nil && mod.name != "" {
		hint = fmt.Sprintf("this project's module is %q; its EXISTING packages are: %s. Use one of those or the standard library. To use a third-party package FIRST add it with bash: go get <module> — then retry this exact patch (NOT go mod tidy, which removes unused deps)", mod.name, strings.Join(modulePackages(mod), ", "))
		// A bare single-segment import ("store") is almost always the model
		// reaching for a project-local package by its short name — teach the
		// exact spelling AND the create-it-in-the-same-patch move, or the model
		// flails (live: derailed to a hello-world after this refusal).
		for _, p := range bad {
			if !strings.Contains(p, "/") && !strings.Contains(p, ".") {
				hint = fmt.Sprintf("%q looks like a project-local package: its import path is %q (module %q + package dir). If %s/ doesn't exist yet, create it in the SAME patch — one patch may contain *** Add File: %s/%s.go AND the file that imports it. Do NOT edit go.mod: project-local packages need no registration", p, mod.name+"/"+p, mod.name, p, p, p)
				break
			}
		}
	}
	return fmt.Errorf("import(s) %s do not exist in this project — the file was NOT written. %s", strings.Join(bad, ", "), hint)
}

// goParseErr returns the first syntax error in src parsed as a Go file, or nil.
func goParseErr(name, src string) error {
	_, err := parser.ParseFile(token.NewFileSet(), name, src, 0)
	return err
}

// guardSyntaxRegression refuses a write that would turn a currently-PARSEABLE .go file
// into an unparseable one — the corruption that strands a small model (it cannot express the
// "-"-only deletion hunks needed to repair structure, so a broken file stays broken).
// This is the tool validating its OWN output and returning an explicit error to the
// model, not a build gate: no compile, no type-check, just "the result must still be
// syntactically Go". A file that is ALREADY broken accepts any patch (repair attempts
// must not be blocked), and non-Go files are untouched.
func guardSyntaxRegression(target, newContent string) error {
	if !strings.HasSuffix(target, ".go") {
		return nil
	}
	old, err := os.ReadFile(target)
	if err != nil {
		return nil // new file — nothing to regress
	}
	if goParseErr(target, string(old)) != nil {
		return nil // already unparseable — allow repair attempts
	}
	if goParseErr(target, newContent) != nil {
		return fmt.Errorf("patch would BREAK %s — the result is no longer valid Go:\n%s\nThe file was NOT modified. Fix the patch, or REWRITE the whole file with *** Add File: %s and the full corrected content — that is the reliable way to delete code or repair structure", filepath.Base(target), describeGoParseErrors(target, newContent), filepath.Base(target))
	}
	return nil
}

// APPLY_PATCH WRITES WHAT THE PATCH SAYS (Greg, 2026-09-28: "it should be
// language agnostic", "it was done for small model. We dont need it for
// frontier models", "no opt-in"). The Go rewriters — import add/prune,
// declaration dedup, module-ref collapse, syntax repair, gofmt — changed code
// the model never wrote: live 2026-09-28 they added a second import block and
// deleted a used import (qrterminal/v3 read as "v3") three patches running,
// and the coder patched a file it did not know. They are gone. What stays
// changes nothing on disk: refusals (a bad go.mod, an import nothing
// provides, a parse regression) and the build report.
func ApplyPatch(input json.RawMessage) (_ string, err error) {
	var params ApplyPatchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid apply_patch input: %w", err)
	}
	// Cwd and TurnReads are kept out of the model-facing schema; the coder-workdir
	// confinement injects them (cwd = the sandbox; turn_reads = files this turn has
	// actually read or written). TurnReads == nil means no enforcement (CLI/tests).
	var conf struct {
		Cwd       string   `json:"cwd"`
		TurnReads []string `json:"turn_reads"`
	}
	_ = json.Unmarshal(input, &conf)
	params.Cwd = conf.Cwd
	// Line-anchored edits (hashline.go) are recognised before the patch
	// format's repairs run: those repairs rewrite +/- hunks.
	params.Input = decodeLiteralEscapes(params.Input)
	if isHashlinePatch(params.Input) {
		return applyHashlinePatch(params.Input, params.Cwd)
	}
	params.Input = normalizePatchInput(params.Input)
	hunks, err := parsePatch(params.Input)
	if err != nil {
		return "", err
	}
	// A refused patch that spans several files says so for all of them: the
	// error names the one that broke and read "the file was NOT modified", so
	// a model took the others as written and patched lines that were never
	// there (2026-09-28). The patch is all or nothing — nothing in it landed.
	if files := patchFiles(hunks); len(files) > 1 {
		defer func() {
			if err != nil {
				err = fmt.Errorf("%w\n\nNothing in this patch was written — not %s. Fix it and resend the whole patch.", err, strings.Join(files, ", "))
			}
		}()
	}
	// ATOMICITY: validate EVERY add hunk before writing ANY file. Live failure:
	// a two-file patch wrote store/store.go, then main.go's hunk was refused —
	// the error said "the file was NOT written" while half the patch HAD landed,
	// and the model was cornered (its own file now trips the overwrite guard,
	// since an errored call records no turn-write). Guards first, writes after —
	// an apply_patch error now always means "nothing changed".
	targets := make([]string, len(hunks))
	pendingGoDirs := map[string]bool{}
	for i := range hunks {
		t := hunks[i].path
		if params.Cwd != "" && !filepath.IsAbs(t) {
			t = filepath.Join(params.Cwd, t)
		}
		t = absorbModulePathEcho(t)
		targets[i] = t
		if hunks[i].kind == "add" && strings.HasSuffix(t, ".go") {
			pendingGoDirs[filepath.Dir(t)] = true
		}
	}
	for i := range hunks {
		h := &hunks[i]
		if h.kind != "add" {
			continue
		}
		target := targets[i]
		// BLIND REWRITE GUARD: overwriting an EXISTING file the model has not
		// read (or just written) this turn is a rewrite-from-MEMORY — where a
		// small model drifts details it didn't just see (live: argument values changed
		// across a rewrite). Enforced only when the harness injected turn_reads.
		if _, statErr := os.Stat(target); statErr == nil && conf.TurnReads != nil &&
			!slices.Contains(conf.TurnReads, filepath.Base(target)) {
			return "", fmt.Errorf("apply_patch add %s: this OVERWRITES an existing file you have not read this turn — rewriting from memory loses details. read_file %s first, then rewrite from its ACTUAL content", h.path, h.path)
		}
		// Overwriting an existing GOOD file with unparseable content is refused
		// like any other syntax regression; a brand-new file always writes.
		if err := guardSyntaxRegression(target, h.contents); err != nil {
			return "", fmt.Errorf("apply_patch add %s: %w", h.path, err)
		}
		h.contents = shearEnvelopeJunkLines(target, h.contents)
		if err := guardGoModSyntax(target, h.contents); err != nil {
			return "", fmt.Errorf("apply_patch add %s: %w", h.path, err)
		}
		if strings.HasSuffix(target, ".go") {
			if goParseErr(target, h.contents) != nil {
				return "", fmt.Errorf("apply_patch add %s: content is not valid Go — the file was NOT written:\n%s\nFix these lines and resend the whole *** Add File patch", h.path, describeGoParseErrors(target, h.contents))
			}
			if err := guardUnresolvableImports(target, h.contents, pendingGoDirs); err != nil {
				return "", fmt.Errorf("apply_patch add %s: %w", h.path, err)
			}
		}
	}
	// ATOMICITY ACROSS SECTIONS: the pre-pass above covers add hunks, but an
	// update is validated as it is applied (each section reads the file the
	// previous one wrote). Every file the patch touches is snapshotted first
	// and restored if any section fails, so an error always means nothing
	// changed. Live 2026-09-28: a three-section update wrote two sections, the
	// third was refused with "the file was NOT written", and the file had
	// doubled — the model then patched a file it no longer knew.
	snap := snapshotPatchTargets(targets, hunks, params.Cwd)
	committed := false
	defer func() {
		if !committed {
			snap.restore()
		}
	}()
	var added, modified, deleted []string
	priorText := map[string]string{} // a modified file's text before this patch, for the echo
	for i := range hunks {
		h := &hunks[i]
		target := targets[i]
		switch h.kind {
		case "add":
			// Guards all passed in the pre-pass — this write cannot half-fail
			// the patch on validation grounds.
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", fmt.Errorf("apply_patch: %w", err)
			}
			if err := os.WriteFile(target, []byte(h.contents), 0o644); err != nil {
				return "", fmt.Errorf("apply_patch add %s: %w", h.path, err)
			}
			added = append(added, h.path)
		case "delete":
			// A small model asked to "delete those two methods" emits a bare *** Delete File —
			// nuking the whole file it was told to EDIT, then losing it when the turn
			// ends before a recreate. Whole-file deletion of an existing file is refused
			// unless this same patch recreates it (an Add File for the same path — the
			// rewrite flow); the error teaches the reliable move. Real file removal, if
			// ever intended, still works via bash rm.
			if _, statErr := os.Stat(target); statErr == nil && !patchRecreates(hunks, h.path) {
				return "", fmt.Errorf("apply_patch delete %s: Delete File removes the WHOLE file — refusing, %s is untouched. To delete code INSIDE it, REWRITE it instead: one patch with \"*** Add File: %s\" and the full corrected content (it overwrites)", h.path, h.path, h.path)
			}
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("apply_patch delete %s: %w", h.path, err)
			}
			deleted = append(deleted, h.path)
		case "update":
			updated, err := applyUpdateHunk(target, h.chunks)
			if err != nil {
				// Updating a GUESSED filename ("greet.go" on a fresh task) dead-ends the
				// turn with a bare not-found. List what IS here — same absorption as
				// read_file's missing-file listing — so the next patch targets a real file.
				if strings.Contains(err.Error(), "no such file") {
					if files := listSourceFiles(filepath.Dir(target)); len(files) > 0 {
						return "", fmt.Errorf("apply_patch update %s: the file does not exist. The directory DOES contain: %s — Update one of those (do not guess names), or use *** Add File: to create a new one", h.path, strings.Join(files, ", "))
					}
				}
				return "", fmt.Errorf("apply_patch update %s: %w", h.path, err)
			}
			// CONTEXT-REWRITE: a small model's favorite broken dialect is a hunk that shows
			// the CORRECTED code as bare context lines — no +/- markers at all
			// (live: three identical "changed NOTHING" refusals on a Mean/Max fix,
			// then it gave up and asked the user). When such a hunk ALMOST matches
			// a file region (anchored by the lines that are equal), it IS the
			// replacement — apply it.
			if prev, rerr := os.ReadFile(target); rerr == nil && contentEquivalent(string(prev), updated) {
				if rew, ok := applyContextRewrite(target, string(prev), h.chunks); ok {
					updated = rew
				}
			}
			updated = shearEnvelopeJunkLines(target, updated)
			if err := guardGoModSyntax(target, updated); err != nil {
				return "", fmt.Errorf("apply_patch update %s: %w", h.path, err)
			}
			if err := guardSyntaxRegression(target, updated); err != nil {
				return "", fmt.Errorf("apply_patch update %s: %w", h.path, err)
			}
			// A NO-OP patch (context-only hunks, or changes that reproduce the file
			// verbatim) must not report success — the model reads "Patch applied" as
			// task done and stops without ever making the change. Refuse with the
			// shape of a real hunk spelled out.
			// Whitespace IS the change when a hunk says so: a "-"/"+" pair that
			// differs only in indentation re-indents a Python line (live: an
			// IndentationError repair refused three times, 2026-10-10), so the
			// collapsed comparison applies only to a patch without such a pair.
			if prev, rerr := os.ReadFile(target); rerr == nil && contentEquivalent(string(prev), updated) &&
				(string(prev) == updated || !hasExplicitReplacement(h.chunks)) {
				return "", fmt.Errorf("apply_patch update %s: the patch changed NOTHING — it contains only context lines (no additions or removals), or its changes match the file as-is. Write a hunk with the line to change as \"-old line\" and its replacement as \"+new line\". Changing several lines? read_file %s and REWRITE it with *** Add File: %s using its ACTUAL content plus your change", h.path, filepath.Base(target), filepath.Base(target))
			}
			dest := target
			if h.movePath != "" {
				dest = h.movePath
				if params.Cwd != "" && !filepath.IsAbs(dest) {
					dest = filepath.Join(params.Cwd, dest)
				}
			}
			if strings.HasSuffix(dest, ".go") {
				if err := guardUnresolvableImports(dest, updated, pendingGoDirs); err != nil {
					return "", fmt.Errorf("apply_patch update %s: %w", h.path, err)
				}
			}
			if prev, rerr := os.ReadFile(target); rerr == nil {
				priorText[h.path] = string(prev)
			}
			if err := os.WriteFile(dest, []byte(updated), 0o644); err != nil {
				return "", fmt.Errorf("apply_patch write %s: %w", h.path, err)
			}
			if h.movePath != "" && dest != target {
				_ = os.Remove(target)
			}
			modified = append(modified, h.path)
		}
	}
	// The success result carries what the patch MEANS: whether the code now
	// compiles. The build check runs here, structurally, because a small model
	// told to "run the build next" skips it and reports done on code it never
	// ran — with the compiler's verdict inside the same tool result, the retry
	// loop becomes execution-grounded selection (sample a patch, keep the first
	// that PASSes). Falls back to the instruction when verification can't run
	// (non-Go files, no toolchain, a test that turned it off, timeout).
	committed = true
	msg := fmt.Sprintf("Patch applied. added=%v modified=%v deleted=%v\n", added, modified, deleted)
	if verdict := autoVerifyGoPatch(params.Cwd, append(append([]string{}, added...), modified...)); verdict != "" {
		msg += verdict
	} else {
		msg += "Next: run the build/test with the bash tool and read the real output. If it fails (e.g. \"undefined: X\" means you called something you did not define yet), fix it with another patch. Do NOT report done until it builds and runs."
	}
	// Echo what changed, as it now reads: the coder's next edit copies its "-"
	// lines from the real text, not from memory. The changed lines ±3, numbered —
	// not the whole file, which was 30% of the new content entering a coder
	// conversation (8 tasks, 2026-09-30: 26,915 of 91,721 tool-result chars).
	// An added file is not echoed: it holds exactly what the patch said.
	for _, path := range modified {
		prev, ok := priorText[path]
		if !ok {
			continue
		}
		full := path
		if params.Cwd != "" && !filepath.IsAbs(full) {
			full = filepath.Join(params.Cwd, full)
		}
		if body, err := os.ReadFile(full); err == nil {
			if shown := changedRegions(prev, string(body)); shown != "" {
				msg += fmt.Sprintf("\n\n%s now (%d lines), around the change:\n%s", path, len(addressableLines(string(body))), shown)
			}
		}
	}
	return msg, nil
}

// changedRegions is after's changed lines ±3, numbered, "…" between regions.
func changedRegions(before, after string) string {
	old, cur := addressableLines(before), addressableLines(after)
	var spans [][2]int
	for _, oc := range difflib.NewMatcher(old, cur).GetOpCodes() {
		if oc.Tag != 'e' {
			spans = append(spans, [2]int{oc.J1, max(oc.J2, oc.J1+1)})
		}
	}
	shown, _ := hashlineWindows(cur, spans, 3)
	return shown
}

// literalEscapeRun matches a run of ONE OR MORE backslashes before an escape char.
// A small model escapes newlines as "\n" in the JSON string; on a retry it often
// DOUBLE-escapes ("\\n") after seeing its own escaped patch echoed back. Collapsing
// any-length backslash run (not just one) recovers both — a fixed two-rule Replacer
// would eat "\\n" as an escaped-backslash then a bare "n", yielding a literal "\n"
// instead of a newline.
var literalEscapeRun = regexp.MustCompile(`\\+([ntr"'])`)

// headerGluedEscape detects a file-header line with a literal "\n" glued on — the
// signature of a nested envelope that half-decoded (path and hunk fused on one line).
var headerGluedEscape = regexp.MustCompile(`\*\*\* (Add|Update|Delete) File: [^\n]*\\n`)

// decodeOneEscape turns one literalEscapeRun match into its real character.
func decodeOneEscape(m string) string {
	switch m[len(m)-1] {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'r':
		return "\r"
	case '"':
		return "\""
	case '\'':
		return "'"
	}
	return m
}

// patchMarkerPrefixes are the tokens a real patch body starts with — used to detect
// when unwrapping has reached the actual patch and should stop.
var patchMarkerPrefixes = []string{beginPatchMarker, addFileMarker, updateFileMarker, deleteFileMarker, moveToMarker}

// looksLikePatch reports whether s (already trimmed) begins a real patch body.
func looksLikePatch(s string) bool {
	for _, m := range patchMarkerPrefixes {
		if strings.HasPrefix(s, strings.TrimRight(m, " ")) {
			return true
		}
	}
	return false
}

// unwrapPatchEnvelope peels the wrappers a small model puts around the patch instead
// of passing the bare patch as "input":
//
//	<tool_call> { "name":"apply_patch", "arguments":{"input":"*** ..."} } </tool_call>
//	{"arguments":{"input":"*** ..."}}                    (JSON-RPC-ish)
//	{"input":"*** ..."}                                  (redundant re-wrap)
//
// possibly nested more than once, and possibly with a <tool_call> tag around the
// JSON. Without this the tool parses the wrapper as the patch ("invalid hunk header:
// <tool_call>" / "{") and the coder loops. It stops as soon as the text is a real
// patch (starts with a *** marker).
func unwrapPatchEnvelope(s string) string {
	for i := 0; i < 5; i++ {
		t := strings.TrimSpace(s)
		t = strings.TrimPrefix(t, openToolCallTag)
		t = strings.TrimSuffix(t, closeToolCallTag)
		t = strings.TrimSpace(t)
		if looksLikePatch(t) {
			return t
		}
		// Skip any junk before an embedded JSON object (e.g. a stray tool-call tag).
		if j := strings.Index(t, "{"); j >= 0 {
			t = t[j:]
		} else {
			return s
		}
		var env struct {
			Input     *string `json:"input"`
			Arguments *struct {
				Input *string `json:"input"`
			} `json:"arguments"`
		}
		if json.Unmarshal([]byte(t), &env) != nil {
			// The inner "input" is a multi-line patch, so it often carries REAL newlines
			// (and tabs) — invalid inside a JSON string. Escape the in-string control
			// chars and retry once, so an envelope-wrapped patch still unwraps instead of
			// leaking the wrapper as a bogus hunk header.
			if repaired := escapeJSONControlChars(t); repaired != t && json.Unmarshal([]byte(repaired), &env) == nil {
				// fall through with env populated
			} else {
				return s
			}
		}
		switch {
		case env.Arguments != nil && env.Arguments.Input != nil:
			s = *env.Arguments.Input
		case env.Input != nil:
			s = *env.Input
		default:
			return s
		}
	}
	return s
}

// escapeJSONControlChars escapes raw newline/tab/CR bytes that appear INSIDE a JSON
// string literal (leaving structure and whitespace outside strings untouched), so a
// multi-line patch a small model wrote with real line breaks inside {"input":"…"} can be
// json.Unmarshal'd. Format-preserving repair, not a JSON-relaxing dependency.
func escapeJSONControlChars(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	inString, escaped := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		switch c {
		case '\\':
			b.WriteByte(c)
			escaped = true
		case '"':
			inString = !inString
			b.WriteByte(c)
		case '\n':
			if inString {
				b.WriteString("\\n")
			} else {
				b.WriteByte(c)
			}
		case '\t':
			if inString {
				b.WriteString("\\t")
			} else {
				b.WriteByte(c)
			}
		case '\r':
			if inString {
				b.WriteString("\\r")
			} else {
				b.WriteByte(c)
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// sliceEmbeddedPatch extracts a patch that is EMBEDDED as a JSON string value the tool
// couldn't unwrap — because the patch (Go code) carries UNESCAPED quotes (import "fmt",
// Println("…")) that make json.Unmarshal fail. It shears the patch out by its own
// markers instead of parsing JSON: slice from the first *** marker, then drop the
// trailing wrapper (the closing quote followed only by }/] that closes the "input"
// string and its objects). A bare, already-clean patch (marker at the front, no JSON
// wrapper before it) is returned untouched.
func sliceEmbeddedPatch(s string) string {
	start := -1
	for _, mk := range []string{beginPatchMarker, "*** Add File:", "*** Update File:", "*** Delete File:"} {
		if i := strings.Index(s, mk); i >= 0 && (start < 0 || i < start) {
			start = i
		}
	}
	if start <= 0 {
		return s // no marker, or already at the front (nothing wrapping it)
	}
	prefix := strings.TrimSpace(s[:start])
	if !strings.HasPrefix(prefix, "{") && !strings.HasPrefix(prefix, "[") && !strings.HasPrefix(prefix, openToolCallTag) {
		return s // preceded by real prose, not a JSON/tag wrapper — leave it
	}
	body := s[start:]
	if j := strings.Index(body, endPatchMarker); j >= 0 {
		return body[:j+len(endPatchMarker)]
	}
	// No End Patch marker: strip a trailing wrapper suffix — the LAST '"' that is
	// followed only by }/] (and whitespace) to the end closes the JSON "input" string.
	if k := strings.LastIndex(body, "\""); k >= 0 {
		if tail := strings.TrimSpace(body[k+1:]); tail != "" && strings.Trim(tail, "}] \t\r\n") == "" {
			return strings.TrimRight(body[:k], " \t\r\n")
		}
	}
	return body
}

// normalizePatchInput repairs the ways a patch arrives mangled from a small model's
// tool call BEFORE parsing (complements the branch's match-time strip-defer):
//   - JSON envelope: the model wraps the patch in {"input":…} / {"arguments":
//     {"input":…}} instead of passing the bare patch string; unwrap it.
//   - Double-wrap: slice to the *** Begin/End Patch markers, which are unambiguous
//     and survive any JSON mangling around them.
//   - HTML unicode escapes: json.Marshal renders < > & as < > &; a
//     re-marshal in the tool path can leave them, corrupting the "<<<<<<<" markers.
//   - Literal escapes: the patch collapses onto ONE line with "\n"/"\t"/"\"" as
//     backslash+char instead of real newlines; decode them when no real newline
//     exists — including the DOUBLE-escaped "\\n" a retry produces.
func normalizePatchInput(s string) string {
	s = unwrapPatchEnvelope(s)
	s = sliceEmbeddedPatch(s)
	if i := strings.Index(s, beginPatchMarker); i > 0 {
		s = s[i:]
		if j := strings.Index(s, endPatchMarker); j >= 0 {
			s = s[:j+len(endPatchMarker)]
		}
	}
	s = decodeHTMLUnicodeEscapes(s)
	// A triple-nested envelope can leave literal "\n" GLUED into the header line
	// itself ("*** Update File: x.go\n@@ …" as one line) while the rest has real
	// newlines — the path then parses as the whole remainder. When a file-header
	// line carries a literal \n, decode the literal escapes across the whole patch
	// even though real newlines exist.
	if headerGluedEscape.MatchString(s) {
		s = literalEscapeRun.ReplaceAllStringFunc(s, decodeOneEscape)
	}
	if !strings.Contains(s, "\n") && strings.Contains(s, "\\n") {
		s = literalEscapeRun.ReplaceAllStringFunc(s, decodeOneEscape)
	}
	s = shearTrailingEnvelopeDebris(s)
	s = dedentHunkBody(s)
	return s
}

// dedentHunkBody strips a UNIFORM indent from a patch whose body the model
// indented wholesale (live: every line four-space indented — "    @@ func" and
// "    +    println(…)" parse as unmatchable context, and the turn dies in the
// retry loop). Only fires when the markers are provably hiding under the
// indent: all non-header lines share a positive common indent AND stripping it
// reveals an @@/+/- structure. File headers (***) already sit at column 0.
func dedentHunkBody(s string) string {
	lines := strings.Split(s, "\n")
	minIndent := -1
	revealed := false
	for _, l := range lines {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "***") {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if n == 0 {
			return s // a body line already at column 0 — not a wholesale indent
		}
		if minIndent < 0 || n < minIndent {
			minIndent = n
		}
	}
	if minIndent <= 0 {
		return s
	}
	for _, l := range lines {
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "***") || len(l) < minIndent {
			continue
		}
		switch t := l[minIndent:]; {
		case strings.HasPrefix(t, "@@"), strings.HasPrefix(t, "+"), strings.HasPrefix(t, "-"):
			revealed = true
		}
	}
	if !revealed {
		return s
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if !strings.HasPrefix(l, "***") && len(l) >= minIndent && strings.TrimSpace(l) != "" {
			out[i] = l[minIndent:]
		} else {
			out[i] = l
		}
	}
	return strings.Join(out, "\n")
}

// shearTrailingEnvelopeDebris drops a MULTI-LINE envelope tail that a partially
// unwrapped nested tool call leaves after the real patch (live: a doubly-nested
// envelope decoded into `…\n}\n"\n }\n}"},"name":"apply_patch` — the bare `"`
// and brace lines became context lines no file can ever match, failing every
// retry of an otherwise-correct hunk). Only runs when the unmistakable junk
// signature is present; then the signature line and the contiguous run of pure
// JSON-debris lines above it (quotes/braces/commas only) are cut. Over-shearing
// a real trailing `}` context line is accepted: a shorter context still anchors,
// junk context is a guaranteed failure.
func shearTrailingEnvelopeDebris(s string) string {
	if !strings.Contains(s, `"name":"apply_patch`) && !strings.Contains(s, `\"name\":\"apply_patch`) {
		return s
	}
	lines := strings.Split(s, "\n")
	sig := -1
	for i, l := range lines {
		if strings.Contains(l, `"name":"apply_patch`) || strings.Contains(l, `\"name\":\"apply_patch`) {
			sig = i
		}
	}
	// The signature must be tail junk, not patch content: everything after it
	// must be debris or blank.
	for _, l := range lines[sig+1:] {
		if strings.TrimSpace(l) != "" && !isEnvelopeDebrisLine(l) {
			return s
		}
	}
	cut := sig
	for cut > 0 && isEnvelopeDebrisLine(lines[cut-1]) {
		cut--
	}
	return strings.Join(lines[:cut], "\n")
}

// isEnvelopeDebrisLine reports whether the line consists solely of JSON
// envelope characters — quotes, braces, brackets, commas, backslashes.
func isEnvelopeDebrisLine(l string) bool {
	t := strings.TrimSpace(l)
	if t == "" {
		return false
	}
	for _, r := range t {
		if !strings.ContainsRune(`"'{}[],\`+"`", r) {
			return false
		}
	}
	return true
}

// decodeHTMLUnicodeEscapes reverses Go's json.Marshal HTML escaping (< > & rendered
// as the < > & escapes). Those never occur literally in a patch, so
// decoding them is safe. The backslash is built from its byte value to avoid an
// escaped literal in this source.
func decodeHTMLUnicodeEscapes(s string) string {
	bs := string(rune(92)) // a single backslash (ASCII 92)
	return strings.NewReplacer(
		bs+"u003c", "<", bs+"u003C", "<",
		bs+"u003e", ">", bs+"u003E", ">",
		bs+"u0026", "&",
	).Replace(s)
}

// truncateForMessage shortens a quoted line so an error stays readable when the
// model pasted a paragraph where a header belongs.
func truncateForMessage(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}

// ---- parser (ported from apply-patch.ts) ----

func parsePatch(text string) ([]patchHunk, error) {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	// Tolerate a patch not wrapped in Begin/End (small models drop the markers).
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == beginPatchMarker {
		lines = lines[1:]
	}
	// Strip a trailing End marker independently of Begin — and tolerate a +/- the
	// model stamps onto it ("+*** End Patch"), which would otherwise be parsed as a
	// bogus trailing hunk.
	if n := len(lines); n > 0 {
		last := strings.TrimSpace(lines[n-1])
		if strings.HasPrefix(last, "+") || strings.HasPrefix(last, "-") {
			last = last[1:]
		}
		if last == endPatchMarker {
			lines = lines[:n-1]
		}
	}
	// A model narrates before it patches. That preamble is prose in front of a
	// perfectly good patch, not a malformed header — measured live 2026-08-30 as
	// `invalid hunk header "The Todo_write tool is broken / unavailable, so I'll
	// skip it..."`. Drop everything before the first file section, and only when
	// there IS one: a patch that starts with a header still reports its own
	// errors, and prose with no patch at all still fails, by the rule below.
	if n := firstFileSection(lines); n > 0 {
		lines = lines[n:]
	}

	var hunks []patchHunk
	i := 0
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		h, consumed, err := parseOneHunk(lines[i:])
		if err != nil {
			return nil, err
		}
		hunks = append(hunks, h)
		i += consumed
	}
	if len(hunks) == 0 {
		// "patch contains no file sections" described the PARSER's state, and the
		// model cannot act on it. When this fires the patch text is blank, which
		// in practice means the argument never arrived — so say that, and say
		// where it goes. Measured live: three of these in one turn while the
		// model's patch was correct and the harness was dropping it.
		return nil, fmt.Errorf("apply_patch: the patch was empty — no patch text reached the tool. " +
			"Put the whole patch in the `input` parameter, starting with a file section header: " +
			"`*** Update File: <path>` (or `*** Add File:` / `*** Delete File:`)")
	}
	return hunks, nil
}

// firstFileSection returns the index of the first line that opens a file
// section, or -1 when the text contains none.
func firstFileSection(lines []string) int {
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, addFileMarker) ||
			strings.HasPrefix(t, updateFileMarker) ||
			strings.HasPrefix(t, deleteFileMarker) {
			return i
		}
	}
	return -1
}

// cleanHunkPath strips the decoration a small model stamps onto a header's
// filename. Live 2026-09-01: a small model wrote "*** Add File: wordcount.go ***" —
// mirroring the marker's own bracketed style — and the patch created a file
// literally named "wordcount.go ***". Every later `go run wordcount.go`
// failed with "no such file", the model re-patched the same wrong name, and
// the turn spiraled. Trailing "*" runs are never part of a real source path,
// and neither are wrapping quotes/backticks ("`main.go`").
func cleanHunkPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, "* \t")
	p = strings.Trim(p, "`'\"")
	return strings.TrimSpace(p)
}

func parseOneHunk(lines []string) (patchHunk, int, error) {
	first := strings.TrimSpace(lines[0])
	switch {
	case strings.HasPrefix(first, addFileMarker):
		path := cleanHunkPath(first[len(addFileMarker):])
		var b strings.Builder
		consumed := 1
		body := lines[1:]
		// Forgiving: a small model often writes the file content raw (no "+" prefix,
		// sometimes after a blank line) instead of the strict "+"-per-line form. Take
		// every line up to the next "***" section marker, stripping an optional "+".
		for len(body) > 0 && body[0] == "" { // skip a leading blank the model adds
			body = body[1:]
			consumed++
		}
		for _, l := range body {
			// Strip the "+" add-marker(s) FIRST, then check for a "***" section marker
			// — a small model routinely stamps the +/- prefix onto the marker line too
			// ("+*** End Patch"), and checking "***" before stripping let that end
			// marker slip through and land as file content (a syntax error). It also
			// STACKS the marker across lines ("+", "++", "+++" going down the file), so
			// strip the whole leading run, not just one — otherwise the extra "+"s leak
			// in as source and every line is a syntax error.
			if strings.HasPrefix(l, "+") {
				l = strings.TrimLeft(l, "+")
			}
			if strings.HasPrefix(l, "***") {
				break
			}
			b.WriteString(l)
			b.WriteString("\n")
			consumed++
		}
		return patchHunk{kind: "add", path: path, contents: stripLeadingLangTag(b.String())}, consumed, nil

	case strings.HasPrefix(first, deleteFileMarker):
		return patchHunk{kind: "delete", path: cleanHunkPath(first[len(deleteFileMarker):])}, 1, nil

	case strings.HasPrefix(first, updateFileMarker):
		path := cleanHunkPath(first[len(updateFileMarker):])
		rest := lines[1:]
		consumed := 1
		var movePath string
		if len(rest) > 0 && strings.HasPrefix(strings.TrimSpace(rest[0]), moveToMarker) {
			movePath = cleanHunkPath(strings.TrimSpace(rest[0])[len(moveToMarker):])
			rest = rest[1:]
			consumed++
		}
		var chunks []patchChunk
		// SEARCH/REPLACE is the simplest edit a small model can express reliably — the
		// exact old text and the new text, no "-"/"+" markers to forget and no context
		// lines to mismatch. Prefer it whenever the body uses the markers; otherwise fall
		// back to the classic "-"/"+" diff hunk.
		if looksLikeSearchReplace(rest) {
			srChunks, used, err := parseSearchReplace(rest)
			if err != nil {
				return patchHunk{}, 0, err
			}
			chunks = srChunks
			consumed += used
		} else {
			for len(rest) > 0 {
				if strings.TrimSpace(rest[0]) == "" {
					rest = rest[1:]
					consumed++
					continue
				}
				if strings.HasPrefix(rest[0], "***") {
					break
				}
				ch, used, err := parseUpdateChunk(rest, len(chunks) == 0)
				if err != nil {
					return patchHunk{}, 0, err
				}
				chunks = append(chunks, ch)
				rest = rest[used:]
				consumed += used
			}
		}
		if len(chunks) == 0 {
			return patchHunk{}, 0, fmt.Errorf("apply_patch: update hunk for %q is empty", path)
		}
		return patchHunk{kind: "update", path: path, movePath: movePath, chunks: chunks}, consumed, nil
	}
	// Reached only when the text has NO file section anywhere — a preamble in
	// front of a real patch is skipped before we get here. So the fault is the
	// missing header, not the line being quoted: name what is absent and show
	// the three forms, rather than echoing the model's prose back as a header.
	return patchHunk{}, 0, fmt.Errorf("apply_patch: no file section header found — a patch must contain "+
		"`*** Add File: <path>`, `*** Update File: <path>` or `*** Delete File: <path>`. "+
		"The text began: %q", truncateForMessage(lines[0], 80))
}

// ---- SEARCH/REPLACE edit form ----
//
// The most forgiving way to change a line: the model gives the exact old text and
// the new text, and the tool locates the old text (fuzzily) and swaps it. No diff
// markers, no line numbers, no surrounding context to get wrong.
//
//	*** Update File: example.go
//	<<<<<<< SEARCH
//	)}
//	=======
//	)
//	>>>>>>> REPLACE
//
// Multiple blocks per file are allowed. An empty SEARCH appends the REPLACE text.
// Marker matching is loose (a small model mangles the punctuation): any "<"-led line
// mentioning SEARCH opens a block, a run of "=" divides old from new, and a ">"-led
// line mentioning REPLACE closes it.

func isSearchMarker(t string) bool {
	return strings.HasPrefix(t, "<<<<<<<") ||
		(strings.HasPrefix(t, "<<") && strings.Contains(strings.ToUpper(t), "SEARCH"))
}

func isReplaceMarker(t string) bool {
	return strings.HasPrefix(t, ">>>>>>>") ||
		(strings.HasPrefix(t, ">>") && strings.Contains(strings.ToUpper(t), "REPLACE"))
}

// isDivider reports a line made only of "=" (the SEARCH/REPLACE separator).
func isDivider(t string) bool {
	return len(t) >= 3 && strings.Trim(t, "=") == ""
}

// looksLikeSearchReplace scans an Update body (up to the next file section) for a
// SEARCH marker, choosing the SEARCH/REPLACE parser over the "-"/"+" diff parser.
func looksLikeSearchReplace(lines []string) bool {
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "***") {
			break
		}
		if isSearchMarker(t) {
			return true
		}
	}
	return false
}

// stripUniformDiffMarker removes a leading "+" or "-" from every line when ALL
// non-empty lines share it — the diff-marker habit a small model brings to
// SEARCH/REPLACE. It never strips a leading space (that's real indentation), and it
// leaves the block untouched unless the marker is uniform, so ordinary code that
// happens to start one line with "+"/"-" is safe.
func stripUniformDiffMarker(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	for _, m := range []byte{'+', '-'} {
		uniform, any := true, false
		for _, l := range lines {
			if l == "" {
				continue
			}
			any = true
			if l[0] != m {
				uniform = false
				break
			}
		}
		if any && uniform {
			out := make([]string, len(lines))
			for i, l := range lines {
				if l != "" {
					l = l[1:]
				}
				out[i] = l
			}
			return out
		}
	}
	return lines
}

// parseSearchReplace turns SEARCH/REPLACE blocks into old/new chunks. Lines between
// blocks (blanks, stray prose) are ignored, so a sloppy emission still parses.
func parseSearchReplace(lines []string) ([]patchChunk, int, error) {
	var chunks []patchChunk
	i := 0
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "***") {
			break
		}
		if !isSearchMarker(t) {
			i++ // skip anything before the first/next block
			continue
		}
		i++ // consume the SEARCH marker
		var search, replace []string
		for i < len(lines) {
			t = strings.TrimSpace(lines[i])
			if isDivider(t) || isReplaceMarker(t) || strings.HasPrefix(t, "***") {
				break
			}
			search = append(search, lines[i])
			i++
		}
		if i < len(lines) && isDivider(strings.TrimSpace(lines[i])) {
			i++ // consume the "=======" divider
		}
		for i < len(lines) {
			t = strings.TrimSpace(lines[i])
			if isReplaceMarker(t) || isSearchMarker(t) || strings.HasPrefix(t, "***") {
				break
			}
			replace = append(replace, lines[i])
			i++
		}
		if i < len(lines) && isReplaceMarker(strings.TrimSpace(lines[i])) {
			i++ // consume the REPLACE marker
		}
		// Keep the RAW lines. A small model may carry its diff habit into
		// SEARCH/REPLACE (a uniform leading "+"/"-"), but so does a real Markdown
		// bullet or YAML sequence — so the diff-marker strip is deferred to match
		// time (computeReplacements): try the raw block first, strip only if it
		// fails to find the text. That fixes the habit without corrupting content
		// that legitimately starts every line with "-"/"+".
		chunks = append(chunks, patchChunk{
			oldLines:             search,
			newLines:             replace,
			markerStripCandidate: true,
		})
	}
	if len(chunks) == 0 {
		return nil, 0, fmt.Errorf("apply_patch: no SEARCH/REPLACE blocks found")
	}
	return chunks, i, nil
}

// parseUpdateChunk parses ONE hunk of an Update body — OpenClaw's design
// (apply-patch-update.ts parseUpdateFileChunk): an optional "@@ <context>" anchor,
// then a run of context (" "), removed ("-"), and added ("+") lines. It STOPS at the
// next "@@" (or "***"/EOF), so the caller loops it once per hunk — each hunk is
// anchored with a forward-seek (computeReplacements) and applied INDEPENDENTLY, which
// is what makes multi-location edits and true "@@" anchoring work. The leading "@@"
// may be omitted only on the FIRST hunk (allowMissingContext).
//
// Two small-model tolerances kept from memdoor's forgiving parser: a stray "@@"
// stamped inside a "-"/"+" line's content is stripped ("-@@ x" → remove "x"), and a
// bare line with no marker is treated as context (a small model routinely drops the leading
// space on a context line, e.g. a lone "}").
func parseUpdateChunk(lines []string, allowMissingContext bool) (patchChunk, int, error) {
	ch := patchChunk{}
	start := 0
	switch first := lines[0]; {
	case first == emptyContextMarker: // bare "@@"
		start = 1
	case strings.HasPrefix(first, changeContextMarker): // "@@ <context>"
		// Git-style coordinates with TRAILING context ("@@ -3,3 +3,4 @@ type Piece
		// struct {") strip down to the context, which anchors as usual — left in,
		// the coords never match and the append recovery lands them as literal
		// file content (live: "expected declaration, found '-'").
		ch.changeContext = strings.TrimSpace(stripUnifiedCoords(strings.TrimSpace(first[len(changeContextMarker):])))
		// The model sometimes DOUBLES the marker ("@@ @@ main() {"): the anchor then
		// carries a literal "@@" that can never match a code line (and, via the
		// append recoveries, could land a literal "@" in the file). Strip repeated
		// markers until the anchor is bare code.
		for strings.HasPrefix(ch.changeContext, emptyContextMarker) {
			ch.changeContext = strings.TrimSpace(stripAtAt(ch.changeContext))
		}
		ch.hasContext = true
		// A model often emits a git-style unified-diff header ("@@ -1,1 +1,1 @@")
		// instead of our "@@ <nearby code line>" anchor. Its line-number ranges match
		// nothing in the file, so treat it as NO anchor (match by content) rather than
		// hunting for a bogus "1,1 @@" line and failing every edit.
		if isDiffLineHeader(ch.changeContext) {
			ch.changeContext = ""
			ch.hasContext = false
		}
		// Same for a compiler-style "file.go:12" reference (live: "@@ tetris.go:10"
		// looping every retry): a path:line token is never a code line, so it can
		// never anchor — drop it and match by content.
		if isFileLineRef(ch.changeContext) {
			ch.changeContext = ""
			ch.hasContext = false
		}
		start = 1
	case !allowMissingContext:
		return patchChunk{}, 0, fmt.Errorf("apply_patch: update hunk must start with a @@ context marker, got %q", first)
	}

	parsed := 0
	for _, line := range lines[start:] {
		switch {
		case line == eofMarker:
			ch.isEndOfFile = true
			parsed++
			return ch, start + parsed, nil
		case strings.HasPrefix(line, "***"):
			return ch, start + parsed, nil // next file section ends the hunk
		case line == emptyContextMarker || strings.HasPrefix(line, changeContextMarker):
			if parsed == 0 {
				return patchChunk{}, 0, fmt.Errorf("apply_patch: empty update hunk")
			}
			return ch, start + parsed, nil // a new "@@" anchor ends this hunk
		case line == "":
			ch.oldLines = append(ch.oldLines, "")
			ch.newLines = append(ch.newLines, "")
			ch.contextOld = append(ch.contextOld, len(ch.oldLines)-1)
		case strings.HasPrefix(line, "-"):
			c := stripAtAt(line[1:])
			ch.oldLines = append(ch.oldLines, c)
			ch.removedLines = append(ch.removedLines, c)
			ch.hasRemoval = true
		case strings.HasPrefix(line, "+"):
			c := stripAtAt(line[1:])
			ch.newLines = append(ch.newLines, c)
			ch.addedLines = append(ch.addedLines, c)
			ch.contextOld = append(ch.contextOld, -1)
		case strings.HasPrefix(line, " "):
			c := line[1:]
			// Git-diff column habit: the model writes its add-lines as " +…" (marker in
			// column 2, like a diff-of-diff), which reads as a context line whose content
			// begins with "+" — and that literal "+" would land IN the file. A code line
			// never starts with "+", so reclassify space-then-plus as an ADD.
			if t := strings.TrimLeft(c, " \t"); strings.HasPrefix(t, "+") {
				a := stripAtAt(strings.TrimPrefix(t, "+"))
				ch.newLines = append(ch.newLines, a)
				ch.addedLines = append(ch.addedLines, a)
				ch.contextOld = append(ch.contextOld, -1)
				parsed++
				continue
			} else if strings.HasPrefix(t, "-") && hunkHasIndentedPlus(lines[start:]) {
				// Indented "-" is reclassified as a REMOVAL only when the same hunk
				// also carries indented "+" markers — that shape is a real diff with
				// shifted columns (live: "    -    fmt.Println…" beside "    +…"),
				// while a lone " - item" stays context (YAML/markdown lists).
				r := stripAtAt(strings.TrimPrefix(t, "-"))
				ch.oldLines = append(ch.oldLines, r)
				ch.removedLines = append(ch.removedLines, r)
				ch.hasRemoval = true
				parsed++
				continue
			}
			ch.oldLines = append(ch.oldLines, c)
			ch.newLines = append(ch.newLines, c)
			ch.contextOld = append(ch.contextOld, len(ch.oldLines)-1)
		default: // bare context line (small-model tolerance)
			ch.oldLines = append(ch.oldLines, line)
			ch.newLines = append(ch.newLines, line)
			ch.contextOld = append(ch.contextOld, len(ch.oldLines)-1)
		}
		parsed++
	}
	return ch, start + parsed, nil
}

// stripLeadingLangTag drops a leaked markdown fence language tag ("golang", "python",
// …) that a small model sometimes writes as the FIRST content line of a new file
// (from wrapping the code in a ```golang fence). A bare language word is never valid
// source in that language, so removing it is safe and keeps the created file compilable.
func stripLeadingLangTag(s string) string {
	nl := strings.IndexByte(s, '\n')
	if nl < 0 {
		return s
	}
	switch strings.ToLower(strings.TrimSpace(s[:nl])) {
	case "golang", "go", "python", "py", "javascript", "js", "typescript", "ts",
		"rust", "rs", "java", "c", "cpp", "c++", "csharp", "cs", "ruby", "rb",
		"bash", "sh", "shell", "console", "json", "yaml", "yml", "toml", "html", "css":
		return s[nl+1:]
	}
	return s
}

// isDiffLineHeader reports whether an "@@" context is really a git unified-diff hunk
// header ("-1,1 +1,1 @@", "1,1 @@", "-3,7 +3,8") — line-number ranges, not a code
// anchor. A real anchor is a line of source (it has letters); a diff header has digits
// and range punctuation and NO letters.
func isDiffLineHeader(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	hasDigit := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return false // real code anchors contain letters
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
	}
	return hasDigit && strings.ContainsAny(s, ",@+-")
}

// isFileLineRef reports whether an "@@" context is a compiler-style file:line
// reference ("tetris.go:10", "pkg/game/board.py:42") rather than a code anchor.
// The shape is a single path token with an extension, a colon, and only digits
// after it — no spaces, so it can never be a real source line.
func isFileLineRef(s string) bool {
	s = strings.TrimSpace(s)
	colon := strings.LastIndexByte(s, ':')
	if colon <= 0 || colon == len(s)-1 || strings.ContainsAny(s, " \t") {
		return false
	}
	for _, c := range s[colon+1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	name := s[:colon]
	dot := strings.LastIndexByte(name, '.')
	return dot > 0 && dot < len(name)-1 // has an extension → a filename, not code
}

// hunkHasIndentedPlus reports whether the hunk (up to its terminator) contains a
// space-indented "+" marker line — the signal that indented "-" lines in the same
// hunk are shifted diff markers, not list-item content.
func hunkHasIndentedPlus(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, "***") || l == emptyContextMarker || strings.HasPrefix(l, changeContextMarker) {
			if strings.HasPrefix(l, "***") {
				break
			}
			continue
		}
		if strings.HasPrefix(l, " ") {
			if t := strings.TrimLeft(l, " \t"); strings.HasPrefix(t, "+") {
				return true
			}
		}
	}
	return false
}

// stripAtAt removes a leading "@@ " / "@@" that a small model stamps on hunk lines.
func stripAtAt(s string) string {
	switch {
	case strings.HasPrefix(s, changeContextMarker): // "@@ "
		return stripUnifiedCoords(s[len(changeContextMarker):])
	case s == emptyContextMarker: // "@@"
		return ""
	case strings.HasPrefix(s, emptyContextMarker): // "@@" with no following space
		return stripUnifiedCoords(s[len(emptyContextMarker):])
	default:
		return s
	}
}

// unifiedCoordsRe matches git-style hunk coordinates the model copies from
// real diffs: "-10,7 +10,8 @@" (live: the whole thing landed in the anchor and
// the '-10,7' half was then parsed as a removal line of garbage).
var unifiedCoordsRe = regexp.MustCompile(`^\s*-\d+(?:,\d+)?\s+\+\d+(?:,\d+)?\s*(?:@@\s*)?`)

// stripUnifiedCoords removes unified-diff coordinates from an @@ anchor,
// keeping the trailing context ("type Piece struct {") that actually anchors.
func stripUnifiedCoords(s string) string {
	return unifiedCoordsRe.ReplaceAllString(s, "")
}

// ---- applier (ported from apply-patch-update.ts) ----

func applyUpdateHunk(path string, chunks []patchChunk) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file to update: %w", err)
	}
	orig, layout := splitFileLines(string(data))
	repls, err := computeReplacements(orig, path, chunks)
	if err != nil {
		return "", err
	}
	out := applyReplacements(orig, repls)
	// A patch that ends the file with an empty line writes the final newline.
	if layout.finalNewline && len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return layout.join(out), nil
}

// fileLayout is how a file's lines are written back: its line ending and
// whether it ends with one. An edit keeps both: a patch's lines arrive with
// "\n" and no final-newline intent, and writing them as they came turned a
// CRLF file into mixed endings and gave a final newline to a file that had
// none (the same class as openclaw#124390).
type fileLayout struct {
	bom          bool // starts with a UTF-8 byte-order mark
	crlf         bool // every line ends with "\r\n"
	finalNewline bool
}

// splitFileLines is text's lines as edits see them — without a byte-order
// mark or the "\r" of a CRLF file, so the patch's lines match and splice as
// plain lines — and the layout that writes them back. A file with mixed
// endings keeps its "\r"s in the lines, as they are.
func splitFileLines(text string) ([]string, fileLayout) {
	var layout fileLayout
	text, layout.bom = strings.CutPrefix(text, utf8BOM)
	layout.finalNewline = text == "" || strings.HasSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	ended := len(lines) - 1 // lines followed by "\n"
	if layout.finalNewline {
		ended = len(lines)
	}
	layout.crlf = ended > 0
	for _, l := range lines[:max(ended, 0)] {
		if !strings.HasSuffix(l, "\r") {
			layout.crlf = false
			break
		}
	}
	if layout.crlf {
		for i := range lines[:ended] {
			lines[i] = strings.TrimSuffix(lines[i], "\r")
		}
	}
	return lines, layout
}

// join writes lines back in the layout.
func (l fileLayout) join(lines []string) string {
	sep := "\n"
	if l.crlf {
		sep = "\r\n"
	}
	out := strings.Join(lines, sep)
	if l.finalNewline {
		out += sep
	}
	if l.bom {
		out = utf8BOM + out
	}
	return out
}

const utf8BOM = "\ufeff"

// preserveContext splices back, as the FILE has them, the lines a hunk
// leaves unchanged. The matcher forgives whitespace (seekSequence's TrimSpace
// tiers), so a hunk whose lines were re-typed one tab short still matches;
// splicing the model's copy then re-indented unchanged code (live 2026-09-30:
// an `if` block moved out of its scope, the model "fixed" the stray brace and
// broke the file).
//
//   - A context line is the file's line.
//   - An added line identical to a removed line as typed ("-  if x {" then
//     "+  if x {") is that line unchanged: the file's line.
//   - Any other added line is the edit. When every line the model typed from
//     the file was off by the same leading whitespace, it is shifted by it,
//     so new code lands at the file's depth; otherwise it stays as written
//     (a re-indent is an edit and keeps its depth).
//
// It applies only when consumed lines up one to one with the hunk's old
// lines (the seekSequence match), newSlice is the hunk's own new lines, and
// every old line is its file line once whitespace is ignored.
func preserveContext(consumed, newSlice []string, ch patchChunk) []string {
	if len(ch.contextOld) != len(newSlice) || len(consumed) != len(ch.oldLines) || !slices.Equal(newSlice, ch.newLines) {
		return newSlice
	}
	type pair struct{ file, model string }
	var pairs []pair
	for i, typed := range ch.oldLines {
		if normalizePunct(strings.TrimSpace(consumed[i])) != normalizePunct(strings.TrimSpace(typed)) {
			return newSlice
		}
		if strings.TrimSpace(typed) != "" {
			pairs = append(pairs, pair{leadingWS(consumed[i]), leadingWS(typed)})
		}
	}
	shift := func(l string) string { return l }
	if d, ok := uniformOffset(pairs, func(p pair) (string, string) { return p.file, p.model }); ok {
		shift = func(l string) string { return d + l }
	} else if d, ok := uniformOffset(pairs, func(p pair) (string, string) { return p.model, p.file }); ok {
		shift = func(l string) string { return strings.TrimPrefix(l, d) }
	}
	isContext := make([]bool, len(ch.oldLines))
	for _, i := range ch.contextOld {
		if i >= 0 {
			isContext[i] = true
		}
	}
	out := make([]string, len(newSlice))
	for j, i := range ch.contextOld {
		switch {
		case i >= 0:
			out[j] = consumed[i]
		case strings.TrimSpace(newSlice[j]) == "":
			out[j] = newSlice[j]
		default:
			out[j] = shift(newSlice[j])
			for k, typed := range ch.oldLines {
				if !isContext[k] && typed == newSlice[j] {
					out[j] = consumed[k]
					isContext[k] = true // each removed line stands for one added line
					break
				}
			}
		}
	}
	return out
}

// uniformOffset is the non-empty whitespace d with long == d+short for every
// pair, if there is one.
func uniformOffset[P any](pairs []P, split func(P) (long, short string)) (string, bool) {
	if len(pairs) == 0 {
		return "", false
	}
	long, short := split(pairs[0])
	d, ok := strings.CutSuffix(long, short)
	if !ok || d == "" {
		return "", false
	}
	for _, p := range pairs[1:] {
		if l, s := split(p); l != d+s {
			return "", false
		}
	}
	return d, true
}

// leadingWS is l's leading spaces and tabs.
func leadingWS(l string) string {
	return l[:len(l)-len(strings.TrimLeft(l, " \t"))]
}

type replacement struct {
	start    int
	oldLen   int
	newLines []string
}

// relocateTrailingAdds moves statement additions that a hunk appends AFTER a
// closing-brace context line to BEFORE it (live: "@@ func main() { / ctx
// println / ctx } / +println(new)" — the splice lands the new statement
// outside the function, "expected declaration", and the turn dies in the
// refusal loop). Fires only when the consumed block is reproduced verbatim as
// the head of the new block, the last consumed line is a bare brace, and the
// trailing additions are plain statements (a top-level declaration after a
// brace is legitimate and stays put).
func relocateTrailingAdds(consumed, newSlice []string) []string {
	n := len(consumed)
	if len(newSlice) <= n || n == 0 {
		return newSlice
	}
	// The head must reproduce the consumed block — compared whitespace-
	// normalized, since the match itself may have landed on a TrimSpace tier
	// (model spaces vs file tabs).
	for i := range consumed {
		if strings.TrimSpace(newSlice[i]) != strings.TrimSpace(consumed[i]) {
			return newSlice
		}
	}
	last := strings.TrimSpace(consumed[n-1])
	if last != "}" && last != "};" {
		return newSlice
	}
	adds := newSlice[n:]
	if addsTopLevelDecl(adds) {
		return newSlice
	}
	out := make([]string, 0, len(newSlice))
	out = append(out, consumed[:n-1]...)
	out = append(out, adds...)
	out = append(out, consumed[n-1])
	return out
}

// groundingHint returns the file's ACTUAL current lines to append to a match
// failure. A small model edits from its (lossy) memory of what it wrote earlier
// in the turn — so it invents context lines that don't exist (e.g. re-adding a
// return to a func that already has one) and the match fails. Handing it the
// real bytes on the failed result turns the retry into a grounded edit against
// truth instead of another guess. Capped so a large file can't blow the context.
func groundingHint(orig []string) string {
	const maxLines, maxBytes = 200, 8000
	var b strings.Builder
	b.WriteString("\n\nThe file's ACTUAL current content is below. Copy your context lines from THIS exact text — do not edit from memory:\n")
	for i, ln := range orig {
		if i >= maxLines || b.Len() >= maxBytes {
			b.WriteString("... (truncated; read_file for the rest)\n")
			break
		}
		fmt.Fprintf(&b, "%d\t%s\n", i+1, ln)
	}
	return b.String()
}

func computeReplacements(orig []string, path string, chunks []patchChunk) ([]replacement, error) {
	var repls []replacement
	lineIndex := 0
	for _, ch := range chunks {
		// A hunk with NO "+" and NO "-" lines is a pure LOCATOR — it changes
		// nothing by definition. Skipping it (instead of letting the recovery
		// paths reinterpret its context echo as new code) makes a context-only
		// patch a clean no-op, which the no-op refusal then reports with the
		// -/+ hunk shape spelled out.
		if !ch.hasRemoval && len(ch.addedLines) == 0 && len(ch.oldLines) > 0 &&
			slices.Equal(ch.oldLines, ch.newLines) {
			continue
		}
		if ch.hasContext {
			ctxIdx := seekSequence(orig, []string{ch.changeContext}, lineIndex, false)
			if ctxIdx < 0 {
				// Not found ahead of the previous hunk. A small model emits hunks OUT
				// of file order (e.g. an edit to main() before one to a func defined
				// earlier), so retry the anchor across the WHOLE file — each "@@" hunk
				// locates independently.
				ctxIdx = seekSequence(orig, []string{ch.changeContext}, 0, false)
			}
			if ctxIdx < 0 {
				// PARTIAL anchor: the model wrote a fragment of the line ("main() {"
				// for "func main() {"). If the fragment appears as a substring of
				// exactly the line a human would mean (the first containing line),
				// use that line as the anchor rather than failing.
				ctxIdx = anchorSubstringIndex(orig, ch.changeContext)
			}
			if ctxIdx < 0 {
				// The anchor line is nowhere in the file. A small model expresses "add
				// a NEW declaration" by putting the declaration's own first line on the
				// "@@" marker (which therefore cannot match) and its body as "+" lines.
				// Recover by APPENDING the reconstructed block instead of failing.
				// Only reconstruct-and-append for a GENUINELY new declaration. When the
				// "@@" anchor already matches an existing line (the model mangled it —
				// e.g. "main() {" for "func main() {"), appending changeContext+newLines
				// would duplicate the existing block (a stray "main() {" clone). Refuse,
				// so the patch fails cleanly with an explicit error the model can retry
				// against, rather than corrupting the file.
				if block := newDeclarationAppend(ch); block != nil && !anchorMatchesExisting(orig, ch.changeContext) {
					ins := len(orig)
					if len(orig) > 0 && orig[len(orig)-1] == "" {
						ins = len(orig) - 1
					}
					repls = append(repls, replacement{ins, 0, block})
					continue
				}
				// "Add an import" that anchored on "import (" a single-line-import file
				// doesn't have (or vice-versa): merge into the real import section.
				if r, ok := importAddReplacement(orig, ch); ok {
					repls = append(repls, r)
					continue
				}
				// "Add a function" the model mangled — anchored on an existing line but the
				// hunk carries a whole new top-level func/type. Append it.
				if r, ok := newFuncFromHunk(orig, ch); ok {
					repls = append(repls, r)
					continue
				}
				return nil, fmt.Errorf("failed to find context %q in %s%s", ch.changeContext, path, groundingHint(orig))
			}
			lineIndex = ctxIdx + 1
		}
		if len(ch.oldLines) == 0 {
			// A pure-insert hunk WITH a matched "@@" anchor belongs right AFTER the
			// anchor — lineIndex already points there (ctxIdx+1). Appending it at EOF
			// (the old behavior) drops a statement outside every function ("expected
			// declaration, found fmt"), which the syntax guard then rightly refuses —
			// so an anchored insert like `@@ fmt.Println(...) / +fmt.Println(...)`
			// could never land. Only an UNANCHORED pure insert goes to EOF —
			// EXCEPT when the added content IS a top-level declaration (func/type):
			// the anchor is usually a line inside some function, and splicing a
			// declaration mid-body breaks the file. Declarations append at EOF
			// (supersede rules then replace any old version).
			if ch.hasContext && lineIndex > 0 && !addsTopLevelDecl(ch.newLines) {
				repls = append(repls, replacement{lineIndex, 0, ch.newLines})
				continue
			}
			ins := len(orig)
			if len(orig) > 0 && orig[len(orig)-1] == "" {
				ins = len(orig) - 1
			}
			repls = append(repls, replacement{ins, 0, ch.newLines})
			continue
		}
		// A context+add hunk whose ADDED lines are top-level declarations: the
		// context anchors it wherever the model happened to echo (live: inside a
		// const block — "expected 'IDENT', found 'const'"), but a declaration can
		// NEVER be spliced into a nested block. Route the decls to EOF and leave
		// the context untouched; the supersede rules replace any old version.
		if len(ch.removedLines) == 0 && len(ch.addedLines) > 0 && addsTopLevelDecl(ch.addedLines) {
			ins := len(orig)
			if len(orig) > 0 && orig[len(orig)-1] == "" {
				ins = len(orig) - 1
			}
			repls = append(repls, replacement{ins, 0, ch.addedLines})
			continue
		}
		// A pure-add hunk whose "+" line is the alnum-twin of a context line is really a
		// one-line CHANGE the model mis-wrote as context+add — intercept BEFORE the
		// literal match, which would splice a stray near-duplicate (a statement after a
		// "}") that won't compile.
		if r, ok := nearDuplicateAddReplacement(orig, ch); ok {
			repls = append(repls, r)
			lineIndex = r.start + r.oldLen
			continue
		}
		// Match the RAW block first so content that legitimately starts every line
		// with "-"/"+" (Markdown bullets, YAML sequences) is found as-is.
		found, oldLen, newSlice := matchPattern(orig, ch.oldLines, ch.newLines, lineIndex, ch.isEndOfFile)
		if found < 0 && ch.markerStripCandidate {
			// Raw didn't match — retry with a uniform leading "+"/"-" stripped (the
			// small model's diff-marker habit). Only when stripping changes the
			// block, so a real bullet list already had its shot at a raw match.
			if sp := stripUniformDiffMarker(ch.oldLines); !slices.Equal(sp, ch.oldLines) {
				found, oldLen, newSlice = matchPattern(orig, sp, stripUniformDiffMarker(ch.newLines), lineIndex, ch.isEndOfFile)
			}
		}
		if found < 0 {
			// OUT-OF-ORDER HUNKS: search from the TOP before giving up.
			//
			// Hunks are located forward-only from the previous match (seekSequence
			// starts at `from`), which assumes the model emits them in file order.
			// It does not always. Measured 2026-08-30 on a live coder turn: hunk 1
			// deleted main() at the END of the file, advancing past it; hunk 2's
			// context was `except Exception as exc:` from a function EARLIER in the
			// file, so the search began past it and the whole patch failed with
			// "failed to find expected lines" — on context that was letter-perfect
			// and right there in the file.
			//
			// A match inside a span already being replaced is refused: that text is
			// on its way out, and splicing two edits over each other corrupts both.
			if f, ol, ns := matchPattern(orig, ch.oldLines, ch.newLines, 0, ch.isEndOfFile); f >= 0 &&
				!overlapsReplacement(repls, f, ol) {
				repls = append(repls, replacement{f, ol, relocateTrailingAdds(orig[f:f+ol], preserveContext(orig[f:f+ol], ns, ch))})
				continue // lineIndex intentionally NOT advanced: we went backwards
			}
			// The hunk's own RESULT may already be in the file — a small model, fixing
			// a build error, re-sends a hunk it already applied (e.g. the call) bundled
			// with the new one (the function). Its old-context no longer matches (the
			// file already changed), but the change IS done, so skip it instead of
			// failing the WHOLE patch and losing the hunk that isn't applied yet.
			if hunkAlreadyApplied(orig, ch.newLines) {
				continue
			}
			// Same import-merge recovery for a no-anchor "add import" hunk.
			if r, ok := importAddReplacement(orig, ch); ok {
				repls = append(repls, r)
				continue
			}
			// Same "add a function" recovery: the hunk's old-context didn't match, but it
			// carries a complete new top-level func/type — append it rather than fail.
			if r, ok := newFuncFromHunk(orig, ch); ok {
				repls = append(repls, r)
				continue
			}
			// The model placed its real -/+ change AFTER spurious context (it echoed the
			// body and "}" then the actual change), so the full old-block can't match.
			// Retry as a MINIMAL change: find just the removed lines, swap in the added
			// ones. This absorbs the common "context-then-change" malformation on an
			// existing-file edit instead of failing it.
			if r, ok := minimalChangeReplacement(orig, ch); ok {
				repls = append(repls, r)
				lineIndex = r.start + r.oldLen
				continue
			}
			return nil, fmt.Errorf("failed to find expected lines in %s:\n%s%s", path, strings.Join(ch.oldLines, "\n"), groundingHint(orig))
		}
		// AMBIGUOUS, REFUSED — NOT THE FIRST MATCH (2026-09-30). A hunk with no
		// "@@" line whose lines also match further on used to land on the first
		// one: `-	return 1` meant for B() changed A(). The description now asks
		// for as little context as makes a hunk unique (models were re-typing
		// 7-17 unchanged lines a hunk), so a hunk that is not unique must say
		// where, not be guessed.
		if !ch.hasContext {
			if again, _, _ := matchPattern(orig, ch.oldLines, ch.newLines, found+1, ch.isEndOfFile); again >= 0 {
				return nil, fmt.Errorf("the lines to change in %s appear more than once (lines %d and %d): add the unchanged line just above them as context, or an @@ line with the first line of the enclosing function. Nothing was written", path, found+1, again+1)
			}
		}
		repls = append(repls, replacement{found, oldLen, relocateTrailingAdds(orig[found:found+oldLen], preserveContext(orig[found:found+oldLen], newSlice, ch))})
		lineIndex = found + oldLen
	}
	// SUPERSEDE, don't redeclare: when a hunk ADDS a complete top-level func/type
	// whose name the file already declares OUTSIDE the replaced spans, applying it
	// literally leaves two declarations ("half redeclared"). The model's intent is
	// the NEW definition replacing the old — so also delete the old decl block.
	repls = append(repls, supersededDeclDeletions(orig, repls)...)

	// applyReplacements splices back-to-front and relies on ascending start order;
	// out-of-order hunks (located independently above) can produce non-monotonic
	// starts, so sort before returning. Stable keeps same-start inserts in order.
	slices.SortStableFunc(repls, func(a, b replacement) int { return a.start - b.start })
	return repls, nil
}

// overlapsReplacement reports whether [start, start+length) intersects any span
// already scheduled for replacement. Two edits spliced over the same lines
// corrupt each other, so a backwards match into a claimed span is refused.
func overlapsReplacement(repls []replacement, start, length int) bool {
	for _, r := range repls {
		if start < r.start+r.oldLen && r.start < start+length {
			return true
		}
	}
	return false
}

// supersededDeclDeletions finds top-level func/type declarations INTRODUCED by the
// replacements whose name already exists in orig outside every replaced span, and
// returns deletions for those old blocks (decl line through its balanced closing
// brace). Only complete, brace-balanced added decls count — a context echo of the
// existing decl (present in orig within the span) is not an introduction.
func supersededDeclDeletions(orig []string, repls []replacement) []replacement {
	inSpan := func(line int) bool {
		for _, r := range repls {
			if line >= r.start && line < r.start+r.oldLen {
				return true
			}
		}
		return false
	}
	var dels []replacement
	seen := map[string]bool{}
	for _, r := range repls {
		for i, nl := range r.newLines {
			name := declName(nl)
			if name == "" || seen[name] || !braceBalanced(r.newLines[i:]) {
				continue
			}
			seen[name] = true
			// EVERY introduced decl gets the supersede check (a hunk adding both
			// sum and main must supersede an old main too — a break after the
			// first decl left "main redeclared" live). Context echoes of the decl
			// inside the replaced span itself are excluded by inSpan.
			for old, l := range orig {
				if declName(l) != name || inSpan(old) {
					continue
				}
				if end := declBlockEnd(orig, old); end > old {
					dels = append(dels, replacement{old, end - old, nil})
				}
				break
			}
		}
	}
	return dels
}

// addsTopLevelDecl reports whether the added lines contain a top-level func/type
// declaration — content that must be placed at file scope, never inside a body.
func addsTopLevelDecl(lines []string) bool {
	for _, l := range lines {
		if declName(l) != "" {
			return true
		}
	}
	return false
}

// contentEquivalent reports whether two file contents are the SAME code modulo
// whitespace (per-line, whitespace-collapsed). A patch whose only effect is
// re-indenting the matched lines (the fuzzy matcher adopting the model's spacing)
// changed nothing the user asked for.
func contentEquivalent(a, b string) bool {
	la := strings.Split(strings.TrimRight(a, "\n"), "\n")
	lb := strings.Split(strings.TrimRight(b, "\n"), "\n")
	if len(la) != len(lb) {
		return false
	}
	for i := range la {
		if collapseWS(la[i]) != collapseWS(lb[i]) {
			return false
		}
	}
	return true
}

// declBlockEnd returns the exclusive end line of the decl block starting at
// `start` — through its brace-balanced closing line — or start when no block
// opens (safety: nothing deleted).
func declBlockEnd(orig []string, start int) int {
	depth, opened := 0, false
	for i := start; i < len(orig); i++ {
		for _, r := range orig[i] {
			switch r {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
			}
		}
		if opened && depth == 0 {
			return i + 1
		}
	}
	return start
}

// importSpecRe matches a Go import spec line: an optional alias/underscore/dot then a
// quoted path — `"fmt"`, `_ "embed"`, `m "math"`, `. "x"`.
var importSpecRe = regexp.MustCompile(`^(?:([A-Za-z_.][\w]*)\s+)?("[^"]+")$`)

// importSpec returns the normalized import spec for a line if it is one, else false.
// It tolerates a leading "import " (a single-line form) and a trailing ")".
func importSpec(line string) (string, bool) {
	t := strings.TrimSpace(line)
	t = strings.TrimSpace(strings.TrimPrefix(t, "import"))
	t = strings.TrimSpace(strings.TrimSuffix(t, ")"))
	m := importSpecRe.FindStringSubmatch(t)
	if m == nil {
		return "", false
	}
	if m[1] != "" {
		return m[1] + " " + m[2], true
	}
	return m[2], true
}

// findImportSection locates the file's import section — a single-line `import "X"` or a
// block `import ( … )` — and returns its [start,end) line range and existing specs, or
// (-1,-1,nil) if there is none.
func findImportSection(orig []string) (start, end int, specs []string) {
	for i, l := range orig {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "import (") {
			for j := i + 1; j < len(orig); j++ {
				if strings.TrimSpace(orig[j]) == ")" {
					for k := i + 1; k < j; k++ {
						if s, ok := importSpec(orig[k]); ok {
							specs = append(specs, s)
						}
					}
					return i, j + 1, specs
				}
			}
			return -1, -1, nil // unterminated block
		}
		if strings.HasPrefix(t, "import ") {
			if s, ok := importSpec(t); ok {
				return i, i + 1, []string{s}
			}
		}
	}
	return -1, -1, nil
}

// importAddReplacement handles the common "add an import" edit a small model gets
// wrong: it anchors on "import (" but the file has a single-line `import "X"` (or the
// reverse), so the hunk never matches. When EVERY net-added line of the hunk is a Go
// import spec, merge those into the file's real import section (deduped), normalized to
// a block. Returns the replacement + true when it applies; false otherwise (so a hunk
// that isn't purely import additions is left to fail normally).
func importAddReplacement(orig []string, ch patchChunk) (replacement, bool) {
	var adds []string
	for _, nl := range ch.newLines {
		if strings.TrimSpace(nl) == "" || slices.Contains(ch.oldLines, nl) {
			continue // context / unchanged line
		}
		s, ok := importSpec(nl)
		if !ok {
			return replacement{}, false // a non-import addition — not an import-only hunk
		}
		adds = append(adds, s)
	}
	if len(adds) == 0 {
		return replacement{}, false
	}
	start, end, existing := findImportSection(orig)
	if start < 0 {
		return replacement{}, false
	}
	seen := map[string]bool{}
	block := []string{"import ("}
	for _, s := range append(existing, adds...) {
		if seen[s] {
			continue
		}
		seen[s] = true
		block = append(block, "\t"+s)
	}
	block = append(block, ")")
	return replacement{start: start, oldLen: end - start, newLines: block}, true
}

// newDeclarationAppend reconstructs a block to APPEND when a hunk's "@@" anchor
// names a line not present in the file. A small model, adding a new top-level
// declaration, routinely puts the declaration's OWN first line on the "@@" marker
// (e.g. "@@ func Peek() (int, bool) {") — which can never match, since that line
// does not exist yet — and the body as "+" lines. We take that anchor as the block's
// first line and return "@@ line + new lines" (with a leading blank), to append at
// EOF. Fires only when the anchor looks like a DECLARATION HEADER — trimmed, it ends
// with "{" or ":" (opens a block: Go/JS/Java/C funcs+types, Python def/class). That
// gate keeps it to the new-declaration case and off ordinary edit hunks whose anchor
// merely had a typo, and it TOLERATES the no-op "-x/+x" a small model puts in the
// body (which older logic rejected as a removal). Returns nil otherwise.
// hunkAlreadyApplied reports whether the hunk's RESULT (its new-side lines) already
// appears verbatim in the file — i.e. the edit was made on an earlier patch. Used to
// SKIP a redundant hunk a small model re-sends rather than fail the whole patch.
// Trailing blanks are ignored; an empty result never counts as applied.
func hunkAlreadyApplied(orig, newLines []string) bool {
	nl := newLines
	for len(nl) > 0 && strings.TrimSpace(nl[len(nl)-1]) == "" {
		nl = nl[:len(nl)-1]
	}
	if len(nl) == 0 {
		return false
	}
	return seekSequence(orig, nl, 0, false) >= 0
}

// anchorSubstringIndex finds the first file line CONTAINING the anchor fragment
// (whitespace-collapsed), for partial anchors like "main() {" meaning
// "func main() {". Fragments under 4 chars are rejected — too ambiguous.
func anchorSubstringIndex(orig []string, anchor string) int {
	a := collapseWS(anchor)
	if len(a) < 4 {
		return -1
	}
	for i, l := range orig {
		if strings.Contains(collapseWS(l), a) {
			return i
		}
	}
	return -1
}

// anchorMatchesExisting reports whether the "@@" anchor already corresponds to a line
// in the file — a looser test than the exact seekSequence (whitespace-collapsed
// substring). When true, the model mangled an EXISTING line into an anchor (e.g.
// "main() {" for "func main() {") rather than declaring something new, so the
// reconstruct-and-append recovery must not fire and duplicate the block.
func anchorMatchesExisting(orig []string, anchor string) bool {
	a := collapseWS(anchor)
	if len(a) < 3 { // too short to be a distinctive anchor
		return false
	}
	for _, l := range orig {
		if strings.Contains(collapseWS(l), a) {
			return true
		}
	}
	return false
}

// collapseWS trims and collapses every run of whitespace to a single space, so anchor
// comparison ignores the indentation a small model routinely gets wrong.
// hasExplicitReplacement: some hunk both removes and adds lines, so the
// model spelled out an old line and its replacement.
func hasExplicitReplacement(chunks []patchChunk) bool {
	for _, ch := range chunks {
		if ch.hasRemoval && len(ch.addedLines) > 0 {
			return true
		}
	}
	return false
}

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func newDeclarationAppend(ch patchChunk) []string {
	ctx := strings.TrimSpace(ch.changeContext)
	if !ch.hasContext || ctx == "" || len(ch.newLines) == 0 {
		return nil
	}
	if !strings.HasSuffix(ctx, "{") && !strings.HasSuffix(ctx, ":") {
		return nil
	}
	block := make([]string, 0, len(ch.newLines)+2)
	block = append(block, "", ch.changeContext)
	block = append(block, ch.newLines...)
	return block
}

// topDeclRe matches the first line of a top-level Go declaration — `func Name(`,
// `func (r Recv) Name(`, or `type Name` — capturing the identifier (group 1 for a func,
// group 2 for a type) so it can be deduped against declarations already in the file.
var topDeclRe = regexp.MustCompile(`^\s*(?:func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)|type\s+([A-Za-z_]\w*))\b`)

// declName returns the identifier a top-level func/type line declares, or "" if the
// line isn't one.
func declName(line string) string {
	m := topDeclRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// declExists reports whether the file already declares a top-level func/type of this name.
func declExists(orig []string, name string) bool {
	for _, l := range orig {
		if declName(l) == name {
			return true
		}
	}
	return false
}

// braceBalanced reports whether the block opens at least one "{" and closes even. Naive
// (counts braces inside strings/comments too), so a pathological body makes it return
// false — which safely bails the append rather than emitting broken code.
func braceBalanced(lines []string) bool {
	depth, sawOpen := 0, false
	for _, l := range lines {
		for _, r := range l {
			switch r {
			case '{':
				depth++
				sawOpen = true
			case '}':
				depth--
			}
		}
	}
	return sawOpen && depth == 0
}

// newFuncFromHunk recovers the "add a function" patch a small model reliably mangles: it
// anchors on an EXISTING line (e.g. "@@ main() {"), echoes a context line, emits a
// spurious "+}", then puts a WHOLE new top-level declaration as the tail of the hunk
// (often with the signature line unmarked). The normal match fails on that shape, but the
// intent is unambiguous — append the new func. When the hunk's new lines contain a
// complete, brace-balanced top-level func/type NOT already in the file, splice it at EOF.
// This is the tool absorbing a consistent small-model malformation rather than erroring on it.
func newFuncFromHunk(orig []string, ch patchChunk) (replacement, bool) {
	start, name := -1, ""
	for i, l := range ch.newLines {
		if n := declName(l); n != "" && !declExists(orig, n) {
			start, name = i, n
			break
		}
	}
	if start < 0 || name == "" {
		return replacement{}, false
	}
	block := ch.newLines[start:]
	if !braceBalanced(block) {
		return replacement{}, false // truncated / not a complete declaration — bail cleanly
	}
	ins := len(orig)
	if len(orig) > 0 && orig[len(orig)-1] == "" {
		ins = len(orig) - 1 // insert before the file's trailing blank line
	}
	out := append([]string{""}, block...) // blank separator before the new decl
	return replacement{ins, 0, out}, true
}

// alnumOnly reduces a line to its letters and digits — dropping whitespace, operators,
// and punctuation. Two lines with the same alnumOnly are "the same statement with a
// different operator/token" (e.g. "return a + b" vs "return a * b").
func alnumOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// nearDuplicateAddReplacement catches the malformation where the model expresses a
// one-line CHANGE as pure context + a bare "+" add: it keeps the original line as
// context and ADDS the edited version (e.g. context "return a + b", add "return a * b").
// Applied literally that splices a near-duplicate line into the file — often a stray
// statement after a "}" that won't compile. When an added line is the alnum-twin of a
// context line (same identifiers, different operator), the intent is to REPLACE that
// context line; do so instead of inserting. Only fires for a pure-add hunk (no "-").
func nearDuplicateAddReplacement(orig []string, ch patchChunk) (replacement, bool) {
	if len(ch.removedLines) != 0 || len(ch.addedLines) == 0 || len(ch.oldLines) == 0 {
		return replacement{}, false
	}
	// A hunk whose adds contain a top-level DECLARATION is adding new code (a
	// function/type), not miswriting a one-line change — even if the new body
	// happens to be the operator-twin of a context line (blend's "a - b" vs
	// combine's "a + b", live misfire). Leave it to the normal match.
	for _, add := range ch.addedLines {
		if declName(add) != "" {
			return replacement{}, false
		}
	}
	for _, add := range ch.addedLines {
		ka := alnumOnly(add)
		if len(ka) < 3 {
			continue
		}
		for _, ctx := range ch.oldLines {
			if ctx == add || alnumOnly(ctx) != ka {
				continue
			}
			// Same statement, different operator — replace ctx with add in the file.
			if found, oldLen, _ := matchPattern(orig, []string{ctx}, []string{ctx}, 0, false); found >= 0 {
				return replacement{found, oldLen, []string{add}}, true
			}
		}
	}
	return replacement{}, false
}

// minimalChangeReplacement recovers a hunk whose full old-block won't match because the
// model buried its real change under spurious context. It matches ONLY the removed
// ("-") lines in the file and replaces them with the added ("+") lines — the hunk's
// actual intent. It requires at least one SUBSTANTIAL removed line (not a bare "}" or
// blank), so a lone brace can't match somewhere arbitrary. Searches the whole file (the
// anchor may have been bogus). Returns false when there's no clean minimal change.
func minimalChangeReplacement(orig []string, ch patchChunk) (replacement, bool) {
	if len(ch.removedLines) == 0 || len(ch.addedLines) == 0 {
		return replacement{}, false
	}
	substantial := false
	for _, l := range ch.removedLines {
		t := strings.TrimSpace(l)
		if len(t) > 2 && strings.ContainsFunc(t, func(r rune) bool {
			return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		}) {
			substantial = true
			break
		}
	}
	if !substantial {
		return replacement{}, false
	}
	found, oldLen, _ := matchPattern(orig, ch.removedLines, ch.removedLines, 0, false)
	if found < 0 {
		return replacement{}, false
	}
	return replacement{found, oldLen, ch.addedLines}, true
}

// matchPattern seeks pattern in orig starting at `from`, retrying once without a
// trailing blank line (a common emission artifact). It returns the match index, the
// number of ORIGINAL lines the match consumed (oldLen — usually len(pattern) but
// larger when the blank-tolerant fallback absorbs an extra file blank), and the
// replacement lines to splice in. found<0 when no form matches.
func matchPattern(orig, pattern, newSlice []string, from int, eof bool) (found, oldLen int, newLines []string) {
	if f := seekSequence(orig, pattern, from, eof); f >= 0 {
		return f, len(pattern), newSlice
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == "" {
		p := pattern[:len(pattern)-1]
		n := newSlice
		if len(n) > 0 && n[len(n)-1] == "" {
			n = n[:len(n)-1]
		}
		if f := seekSequence(orig, p, from, eof); f >= 0 {
			return f, len(p), n
		}
	}
	// Last resort: tolerate a blank line dropped from (or added to) the context. A
	// small model reconstructing a region from memory routinely omits a blank
	// separator between logical blocks, which fails the contiguous match above.
	// We match ignoring blank-line differences and consume the REAL file span (so
	// applyReplacements splices correctly), replacing it with the model's new block
	// — which represents the region's intended final state. Skipped for eof anchors.
	if !eof {
		if f, consumed := seekSkippingBlanks(orig, pattern, from); f >= 0 {
			return f, consumed, newSlice
		}
	}
	return -1, 0, newSlice
}

// seekSkippingBlanks finds pattern in lines at or after start, allowing blank lines
// to differ between the two (present in one but not the other). It returns the match
// start and the number of ORIGINAL lines consumed (which may exceed len(pattern) when
// the file carries extra blanks), or (-1, 0). Comparison is trim+punct-normalized —
// this is a recovery path taken only after the exact ladder fails.
func seekSkippingBlanks(lines, pattern []string, start int) (int, int) {
	norm := func(s string) string { return normalizePunct(strings.TrimSpace(s)) }
	isBlank := func(s string) bool { return strings.TrimSpace(s) == "" }
	// Require at least one non-blank pattern line to anchor on, else a pattern of
	// only blanks would match anywhere.
	hasContent := false
	for _, p := range pattern {
		if !isBlank(p) {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return -1, 0
	}
	for i := start; i < len(lines); i++ {
		f, p := i, 0
		for p < len(pattern) && f < len(lines) {
			if norm(lines[f]) == norm(pattern[p]) {
				f, p = f+1, p+1
				continue
			}
			if isBlank(lines[f]) && !isBlank(pattern[p]) {
				f++ // extra blank in the file the pattern dropped
				continue
			}
			if isBlank(pattern[p]) && !isBlank(lines[f]) {
				p++ // extra blank in the pattern the file lacks
				continue
			}
			break
		}
		for p < len(pattern) && isBlank(pattern[p]) {
			p++ // trailing pattern blanks the file doesn't have
		}
		if p == len(pattern) {
			return i, f - i
		}
	}
	return -1, 0
}

func applyReplacements(lines []string, repls []replacement) []string {
	result := append([]string(nil), lines...)
	// Apply back-to-front so earlier indices stay valid.
	for i := len(repls) - 1; i >= 0; i-- {
		r := repls[i]
		end := r.start + r.oldLen
		if end > len(result) {
			end = len(result)
		}
		tail := append([]string(nil), result[end:]...)
		result = append(result[:r.start], append(append([]string(nil), r.newLines...), tail...)...)
	}
	return result
}

// seekSequence finds pattern (a run of lines) in lines at or after start, trying an
// exact match then progressively looser normalizations. Returns -1 if not found.
func seekSequence(lines, pattern []string, start int, eof bool) int {
	if len(pattern) == 0 {
		return start
	}
	if len(pattern) > len(lines) {
		return -1
	}
	maxStart := len(lines) - len(pattern)
	searchStart := start
	if eof {
		searchStart = maxStart
	}
	if searchStart > maxStart {
		return -1
	}
	norms := []func(string) string{
		func(s string) string { return s },
		func(s string) string { return strings.TrimRight(s, " \t\r") },
		strings.TrimSpace,
		func(s string) string { return normalizePunct(strings.TrimSpace(s)) },
	}
	for _, norm := range norms {
		for i := searchStart; i <= maxStart; i++ {
			if linesMatch(lines, pattern, i, norm) {
				return i
			}
		}
	}
	return -1
}

func linesMatch(lines, pattern []string, start int, norm func(string) string) bool {
	for idx := 0; idx < len(pattern); idx++ {
		if norm(lines[start+idx]) != norm(pattern[idx]) {
			return false
		}
	}
	return true
}

// normalizePunct folds the unicode dashes/quotes a model may substitute back to ASCII.
func normalizePunct(s string) string {
	repl := strings.NewReplacer(
		"‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "―", "-", "−", "-",
		"‘", "'", "’", "'", "“", "\"", "”", "\"",
	)
	return repl.Replace(s)
}

// dirHasGoFiles reports whether dir contains at least one .go file — the
// existence test for a module-local import.
func dirHasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}

// modulePackages lists the module's real importable packages (dirs under the
// module root containing .go files, skipping hidden/vendor), capped, so the
// import-guard's teaching shows what actually exists.
func modulePackages(mod *moduleInfo) []string {
	var pkgs []string
	_ = filepath.WalkDir(mod.root, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(pkgs) >= 10 {
			return filepath.SkipAll
		}
		if d.IsDir() {
			n := d.Name()
			if path != mod.root && (strings.HasPrefix(n, ".") || n == "vendor" || n == "node_modules" || n == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			rel, _ := filepath.Rel(mod.root, filepath.Dir(path))
			imp := mod.name
			if rel != "." {
				imp = mod.name + "/" + filepath.ToSlash(rel)
			}
			if len(pkgs) == 0 || pkgs[len(pkgs)-1] != imp {
				pkgs = append(pkgs, imp)
			}
		}
		return nil
	})
	if len(pkgs) == 0 {
		return []string{mod.name}
	}
	return pkgs
}

// absorbModulePathEcho fixes a small-model habit: echoing the module name into file
// paths ("demo/mathx/add_test.go" inside module demo), which would create a
// spurious nested dir and a package go can't resolve. When the path's first
// segment below the module root repeats the module name and that dir doesn't
// actually exist, the redundant segment is stripped.
func absorbModulePathEcho(target string) string {
	mod := findGoMod(filepath.Dir(target))
	if mod == nil || mod.name == "" {
		return target
	}
	rel, err := filepath.Rel(mod.root, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return target
	}
	seg, rest, ok := strings.Cut(filepath.ToSlash(rel), "/")
	if !ok || rest == "" {
		return target
	}
	if seg != mod.name && seg != path.Base(mod.name) {
		return target
	}
	// Only a NON-EMPTY dir blocks the strip — a project may legitimately have a
	// subdir named after its module, but an empty one is debris from a prior
	// refused patch.
	if entries, readErr := os.ReadDir(filepath.Join(mod.root, seg)); readErr == nil && len(entries) > 0 {
		return target
	}
	return filepath.Join(mod.root, filepath.FromSlash(rest))
}

// describeGoParseErrors formats up to three parse errors WITH the offending
// source line quoted — "main.go:25:90" is useless to a model that cannot count
// lines in its own patch; the line text is what it can actually act on.
func describeGoParseErrors(target, content string) string {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, target, content, 0)
	if err == nil {
		return ""
	}
	lines := strings.Split(content, "\n")
	var out []string
	if list, ok := err.(scanner.ErrorList); ok {
		for i, e := range list {
			if i >= 3 {
				out = append(out, fmt.Sprintf("  (and %d more)", len(list)-3))
				break
			}
			quoted := ""
			if ln := e.Pos.Line - 1; ln >= 0 && ln < len(lines) {
				quoted = fmt.Sprintf(" — line reads: %q", strings.TrimSpace(lines[ln]))
			}
			out = append(out, fmt.Sprintf("  line %d: %s%s", e.Pos.Line, e.Msg, quoted))
		}
		return strings.Join(out, "\n")
	}
	return "  " + err.Error()
}

// applyContextRewrite treats a context-only chunk (no +/- lines) whose lines
// ALMOST match one region of the file as an implicit replacement: the matching
// lines anchor it, the differing lines are the edit. Guarded three ways — a
// majority of lines must match, at least one must differ, and for .go targets
// the result must still parse.
func applyContextRewrite(target, fileContent string, chunks []patchChunk) (string, bool) {
	lines, layout := splitFileLines(fileContent)
	changed := false
	for _, ch := range chunks {
		if len(ch.addedLines) > 0 || len(ch.removedLines) > 0 {
			continue // a real hunk — not this dialect
		}
		hl := append([]string{}, ch.newLines...)
		for len(hl) > 0 && strings.TrimSpace(hl[len(hl)-1]) == "" {
			hl = hl[:len(hl)-1]
		}
		if len(hl) < 2 {
			continue // one context line is too weak an anchor to rewrite by
		}
		matchLine := func(fileLine, hunkLine string) bool {
			if collapseWS(fileLine) == collapseWS(hunkLine) {
				return true
			}
			// Envelope-tail junk glues onto the LAST hunk line ('}"}},"name":…');
			// if the prefix up to the first quote matches the file line, the line
			// is really unchanged and the junk is transport noise.
			if qi := strings.Index(hunkLine, `"`); qi > 0 {
				return collapseWS(fileLine) == collapseWS(strings.TrimRight(hunkLine[:qi], " \t"))
			}
			return false
		}
		bestIdx, bestScore := -1, 0
		for i := 0; i+len(hl) <= len(lines); i++ {
			score := 0
			for j, l := range hl {
				if matchLine(lines[i+j], l) {
					score++
				}
			}
			if score > bestScore {
				bestScore, bestIdx = score, i
			}
		}
		// Majority must anchor, and something must actually change.
		if bestIdx < 0 || bestScore <= len(hl)/2 || bestScore == len(hl) {
			continue
		}
		// Matching lines keep the FILE's version (original indentation, no junk);
		// only the differing lines — the actual edit — come from the hunk.
		region := make([]string, len(hl))
		for j, l := range hl {
			if matchLine(lines[bestIdx+j], l) {
				region[j] = lines[bestIdx+j]
			} else {
				region[j] = l
			}
		}
		next := append([]string{}, lines[:bestIdx]...)
		next = append(next, region...)
		next = append(next, lines[bestIdx+len(hl):]...)
		lines = next
		changed = true
	}
	if !changed {
		return "", false
	}
	out := layout.join(lines)
	if strings.HasSuffix(target, ".go") && goParseErr(target, out) != nil {
		return "", false // never trade a parseable file for a broken one
	}
	return out, true
}

// shearEnvelopeJunkLines drops lines that are unmistakably tool-call envelope
// debris — they contain the envelope's own "name":"apply_patch" fragment. The
// .go path already repairs these parse-gated; this is the universal backstop
// for every OTHER file type (live: go.mod ended with ')","name":"apply_patch'
// TWICE and go could no longer parse it).
func shearEnvelopeJunkLines(target, content string) string {
	if !strings.Contains(content, `"name":"apply_patch`) && !strings.Contains(content, `\"name\":\"apply_patch`) {
		return content
	}
	shear := func(content string) string {
		lines := strings.Split(content, "\n")
		var out []string
		for _, l := range lines {
			if strings.Contains(l, `"name":"apply_patch`) || strings.Contains(l, `\"name\":\"apply_patch`) {
				// junk glued onto real content: keep the part before the first quote
				if qi := strings.IndexAny(l, `"\`); qi > 0 {
					if kept := strings.TrimRight(l[:qi], " \t"); kept != "" {
						out = append(out, kept)
					}
				}
				continue
			}
			out = append(out, l)
		}
		return strings.Join(out, "\n")
	}
	// A .go file that MENTIONS the fragment in a real comment or string (live
	// 2026-09-30: apply_patch.go quoting the envelope in its own doc comment)
	// must not be cut mid-line — the shear corrupts the file and the syntax
	// guard then blames the patch. The shear fires for a Go target only when
	// the file as-is does not parse and the sheared form does.
	if strings.HasSuffix(target, ".go") && goParseErr(target, content) == nil {
		return content
	}
	sheared := shear(content)
	if strings.HasSuffix(target, ".go") && goParseErr(target, sheared) != nil {
		return content // shearing buys nothing — keep the file whole
	}
	return sheared
}

// guardGoModSyntax refuses a write that would corrupt go.mod — go's OWN parser
// is the gate. The model edits go.mod under teaching pressure (go get flows)
// and a broken go.mod bricks EVERY subsequent go command in the project.
func guardGoModSyntax(target, content string) error {
	if filepath.Base(target) != "go.mod" {
		return nil
	}
	if _, err := modfile.Parse(target, []byte(content), nil); err != nil {
		return fmt.Errorf("this would CORRUPT go.mod (%v) — the file was NOT written. go.mod rarely needs manual edits: use bash 'go get <module>' to add dependencies, or leave it alone", err)
	}
	return nil
}

// PatchParses reports whether a model's patch text is one this harness can
// apply. Used to probe a model before somebody pins it (gateway/model_check.go):
// a model that advertises tools can still write a patch nothing accepts.
func PatchParses(text string) bool {
	hunks, err := parsePatch(text)
	return err == nil && len(hunks) > 0
}

// patchSnapshot is the content of every file a patch touches, as it was
// before the patch; exists=false records a file the patch would create.
type patchSnapshot map[string]struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

func snapshotPatchTargets(targets []string, hunks []patchHunk, cwd string) patchSnapshot {
	snap := patchSnapshot{}
	add := func(p string) {
		if _, seen := snap[p]; seen {
			return
		}
		fi, err := os.Stat(p)
		if err != nil {
			snap[p] = struct {
				exists bool
				data   []byte
				mode   os.FileMode
			}{}
			return
		}
		data, _ := os.ReadFile(p)
		snap[p] = struct {
			exists bool
			data   []byte
			mode   os.FileMode
		}{true, data, fi.Mode().Perm()}
	}
	for i, t := range targets {
		add(t)
		if mp := hunks[i].movePath; mp != "" {
			if cwd != "" && !filepath.IsAbs(mp) {
				mp = filepath.Join(cwd, mp)
			}
			add(mp)
		}
	}
	return snap
}

// restore puts every snapshotted file back as it was.
func (s patchSnapshot) restore() {
	for p, f := range s {
		if f.exists {
			_ = os.WriteFile(p, f.data, f.mode)
		} else {
			_ = os.Remove(p)
		}
	}
}

// patchFiles is every file a patch touches, in order, each once.
// decodeLiteralEscapes turns a patch sent with literal \n escapes (and no
// real newline) into lines.
func decodeLiteralEscapes(input string) string {
	if !strings.Contains(input, "\n") && strings.Contains(input, "\\n") {
		return literalEscapeRun.ReplaceAllStringFunc(input, decodeOneEscape)
	}
	return input
}

// PatchTargets is the files an apply_patch input names, read the way
// ApplyPatch reads it (both formats); nil when it does not parse.
func PatchTargets(input json.RawMessage) []string {
	var params ApplyPatchInput
	if json.Unmarshal(input, &params) != nil {
		return nil
	}
	text := decodeLiteralEscapes(params.Input)
	if isHashlinePatch(text) {
		secs, err := parseHashline(text)
		if err != nil {
			return nil
		}
		var out []string
		for _, s := range secs {
			if !slices.Contains(out, s.path) {
				out = append(out, s.path)
			}
		}
		return out
	}
	hunks, err := parsePatch(normalizePatchInput(text))
	if err != nil {
		return nil
	}
	return patchFiles(hunks)
}

func patchFiles(hunks []patchHunk) []string {
	var out []string
	seen := map[string]bool{}
	for _, h := range hunks {
		for _, p := range []string{h.path, h.movePath} {
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}
