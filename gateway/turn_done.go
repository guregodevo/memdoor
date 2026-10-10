package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/decision"
	"memdoor/tools"
)

// turn_done: DONE MEANS THE RECEIPTS SHOW IT, not that the model said so.
//
// The turn verdict (turn_verdict.go) reads the model's last sentence. The
// evidence of what the turn actually did sits in the tool records and was
// never judged: a verify that returned FAIL, a patch whose build failed, a
// file changed with no build or test after it, a "done" with nothing run.
// Greg, 2026-10-03: "let's use Jev for the harness", "let's start with no
// babysitting" — the person should never come back to a "done" that isn't.
//
// Two parts. turnReceipts is mechanical: what changed, what was checked, and
// whether the last change was checked at all; it ends every codebase turn
// with one line the person can read. unfinished asks the decision model one
// yes/no over the request, those receipts and the final message; a yes sends
// the model back once, with the receipts' own reasons at the top of its
// prompt. Unavailable means the turn ends as it did before this existed.

const (
	settingTurnDone          = "decision_turn_done"
	settingTurnDoneThreshold = "decision_turn_done_threshold"
	turnDoneDefaultThreshold = 0.7
	turnDoneTimeout          = 2500 * time.Millisecond
	turnDoneMaxEvidence      = 3000
	// The question is about what the receipts SHOW, not about whether every
	// requirement was met: the receipts cannot show that, and a question that
	// invited it scored a correct, vet-and-race-tested result 0.84 and 0.85
	// unfinished twice (live 2026-10-03, a 16k-token round wasted).
	turnDoneQuestion = "Looking only at the receipts and the agent's final message: do the receipts contradict the message — " +
		"a change with no build, test or run after it, a check that FAILED after the last change, or a claim of work the receipts show was never done? " +
		"(A passing check after the last change, an honest report of being blocked, or a question back to the person is not a contradiction.)"
	receiptMaxChecks = 3
	// checkOutputHead is how much of a check's output the evidence carries.
	checkOutputHead = 120

	// settingTurnBudget is the person's token budget for running on without
	// them: after the first continuation, the turn keeps going while it is
	// judged unfinished and the budget is not spent (Greg, 2026-10-03: "a
	// token budget the user can set until it do it autonomously"). 0, the
	// default, is one continuation, as before.
	settingTurnBudget = "turn_token_budget"
	// turnBudgetKey on a session overrides the workspace budget for its
	// turns: a workflow task's `budget:` (workflow_runs.go).
	turnBudgetKey   = "turn_budget"
	continueMaxSaid = 400

	// doneWhenKey on a session is the turn's own definition of done, in
	// mario's words: "file X" or "command: Y" (a workflow task's target,
	// gateway/workflow_runs.go). When it is declared, the receipts are not
	// judged: the artifact exists or it does not. Found live 2026-10-03: the
	// trading run's two tasks made their files and were judged unfinished
	// twice each, because nothing told the turn what done was.
	doneWhenKey     = "done_when"
	doneWhenCommand = 60 * time.Second
)

// doneWhen checks a declared definition of done in the workdir. declared is
// false when the string is not one; ok says the artifact is there.
func doneWhen(ctx context.Context, workdir, target string) (label string, ok, declared bool) {
	switch {
	case strings.HasPrefix(target, "file "):
		name := strings.TrimSpace(strings.TrimPrefix(target, "file "))
		if name == "" {
			return "", false, false
		}
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(workdir, name)
		}
		st, err := os.Stat(path)
		return "target " + name, err == nil && !st.IsDir(), true
	case strings.HasPrefix(target, "command: "):
		cmd := strings.TrimSpace(strings.TrimPrefix(target, "command: "))
		if cmd == "" {
			return "", false, false
		}
		cctx, cancel := context.WithTimeout(ctx, doneWhenCommand)
		defer cancel()
		c := exec.CommandContext(cctx, "sh", "-c", cmd)
		c.Dir = workdir
		return "target `" + firstLine(cmd, 60) + "`", c.Run() == nil, true
	}
	return "", false, false
}

