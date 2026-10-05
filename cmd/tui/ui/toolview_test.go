package ui

import (
	"strings"
	"testing"
)

// Each kind of work wears its own colour, and a tool nobody registered stays
// grey rather than being guessed at.
func TestViewForColours(t *testing.T) {
	cases := map[string]string{
		"bash":                       colRun,
		"Bash":                       colRun,
		"read_file":                  colRead,
		"grep":                       colRead,
		"apply_patch":                colWrite,
		"edit_file":                  colWrite,
		"todo_write":                 colPlan,
		"recall":                     colRecall,
		"ask_user_question":          colAsk,
		"something_a_model_invented": colDim,
	}
	for tool, want := range cases {
		if got := viewFor(tool).Colour(); got != want {
			t.Errorf("viewFor(%q).Colour() = %s, want %s", tool, got, want)
		}
	}
}

// Labels show the argument that matters, not whichever key Go's map iteration
// happened to hand back first.
func TestViewLabels(t *testing.T) {
	cases := []struct {
		tool, input, want string
	}{
		{"bash", `{"command":"go test ./..."}`, "Bash(go test ./...)"},
		{"read_file", `{"file_path":"main.go"}`, "Read(main.go)"},
		{"grep", `{"pattern":"func main"}`, "grep(func main)"},
		{"recall", `{"limit":1,"query":"deploy infra"}`, "recall(deploy infra)"},
		{"edit_file", `{"file_path":"pkg/x.go"}`, "Edit(pkg/x.go)"},
		{"apply_patch", `{"input":"*** Add File: sq.py\n+print(1)"}`, "Edit(sq.py)"},
	}
	for _, c := range cases {
		if got := viewFor(c.tool).Label(c.input, 100); got != c.want {
			t.Errorf("%s label = %q, want %q", c.tool, got, c.want)
		}
	}
}

// The label must be stable: the same call renders the same way every turn.
func TestLabelStableAcrossCalls(t *testing.T) {
	input := `{"limit":1,"query":"deploy infra","depth":2}`
	first := viewFor("recall").Label(input, 100)
	for i := 0; i < 50; i++ {
		if got := viewFor("recall").Label(input, 100); got != first {
			t.Fatalf("label changed between turns: %q then %q", first, got)
		}
	}
}

// A change renders as a diff with a header saying what moved; everything else
// falls through to the generic body.
func TestWriteViewRendersDiff(t *testing.T) {
	patch := `{"input":"*** Update File: sq.py\n for i in range(3):\n-    print(i)\n+    print(i ** 2)\n+    print('done')"}`
	out := "Patch applied.\n\nsq.py is now:\nfor i in range(3):\n    print(i ** 2)\n    print('done')\n"
	body := stripANSI(viewFor("apply_patch").Body(ToolRender{Input: patch, Output: out, Width: 100}))
	if body == "" {
		t.Fatal("apply_patch rendered no diff body")
	}
	for _, want := range []string{"Update sq.py", "+2", "−1", "print(i ** 2)"} {
		if !strings.Contains(body, want) {
			t.Errorf("diff body missing %q:\n%s", want, body)
		}
	}
	// The control lines are said by the header, not repeated in the hunk.
	if strings.Contains(body, "*** Update File") {
		t.Errorf("diff repeats the patch control line:\n%s", body)
	}

	if got := viewFor("bash").Body(ToolRender{Input: `{"command":"ls"}`}); got != "" {
		t.Errorf("bash body = %q, want empty (generic body)", got)
	}
}

// A patch that failed changed nothing — the frame must not read as applied.
func TestFailedPatchSaysNotApplied(t *testing.T) {
	patch := `{"input":"*** Update File: hi.py\n+print(\"hi\")"}`
	body := stripANSI(viewFor("apply_patch").Body(ToolRender{
		Input: patch, Err: "the file does not exist", Width: 100,
	}))
	if !strings.Contains(body, "not applied") {
		t.Errorf("a failed patch must say so in the header:\n%s", body)
	}
	// The counts belong to a change that happened.
	if strings.Contains(body, "+1") {
		t.Errorf("a failed patch must not advertise additions:\n%s", body)
	}
	// The attempt is still worth seeing.
	if !strings.Contains(body, `print("hi")`) {
		t.Errorf("the attempted change should still render:\n%s", body)
	}
}

// The checklist label counts steps rather than stringifying the list, and shows
// the DISPLAY name: "todo_write" is the contract, "Plan" is what a human reads.
func TestPlanLabelCountsSteps(t *testing.T) {
	in := `{"todos":[{"content":"a"},{"content":"b"},{"content":"c"}]}`
	if got := viewFor("todo_write").Label(in, 100); got != "Plan(3 steps)" {
		t.Errorf("label = %q, want Plan(3 steps)", got)
	}
	one := `{"todos":[{"content":"a"}]}`
	if got := viewFor("todo_write").Label(one, 100); got != "Plan(1 step)" {
		t.Errorf("label = %q, want the singular", got)
	}
}

