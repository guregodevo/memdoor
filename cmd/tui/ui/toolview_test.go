package ui

import (
	"strings"
	"testing"
	"time"
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

// A grep reads as its hits grouped by file, most hits first; nothing found
// says so; ctrl+o hands the raw output back to the generic body.
func TestSearchViewGroupsGrepHits(t *testing.T) {
	out := "a.go:10:func main() {\nb.go:3:x := 1\na.go:20:main()\n"
	body := stripANSI(viewFor("grep").Body(ToolRender{Output: out, Width: 100}))
	for _, want := range []string{"3 hits in 2 files", "a.go · 2", "b.go · 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("grep body missing %q:\n%s", want, body)
		}
	}
	if strings.Index(body, "a.go") > strings.Index(body, "b.go") {
		t.Errorf("the file with more hits comes first:\n%s", body)
	}
	if got := viewFor("grep").Body(ToolRender{Output: out, Expand: true}); got != "" {
		t.Errorf("an expanded frame shows the raw output, got %q", got)
	}
	if got := stripANSI(viewFor("grep").Body(ToolRender{Output: "No matches found"})); !strings.Contains(got, "no hits") {
		t.Errorf("nothing found must read as nothing found, got %q", got)
	}
	if got := stripANSI(viewFor("glob").Body(ToolRender{Output: "x/a.go\nx/b.go\n"})); !strings.Contains(got, "2 files") {
		t.Errorf("glob body: %q", got)
	}
}