// doneWhenNudge is the continuation when the declared artifact is missing.
func doneWhenNudge(label string) string {
	if strings.HasPrefix(label, "target `") {
		return "Not done yet: the " + label + " does not pass. This task is done only when it exits 0 — make it pass now, run it, and end on what it printed."
	}
	return "Not done yet: the " + label + " does not exist. This task is done only when it does — produce it now, check that it is there, and end on that."
}

// sessionBudget is a session's own budget (a workflow task's), else the
// workspace's.
func (v *turnVerdict) sessionBudget(s interface {
	GetMetadataValue(string) (interface{}, bool)
}) int64 {
	if s != nil {
		if b, ok := s.GetMetadataValue(turnBudgetKey); ok {
			switch n := b.(type) {
			case int64:
				if n > 0 {
					return n
				}
			case float64:
				if n > 0 {
					return int64(n)
				}
			}
		}
	}
	return v.tokenBudget()
}

// tokenBudget is the workspace's unattended budget in tokens, 0 when unset.
func (v *turnVerdict) tokenBudget() int64 {
	if v == nil || v.read == nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v.read(settingTurnBudget)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// continueAllowed: the first continuation is free; the rest spend the budget.
func continueAllowed(rounds int, spent, budget int64) bool {
	if rounds == 0 {
		return true
	}
	return budget > 0 && spent < budget
}

// unattendedPrompt is added to the system prompt of a turn with a budget:
// the model will not be stopped round by round, so what it needs from the
// person it must ask for first (Greg, 2026-10-03: "the model should ask
// question early if it feels it wont do it without asking").
func unattendedPrompt(budget int64) string {
	return fmt.Sprintf("You run unattended on this request, up to about %s tokens, continuing until the work is shown done by a build, test or run. "+
		"Anything you can find out or check yourself — by reading the code, querying, running a command, validating a result — you do, not ask. "+
		"Only what cannot be verified without the person — a choice between designs, a missing requirement, an ambiguous target — "+
		"ask now with ask_user_question, before changing anything, in one message. Then work to the end.", approxTokens(budget))
}

func approxTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1_000)
	}
	return strconv.FormatInt(n, 10)
}

// continueNudge turns the model's own last words into its next instruction:
// a reply that ended on "next I'll run the tests" is told to do exactly that
// (Greg, 2026-10-03: "the model to say what is next as prompt so it
// continues"). The receipts' reasons stay under it.
func continueNudge(said string, r turnReceipts) string {
	said = strings.TrimSpace(clipTail(strings.TrimSpace(said), continueMaxSaid))
	why := strings.Join(r.reasons(), "; ")
	if why != "" {
		why = " The receipts: " + why + "."
	}
	if said == "" {
		return doneNudge(r)
	}
	return "Continue. You ended on: \"" + said + "\"." + why +
		" Do that next step now, keep going until a build, test or run shows the request done, and end on what the check returned."
}

// turnCheck is one build, test or run the turn made, and whether it passed.
type turnCheck struct {
	label string
	pass  bool
	// command and output keep a bash run's evidence for the exit-zero
	// judge (judgeExitZeroChecks); empty for other checks.
	command, output string
}

// turnReceipts is what a turn did, read from the tool records.
type turnReceipts struct {
	changed []string    // files a mutating tool touched and did not fail on
	checks  []turnCheck // verify results and post-patch builds, in order
	// checkedAfterChange: a check ran after the last change, and passed.
	checkedAfterChange bool
	// failingAfterChange: the last check after the last change failed.
	failingAfterChange bool
	// lastFailure is the last tool error of the turn, one line, or "".
	lastFailure string
}

