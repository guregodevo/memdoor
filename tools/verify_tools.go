package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/pkg/decision"
	"os"
	"strings"
	"time"

	"memdoor/gateway/logs"
)

// verify tool — run a build/test/run command and report, from the toolchain's own
// exit status, whether the code PASSES. It is a plain capability an agent (e.g. a
// verifier) calls when it decides to; the runtime does not run it automatically or
// force it. Distinct from bash only in framing: the result leads with PASS/FAIL so
// the model grounds its judgement in what actually happened, not its own claim.
var VerifyDefinition = ToolDefinition{
	Name: "verify",
	Description: "Run a build/test/run command and report whether it PASSES (exit 0) or FAILS, " +
		"with the real command output. Use it to check that code actually compiles/runs/tests " +
		"green, with the standard command for the file's language — e.g. 'go run file.go', " +
		"'python3 file.py', 'node file.js', 'go test ./...'. Report FAIL honestly; " +
		"do not claim success the command did not produce.",
	InputSchema: VerifyInputSchema,
	Function:    Verify,
}

type VerifyInput struct {
	Command string `json:"command" jsonschema_description:"The build/test/run command to execute — the standard one for the file's language, e.g. 'go run file.go', 'python3 file.py', 'node file.js'."`
	Dir     string `json:"dir,omitempty" jsonschema_description:"Optional working directory to run the command in (absolute path). Defaults to the current directory."`
}

var VerifyInputSchema = GenerateSchema[VerifyInput]()

func Verify(input json.RawMessage) (string, error) {
	in := VerifyInput{}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	if in.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	log := logs.New("Agent")
	log.Debug("Verifying", slog.String("command", in.Command), slog.String("dir", in.Dir))

	dir := in.Dir
	if dir != "" {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			return fmt.Sprintf("FAIL — working directory %q does not exist", dir), nil
		}
	}
	// Same reason as the bash tool: this output is read by a model, so it must
	// be text and not terminal escapes. See bash_plain.go.
	output, err := runPlainBash(in.Command, dir)
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		trimmed = "(no output)"
	}

	if err != nil {
		log.Debug("verify FAIL", slog.String("command", in.Command), slog.String("error", err.Error()))
		return fmt.Sprintf("FAIL — `%s` (%s)\n\n%s", in.Command, err.Error(), trimmed), nil
	}
	// Exit 0 is necessary, not sufficient: ask the decision model whether the
	// output actually shows success (verifyJudge, below).
	if p, judged := judgeExitZero(in.Command, trimmed); judged && p < verifyFailBelow {
		log.Info("verify: exit 0 judged a failure", slog.String("command", in.Command), slog.Float64("p_success", p))
		return fmt.Sprintf("FAIL — `%s` exited 0, but its output reads as a failure (decision model: P(success)=%.2f)\n\n%s",
			in.Command, p, trimmed), nil
	}
	log.Debug("verify PASS", slog.String("command", in.Command))
	return fmt.Sprintf("PASS — `%s`\n\n%s", in.Command, trimmed), nil
}

// THE VERIFIER ASKS JEV WHEN THERE IS A SEAT (Greg, 2026-09-27: "the verifier
// should use jev", "if there is a seat").
//
// An exit code is the toolchain's own word, and a non-zero one is final: that
// is a FAIL and the decision model is not even asked. But exit 0 says less than
// it seems. A test runner that finds no tests exits 0. `go test` on a package
// with no test files exits 0. A program that catches its own panic and prints it
// exits 0. A script that echoes "Error:" and carries on exits 0. Every one of
// those used to come back PASS, and the coder moved on believing it had checked
// something.
//
// So on exit 0 the output goes to the decision model with one typed question:
// did this actually show success? Below verifyFailBelow the verdict becomes
// FAIL, with the probability in the line so nobody has to trust it blind.
//
// It can only make the verdict STRICTER. A failing exit is never turned into a
// pass, so the one mistake that ships broken code is not one this can cause —
// the worst it can do is send the coder to look again at something that was
// fine. Without a seat the decision service answers unavailable, and the
// verdict is the exit code, exactly as before.
const (
	verifyFailBelow    = 0.4
	verifyJudgeTail    = 8000 // failures print at the end; keep the tail
	verifyJudgeTimeout = 3 * time.Second
)

// judgeExitZero is P(the output shows success), and whether the decision model
// answered at all.
func judgeExitZero(command, output string) (float64, bool) {
	svc := decisionService()
	if svc == nil {
		return 0, false
	}
	if len(output) > verifyJudgeTail {
		output = "…" + output[len(output)-verifyJudgeTail:]
	}
	q, err := decision.Boolean(
		"Did this command show that the code works — that it built, ran, or had tests run and pass?",
		"the output shows success: it built, the program ran as intended, or tests actually ran and passed",
		"the output shows a failure despite exiting 0, or nothing was actually checked (no tests ran, no files matched, an error was printed and swallowed)",
	)
	if err != nil {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), verifyJudgeTimeout)
	defer cancel()
	res := svc.Evaluate(ctx, decision.Request{
		State:     "A verifier ran this command and it exited 0.\n\nCommand: " + command + "\n\nOutput:\n" + output,
		Questions: map[string]decision.Question{"ok": q},
	}, decision.Options{Purpose: "verify"})
	if !res.OK() {
		return 0, false
	}
	a, ok := res.Answers["ok"]
	if !ok {
		return 0, false
	}
	return a.ProbabilityTrue, true
}