// A tool's DISPLAY name and its TECHNICAL name are deliberately different and
// must not be confused: "apply_patch" is what the model calls, what the session
// records and what `memdoor logs query` prints; "Edit" is only a label.
//
// Keeping the mapping in one place is what makes that safe. It used to be
// hardcoded inside each view, so the same tool could render differently in two
// frames — and counting tools off the pane on 2026-08-30 missed every
// apply_patch, because the screen says "Edit" and the logs say "apply_patch".
func TestDisplayNameIsNotTheTechnicalName(t *testing.T) {
	for technical, shown := range map[string]string{
		"apply_patch": "Edit",
		"write_file":  "Write",
		"read_file":   "Read",
		"todo_write":  "Plan",
		"todo_read":   "Plan",
	} {
		if got := displayName(technical); got != shown {
			t.Errorf("displayName(%q) = %q, want %q", technical, got, shown)
		}
	}
	// Several technical names share one label, which is exactly why the display
	// name must never be used to identify a tool.
	if displayName("apply_patch") != displayName("edit_file") {
		t.Error("apply_patch and edit_file both render as Edit — that collision is intended")
	}
	// An unmapped tool renders under its own name: a missing mapping should look
	// plain, never wrong.
	if got := displayName("locate"); got != "locate" {
		t.Errorf("an unmapped tool must render as itself, got %q", got)
	}
}

// A long diff is capped, and says so — a silent truncation reads as "that was
// the whole change".
func TestDiffAnnouncesTruncation(t *testing.T) {
	var b strings.Builder
	b.WriteString("*** Update File: big.go\n")
	for i := 0; i < 60; i++ {
		b.WriteString("+line\n")
	}
	body := stripANSI(renderDiff(b.String(), "", false, false))
	if !strings.Contains(body, "+46 lines (ctrl+o)") {
		t.Errorf("capped diff does not announce how much it cut:\n%s", body)
	}
	expanded := stripANSI(renderDiff(b.String(), "", true, false))
	if strings.Contains(expanded, "lines (ctrl+o)") {
		t.Errorf("expanded diff still truncates:\n%s", expanded)
	}
}

// Rows carry their real line number in the file the patch produced — an update
// numbers against the echoed file, an added file numbers by construction.
func TestDiffLineNumbers(t *testing.T) {
	patch := "*** Update File: sq.py\n def sq(n):\n-    return n + n\n+    return n * n\n"
	out := "Patch applied.\n\nsq.py is now:\ndef sq(n):\n    return n * n\n"
	body := stripANSI(renderDiff(patch, fileEcho(out), false, false))
	for _, want := range []string{"   1  def sq(n):", "   2 -    return n + n", "   2 +    return n * n"} {
		if !strings.Contains(body, want) {
			t.Errorf("diff missing numbered row %q:\n%s", want, body)
		}
	}

	// Blank lines used to derail numbering: an empty line matches every other
	// empty line, so a search-first walk leapt down the file.
	add := "*** Add File: a.py\n+one\n+\n+two\n+\n+three\n"
	body = stripANSI(renderDiff(add, "", false, false))
	for _, want := range []string{"   1 +one", "   3 +two", "   5 +three"} {
		if !strings.Contains(body, want) {
			t.Errorf("added file missing numbered row %q:\n%s", want, body)
		}
	}
}

// A line the renderer cannot place gets no number rather than a wrong one.
func TestDiffOmitsUnknownLineNumbers(t *testing.T) {
	patch := "*** Update File: x.go\n unrelated context that is not in the file\n"
	body := stripANSI(renderDiff(patch, "package main\n", false, false))
	if strings.Contains(body, "1 unrelated") {
		t.Errorf("unlocatable row was numbered anyway:\n%s", body)
	}
}

// The patch arrives wrapped in whatever envelope the model produced.
func TestUnwrapPatch(t *testing.T) {
	cases := []string{
		"*** Add File: a.py\n+x",
		`{"input":"*** Add File: a.py\n+x"}`,
		`{"name":"apply_patch","arguments":{"input":"*** Add File: a.py\n+x"}}`,
	}
	for _, raw := range cases {
		got := unwrapPatch(raw)
		if !strings.HasPrefix(got, "*** Add File: a.py") {
			t.Errorf("unwrapPatch(%q) = %q", raw, got)
		}
		if path, op := patchTarget(got); path != "a.py" || op != "Add" {
			t.Errorf("patchTarget = (%q, %q), want (a.py, Add)", path, op)
		}
	}
}

// A model sends {"path": null} and the label printed Go's nil formatting:
// ⏺ skill(path: "<nil>") — live, 2026-08-31 18:43. A null argument is the
// model's mistake, but the label's job is to say what was asked, and quoting
// runtime internals says nothing. Skip null/empty values when choosing what
// to show; a call with ONLY null args renders as the bare (...) form.
func TestANullArgumentNeverRendersAsNil(t *testing.T) {
	got := genericLabel("skill", `{"path": null}`, 120)
	if strings.Contains(got, "<nil>") {
		t.Errorf("label quotes Go internals: %s", got)
	}
	if got != "skill(...)" {
		t.Errorf("a call with only null args should render bare, got %s", got)
	}

	// A null beside a real value: show the real one.
	got = genericLabel("skill", `{"path": null, "name": "new-program"}`, 120)
	if !strings.Contains(got, "new-program") || strings.Contains(got, "<nil>") {
		t.Errorf("the real argument lost to the null one: %s", got)
	}
}