func receiptsOf(executed []ToolExecutionInfo) turnReceipts {
	var r turnReceipts
	lastChange, lastCheck := -1, -1
	lastCheckPass := false
	for i, t := range executed {
		switch t.Name {
		case tools.ApplyPatchDefinition.Name:
			if t.Error == "" {
				files := tools.PatchTargets(json.RawMessage(t.Input))
				if len(files) == 0 {
					files = []string{"(apply_patch)"}
				}
				r.changed = appendNew(r.changed, files...)
				lastChange = i
			}
			if c, ok := patchBuildCheck(t.Output); ok {
				r.checks = append(r.checks, c)
				lastCheck, lastCheckPass = i, c.pass
			}
		case "write_file", "edit_file":
			if t.Error == "" {
				var in struct {
					Path     string `json:"path"`
					FilePath string `json:"file_path"`
				}
				_ = json.Unmarshal([]byte(t.Input), &in)
				if in.Path == "" {
					in.Path = in.FilePath
				}
				r.changed = appendNew(r.changed, in.Path)
				lastChange = i
			}
		case "bash":
			// A command run is a check: a test, a script, a build. Pass is
			// its exit status; what it wrote, if anything, is unknown.
			var in struct {
				Command string `json:"command"`
			}
			_ = json.Unmarshal([]byte(t.Input), &in)
			if tools.IsBashRead(in.Command) {
				// ls, cat, grep …: a read, not a check. Live 2026-10-04 a probe
				// for a missing folder read "FAIL" in the receipt line.
				// EXCEPT A READ-BACK: a file just written, then read (ls, cat,
				// head …) and found, IS checked — the proof a file with no
				// build or test can have. A child that wrote hello.txt and
				// `cat`ed it was judged unfinished at 0.90 (live 2026-10-05).
				if lastChange < 0 || t.Error != "" || !readsBack(in.Command, r.changed) {
					continue
				}
				c := turnCheck{label: "read back: " + firstLine(in.Command, 50), pass: true, command: in.Command, output: t.Output}
				r.checks = append(r.checks, c)
				lastCheck, lastCheckPass = i, true
				continue
			}
			// A failed command is reported to the model in the output
			// ("Command FAILED (exit status 1): …"), not as a tool error:
			// live 2026-10-03 a failed `go build` read PASS here, the turn
			// looked proven, and a broken stub was called done.
			pass := t.Error == "" && !strings.Contains(t.Output, tools.CommandFailedPrefix)
			c := turnCheck{label: firstLine(in.Command, 60), pass: pass}
			if pass {
				c.command, c.output = in.Command, t.Output
			}
			r.checks = append(r.checks, c)
			lastCheck, lastCheckPass = i, pass
		case "search_replace":
			if t.Error == "" {
				lastChange = i
				r.changed = appendNew(r.changed, "(search_replace)")
			}
		case tools.VerifyDefinition.Name:
			if c, ok := verifyCheck(t.Output, t.Error); ok {
				r.checks = append(r.checks, c)
				lastCheck, lastCheckPass = i, c.pass
			}
		}
		if t.Error != "" {
			r.lastFailure = t.Name + ": " + firstLine(t.Error, 160)
		}
	}
	if lastChange >= 0 && lastCheck >= lastChange {
		r.checkedAfterChange = lastCheckPass
		r.failingAfterChange = !lastCheckPass
	}
	return r
}

// readsBack says whether a read command names one of the files the turn
// changed (by path or by base name).
func readsBack(command string, changed []string) bool {
	for _, f := range changed {
		if f == "" || strings.HasPrefix(f, "(") {
			continue
		}
		if strings.Contains(command, f) || strings.Contains(command, filepath.Base(f)) {
			return true
		}
	}
	return false
}