// A web search reads as its sources, the answer's first line above them.
func TestSearchViewListsWebSources(t *testing.T) {
	out := "Go 1.26 was released in February.\nMore text.\n\nSources:\n[1] Go 1.26 release notes — https://go.dev/doc/go1.26\n    snippet here\n[2] Blog — https://www.example.org/post\n"
	body := stripANSI(viewFor("web_search").Body(ToolRender{Output: out, Width: 100}))
	for _, want := range []string{"2 sources", "[1] Go 1.26 release notes go.dev", "[2] Blog example.org", "Go 1.26 was released"} {
		if !strings.Contains(body, want) {
			t.Errorf("web search body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "snippet here") {
		t.Errorf("snippets are the model's to read, not the frame's:\n%s", body)
	}
}

// locate keeps its head line and the ranked places, not the quoted sections.
func TestSearchViewLocateKeepsPlaces(t *testing.T) {
	out := "Where \"retry\" lives (judged: 2 of 5 sections kept), most relevant first:\n1. net/retry.go:12-40  p=0.93\n2. cmd/run.go:88-120  p=0.71\n\n== net/retry.go:12-40 (p=0.93)\n12 func retry() {\n13   …\n"
	body := stripANSI(viewFor("locate").Body(ToolRender{Output: out, Width: 100}))
	for _, want := range []string{"Where \"retry\" lives", "1. net/retry.go:12-40", "2. cmd/run.go:88-120"} {
		if !strings.Contains(body, want) {
			t.Errorf("locate body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "func retry()") {
		t.Errorf("the section text belongs to ctrl+o:\n%s", body)
	}
	if got, _ := searchHitFiles("locate", out); len(got) != 2 || got[0] != "net/retry.go" {
		t.Errorf("locate's files for the files panel: %v", got)
	}
}

// A spawned run's frame draws its trail live and closes it when it reports.
func TestSpawnViewDrawsTheTrail(t *testing.T) {
	out := "▶ run-7f3a spawned as coder (session agent:coder:subagent:run-7f3a, up to 600s). It runs in the background and reports here when done — do not wait."
	if got := spawnSession(out); got != "agent:coder:subagent:run-7f3a" {
		t.Fatalf("spawnSession = %q", got)
	}
	labelled := "▶ run-31e4 \"test-and-read\" spawned as coder (session agent:coder:subagent:run-31e4, up to 300s). It runs in the background."
	if got := spawnSession(labelled); got != "agent:coder:subagent:run-31e4" {
		t.Fatalf("a labelled spawned line (live 2026-10-11) must parse, got %q", got)
	}
	if body := stripANSI(viewFor("sessions_spawn").Body(ToolRender{Output: labelled})); !strings.Contains(body, "▶ coder · test-and-read") {
		t.Fatalf("the label is in the head line:\n%s", body)
	}
	body := stripANSI(viewFor("sessions_spawn").Body(ToolRender{Output: out, Trail: []trailStep{{"bash", 12}, {"read_file", 0}}}))
	for _, want := range []string{"▶ coder", "run-7f3a", "up to 600s", "◦ Bash 12s", "◦ Read"} {
		if !strings.Contains(body, want) {
			t.Errorf("spawn body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "reported back") {
		t.Errorf("a running child has not reported back:\n%s", body)
	}
	done := stripANSI(viewFor("sessions_spawn").Body(ToolRender{Output: out, Trail: []trailStep{{"bash", 12}}, TrailDone: true}))
	if !strings.Contains(done, "✓ reported back") {
		t.Errorf("a finished child closes the trail:\n%s", done)
	}
	if got := stripANSI(viewFor("sessions_spawn").Label(`{"task":"run the tests\nand report"}`, 100)); got != "Spawn(run the tests …)" {
		t.Errorf("label = %q", got)
	}

	// The window keeps the trail from the child's beats: a new tool is a
	// step, the same tool again only grows its seconds, done closes it.
	var m Model
	for _, b := range []subagentWorkMsg{
		{sessionID: "s1", agent: "coder", tool: "bash", seconds: 5},
		{sessionID: "s1", agent: "coder", tool: "bash", seconds: 10},
		{sessionID: "s1", agent: "coder", tool: "read_file", seconds: 0},
		{sessionID: "s1", done: true},
	} {
		m.noteSubagentStep(b)
	}
	tr := m.subagentTrail["s1"]
	if tr == nil || len(tr.steps) != 2 || tr.steps[0].seconds != 10 || !tr.done {
		t.Fatalf("trail = %+v", tr)
	}
}

// The workflow tool's frame is the run's head line; the tasks are the live
// graph's below it, never a stale copy here.
func TestWorkflowViewDrawsTasks(t *testing.T) {
	out := "▶ release · 4 tasks · started. It reports here on its own — do not wait, sleep or poll; say it is running and end your turn.\nwf-12 · running · 1/4 done\n  done     vet\n  running  test  ← vet\n  waiting  notes  ← test  (external)\n  waiting  tag  ← notes"
	body := stripANSI(viewFor("workflow").Body(ToolRender{Output: out, Width: 100}))
	for _, want := range []string{"wf-12 · running · 1/4 done", "4 tasks, drawn below"} {
		if !strings.Contains(body, want) {
			t.Errorf("workflow body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "✓ vet") || strings.Contains(body, "○ tag") {
		t.Errorf("the tasks belong to the live graph, not to a copy in the frame:\n%s", body)
	}
	if got := viewFor("workflow").Label(`{"action":"run","name":"release"}`, 100); got != "Workflow(run release)" {
		t.Errorf("label = %q", got)
	}
	if got := viewFor("workflow").Body(ToolRender{Output: "■ wf-12 stopped — nothing more starts."}); got != "" {
		t.Errorf("an answer with no task lines falls through to the generic body, got %q", got)
	}
}

// The gateway's grep prints file:content with no line number and absolute
// paths (live 2026-10-11): still grouped, the shared folder said once.
func TestSearchViewGroupsPathOnlyHits(t *testing.T) {
	out := "/tmp/w/calc.go:func Add(a, b int) int {\n/tmp/w/calc_test.go:func TestAdd(t *testing.T) {\n/tmp/w/calc_test.go:    if Add(2, 3) != 5 {\n"
	body := stripANSI(viewFor("grep").Body(ToolRender{Output: out, Width: 100}))
	for _, want := range []string{"3 hits in 2 files", "in /tmp/w/", "calc_test.go · 2", "calc.go · 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("grep body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "/tmp/w/calc.go") {
		t.Errorf("the shared folder is said once, not per file:\n%s", body)
	}
	// In the project's own window the folder is the project: not said at all;
	// a folder under it reads relative.
	if body := stripANSI(viewFor("grep").Body(ToolRender{Output: out, Root: "/tmp/w"})); strings.Contains(body, " · in ") {
		t.Errorf("the project folder itself is not named:\n%s", body)
	}
	if body := stripANSI(viewFor("grep").Body(ToolRender{Output: out, Root: "/tmp"})); !strings.Contains(body, "in w/") {
		t.Errorf("a folder under the project reads relative:\n%s", body)
	}
}

// A spawn frame stays out of scrollback while its child works, and its
// render follows the trail (the cache key moves with it).
func TestSpawnFrameStaysLiveWhileTheChildWorks(t *testing.T) {
	out := "▶ run-1 spawned as coder (session agent:coder:subagent:run-1, up to 300s). It runs in the background."
	var m Model
	m.width, m.height = 100, 40
	m.messages = []Message{
		{Role: "user", Content: "spawn it"},
		{Role: "tool_call", ToolName: "sessions_spawn", ToolInput: `{"task":"t"}`, ToolOutput: out, ToolDone: true, Timestamp: time.Now()},
		{Role: "assistant", Content: "Spawned."},
	}
	if !m.spawnLive(m.messages[1]) {
		t.Fatal("a fresh spawn frame waits for its child's first beat")
	}
	m.noteSubagentStep(subagentWorkMsg{sessionID: "agent:coder:subagent:run-1", agent: "coder", tool: "bash", seconds: 3})
	first := plainText(m.renderBlock(1, m.messages[1]))
	if !strings.Contains(first, "◦ Bash 3s") {
		t.Fatalf("the frame draws the trail:\n%s", first)
	}
	m.noteSubagentStep(subagentWorkMsg{sessionID: "agent:coder:subagent:run-1", agent: "coder", tool: "bash", seconds: 8})
	second := plainText(m.renderBlock(1, m.messages[1]))
	if !strings.Contains(second, "◦ Bash 8s") {
		t.Fatalf("a new beat re-renders the frame (the cache key must move):\n%s", second)
	}
	// settledCount flushes a frame only when the message rule AND the
	// trail rule agree: this one is settled as a message, live as a spawn.
	if !settled(m.messages[1]) || !m.spawnLive(m.messages[1]) {
		t.Fatal("the spawn frame must not be flushed while the child works")
	}
	m.noteSubagentStep(subagentWorkMsg{sessionID: "agent:coder:subagent:run-1", done: true})
	if m.spawnLive(m.messages[1]) {
		t.Fatal("a reported child settles its frame")
	}
	if !strings.Contains(plainText(m.renderBlock(1, m.messages[1])), "✓ reported back") {
		t.Fatal("the settled frame says the child reported back")
	}
}