// verifyCheck reads the verify tool's "PASS — `cmd`" / "FAIL — `cmd`" head.
func verifyCheck(output, errText string) (turnCheck, bool) {
	head := firstLine(output, 200)
	switch {
	case strings.HasPrefix(head, "PASS"):
		return turnCheck{label: backticked(head), pass: true}, true
	case strings.HasPrefix(head, "FAIL"):
		return turnCheck{label: backticked(head), pass: false}, true
	case errText != "":
		return turnCheck{label: "verify", pass: false}, true
	}
	return turnCheck{}, false
}

// patchBuildCheck reads the build apply_patch folds into its result
// (tools/patch_verify.go): "verify: PASS — `go build ./...` …".
func patchBuildCheck(output string) (turnCheck, bool) {
	i := strings.Index(output, "verify: ")
	if i < 0 {
		return turnCheck{}, false
	}
	head := firstLine(output[i+len("verify: "):], 200)
	switch {
	case strings.HasPrefix(head, "PASS"):
		return turnCheck{label: backticked(head), pass: true}, true
	case strings.HasPrefix(head, "FAIL"):
		return turnCheck{label: backticked(head), pass: false}, true
	}
	return turnCheck{}, false
}

// backticked is the `command` inside a PASS/FAIL head, or the head itself.
func backticked(head string) string {
	if a := strings.IndexByte(head, '`'); a >= 0 {
		if b := strings.IndexByte(head[a+1:], '`'); b >= 0 {
			return head[a+1 : a+1+b]
		}
	}
	return strings.TrimSpace(head)
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[:nl]
	}
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

func appendNew(list []string, items ...string) []string {
	for _, it := range items {
		if it == "" {
			continue
		}
		dup := false
		for _, have := range list {
			if have == it {
				dup = true
				break
			}
		}
		if !dup {
			list = append(list, it)
		}
	}
	return list
}

// line is the receipt the window shows under a codebase turn, or "" for a
// turn that changed nothing and checked nothing (an answer, a read).
func (r turnReceipts) line() string {
	if len(r.changed) == 0 && len(r.checks) == 0 {
		return ""
	}
	var b strings.Builder
	switch {
	case len(r.changed) == 0:
		// Commands ran and nothing changed: a ✓ here read as proof of work
		// that was never done (live 2026-10-03: mkdir and touch under a
		// turn whose every patch was refused).
		b.WriteString("⚠ nothing changed · ran: ")
	case r.failingAfterChange:
		b.WriteString("⚠ failing after the change: ")
	case !r.checkedAfterChange:
		b.WriteString("⚠ unverified: ")
	default:
		b.WriteString("✓ checked: ")
	}
	var parts []string
	if n := len(r.changed); n > 0 {
		parts = append(parts, fmt.Sprintf("%d file%s changed", n, plural(n)))
	}
	if len(r.checks) == 0 && len(r.changed) > 0 {
		parts = append(parts, "no build, test or run after it")
	}
	from := len(r.checks) - receiptMaxChecks
	if from < 0 {
		from = 0
	}
	for _, c := range r.checks[from:] {
		verdict := "PASS"
		if !c.pass {
			verdict = "FAIL"
		}
		parts = append(parts, "`"+c.label+"` "+verdict)
	}
	b.WriteString(strings.Join(parts, " · "))
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// evidence is the receipts as the decision model reads them.
func (r turnReceipts) evidence() string {
	var b strings.Builder
	if len(r.changed) == 0 {
		b.WriteString("No file was changed.\n")
	} else {
		fmt.Fprintf(&b, "Files changed: %s\n", strings.Join(r.changed, ", "))
	}
	if len(r.checks) == 0 {
		b.WriteString("No build, test or run was made.\n")
	}
	for _, c := range r.checks {
		verdict := "PASS"
		if !c.pass {
			verdict = "FAIL"
		}
		// What the run printed decides what PASS means: `test -f X && echo
		// EXISTS || echo NO` exits 0 either way, and a PASS alone read as
		// "it exists" against an honest "not yet" (0.67 live, 2026-10-05).
		if head := firstLine(c.output, checkOutputHead); head != "" {
			fmt.Fprintf(&b, "Check: %s → %s · printed: %s\n", c.label, verdict, head)
			continue
		}
		fmt.Fprintf(&b, "Check: %s → %s\n", c.label, verdict)
	}
	switch {
	case r.failingAfterChange:
		b.WriteString("The last check after the last change FAILED.\n")
	case r.checkedAfterChange:
		b.WriteString("The last check after the last change PASSED.\n")
	case len(r.changed) > 0:
		b.WriteString("Nothing was checked after the last change.\n")
	}
	if r.lastFailure != "" {
		fmt.Fprintf(&b, "Last tool error: %s\n", r.lastFailure)
	}
	return clipHead(b.String(), turnDoneMaxEvidence)
}

// reasons are the mechanical grounds a retry can name; empty when the
// receipts alone do not say what is missing.
func (r turnReceipts) reasons() []string {
	var out []string
	if r.failingAfterChange {
		out = append(out, "the last check after your change FAILED")
	}
	if len(r.changed) > 0 && !r.checkedAfterChange && !r.failingAfterChange {
		out = append(out, "nothing was built, tested or run after your last change")
	}
	if len(r.changed) == 0 && len(r.checks) == 0 {
		out = append(out, "no file was changed and nothing was run")
	}
	return out
}

// doneNudge is the one sentence at the top of the retry's prompt. The
// receipts' own reasons, when they have one; otherwise the judge's verdict.
func doneNudge(r turnReceipts) string {
	why := strings.Join(r.reasons(), "; ")
	if why == "" {
		why = "the work asked for is not shown done by what this turn ran"
	}
	return "Not done yet: " + why + ". Finish the request now — make the change, run the build or test that proves it — and end on what the check returned. Do not report done without it."
}

// unfinished asks the decision model whether the receipts show the request
// left undone. P and whether it clears the threshold; false when off,
// unavailable, or under it.
func (v *turnVerdict) unfinished(ctx context.Context, agent, request, reply string, r turnReceipts) (float64, bool) {
	if v == nil || v.svc == nil {
		return 0, false
	}
	// PROOF BEATS JUDGMENT: a check that passed after the last change is the
	// artifact a code task produces. The judge is for what has no proof — a
	// change nothing checked, a failing check, a claim with no change.
	if r.checkedAfterChange {
		return 0, false
	}
	if strings.EqualFold(strings.TrimSpace(v.read(settingTurnDone)), "off") {
		return 0, false
	}
	threshold := turnDoneDefaultThreshold
	if t, err := strconv.ParseFloat(strings.TrimSpace(v.read(settingTurnDoneThreshold)), 64); err == nil && t > 0 && t <= 1 {
		threshold = t
	}
	q, err := decision.Boolean(turnDoneQuestion, "", "")
	if err != nil {
		return 0, false
	}
	state := "Request: " + clipHead(request, turnVerdictMaxRequest) +
		"\n\nReceipts of this turn:\n" + r.evidence() +
		"\nAgent's final message:\n" + clipTail(reply, turnVerdictMaxReply)
	start := time.Now()
	res := v.svc.Evaluate(ctx, decision.Request{State: state, Questions: map[string]decision.Question{"unfinished": q}},
		decision.Options{AgentID: agent, Purpose: "turn-done", Timeout: turnDoneTimeout})
	log := logs.New("Decisions")
	if !res.OK() {
		log.Info("Turn done check skipped", slog.String("agent", agent), slog.String("reason", string(res.Reason)))
		return 0, false
	}
	p := res.Answers["unfinished"].ProbabilityTrue
	retry := p >= threshold
	log.Info("Turn done verdict", slog.String("agent", agent), slog.Float64("p_unfinished", p), slog.Bool("retry", retry),
		slog.Int("changed", len(r.changed)), slog.Int("checks", len(r.checks)), slog.Int64("ms", time.Since(start).Milliseconds()))
	return p, retry
}

// EXIT 0 IS NOT A PASS WHEN THE OUTPUT SAYS FAIL (live 2026-10-03:
// `go test … 2>&1 | tail -20` printed FAIL and returned tail's 0, and the
// receipt read PASS). The decision model reads the output of the bash runs
// the receipt shows — the last few, the deciding one among them — and a run
// whose output shows a failure becomes a failing check. Model- and
// language-agnostic (Greg, 2026-10-03): the question names no language, tool
// or test runner, and a run that only lists or reads is not a failure.
// Each output is judged once per process (exitZeroSeen).

const (
	// An error the command provokes on purpose is its expected result (live
	// 2026-10-09: a run that fed an unknown strategy and printed "exit=1" to
	// test the error path read FAIL under a turn whose checks all held).
	exitZeroQuestion = "This shell command exited 0. Does its output show that something failed — an error, a failing test, a failed build or check — " +
		"even though the exit status says success? (Output that only lists, prints or reads things, with no error, is not a failure. " +
		"An error the command provokes on purpose — bad input fed to test how a program refuses it, its exit code then printed or expected — is the result it wanted, not a failure.)"
	exitZeroFailAt  = 0.7
	exitZeroTail    = 6000
	exitZeroTimeout = 2500 * time.Millisecond
	// exitZeroCommandMax is how much of the command the judge reads: all of
	// it, short of a heredoc's body. What a command means to test often comes
	// after its first line.
	exitZeroCommandMax = 1200
)

var exitZeroSeen sync.Map // hash(command+output) → bool failed

// judgeExitZeroChecks marks as failing the shown bash checks whose output
// the decision model reads as a failure, and recomputes the verdicts that
// depend on the last check. Nothing changes when no model answers.
func (v *turnVerdict) judgeExitZeroChecks(ctx context.Context, r turnReceipts) turnReceipts {
	if v == nil || v.svc == nil || len(r.checks) == 0 {
		return r
	}
	from := len(r.checks) - receiptMaxChecks
	if from < 0 {
		from = 0
	}
	changed := false
	for i := from; i < len(r.checks); i++ {
		c := r.checks[i]
		if !c.pass || strings.TrimSpace(c.output) == "" {
			continue
		}
		if v.exitZeroFailed(ctx, c.command, c.output) {
			r.checks[i].pass = false
			changed = true
		}
	}
	if changed && len(r.changed) > 0 && (r.checkedAfterChange || r.failingAfterChange) {
		last := r.checks[len(r.checks)-1]
		r.checkedAfterChange, r.failingAfterChange = last.pass, !last.pass
	}
	return r
}

func (v *turnVerdict) exitZeroFailed(ctx context.Context, command, output string) bool {
	key := questionHash(command + "\x00" + output)
	if f, ok := exitZeroSeen.Load(key); ok {
		return f.(bool)
	}
	if len(output) > exitZeroTail {
		output = "…" + output[len(output)-exitZeroTail:]
	}
	q, err := decision.Boolean(exitZeroQuestion, "", "")
	if err != nil {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, exitZeroTimeout)
	defer cancel()
	res := v.svc.Evaluate(cctx, decision.Request{
		State:     "Command: " + clipHead(command, exitZeroCommandMax) + "\n\nOutput:\n" + output,
		Questions: map[string]decision.Question{"failed": q},
	}, decision.Options{Purpose: "receipt-exit-zero", Timeout: exitZeroTimeout})
	if !res.OK() {
		return false // unknown is not a failure; nothing cached, asked again next time
	}
	failed := res.Answers["failed"].ProbabilityTrue >= exitZeroFailAt
	exitZeroSeen.Store(key, failed)
	logs.New("Decisions").Info("Receipt exit-zero judged", slog.String("command", firstLine(command, 80)),
		slog.Float64("p_failed", res.Answers["failed"].ProbabilityTrue), slog.Bool("failed", failed))
	return failed
}
