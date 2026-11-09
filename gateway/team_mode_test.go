package gateway

import (
	"context"
	"strings"
	"testing"
	"time"
)

// teamOps records the run a team turn starts: which workflow, the model line
// and the prompt.
type teamOps struct {
	fakeWorkflowOps
	name, model, prompt string
}

func (f *teamOps) startWorkflow(dir, name, session, _, _, _, model, prompt string, _ time.Duration) (*workflowRun, error) {
	f.name, f.model, f.prompt = name, model, prompt
	return f.run, nil
}

// THE MODE IS ITS OWN SETTING: setMode("team") keeps it on the session and
// the route shows it; the reasoning effort is untouched either way, and auto
// leaves team mode.
func TestTheModeIsItsOwnSetting(t *testing.T) {
	sess := &Session{ID: "ws:chan", Metadata: map[string]interface{}{}}
	if err := setEffort(sess, "high"); err != nil {
		t.Fatal(err)
	}
	if err := setMode(sess, "team"); err != nil {
		t.Fatal(err)
	}
	if !sessionTeam(sess) || sessionString(sess, sessionEffortKey) != "high" {
		t.Fatalf("team mode on, the effort kept: team=%v effort=%q", sessionTeam(sess), sessionString(sess, sessionEffortKey))
	}
	if v := viewRoute(nil, sess, "coder"); v.Mode != "team" || v.Effort != "high" {
		t.Fatalf("the route shows mode %q and effort %q", v.Mode, v.Effort)
	}
	if err := setEffort(sess, "low"); err != nil || !sessionTeam(sess) {
		t.Fatalf("changing the effort keeps the mode: team=%v err=%v", sessionTeam(sess), err)
	}
	if err := setMode(sess, "auto"); err != nil || sessionTeam(sess) || viewRoute(nil, sess, "coder").Mode != "auto" {
		t.Fatalf("auto leaves team mode: team=%v err=%v", sessionTeam(sess), err)
	}
	if err := setMode(sess, "crew"); err == nil {
		t.Fatal("a mode that is not auto or team is refused")
	}
}

// A TURN IN TEAM MODE IS TEAM-SOLVE on the project, with the message as its
// prompt and the conversation's pinned model; an ordinary turn, and a
// workflow step's own turn, never start one. Mutation check: drop the
// "workflow:" guard and the step case fails; drop sessionTeam and the
// ordinary case fails.
func TestATeamModeTurnRunsTeamSolve(t *testing.T) {
	ar, base, ctx := workflowToolForTest(t)
	ops := &teamOps{fakeWorkflowOps: *base}
	ar.workflows = ops

	plain := &Session{ID: "ws:chan", Metadata: map[string]interface{}{}}
	if _, ok := ar.teamTurn(ctx, "fix the scan", plain, "r1"); ok || ops.name != "" {
		t.Fatalf("an ordinary turn started %q", ops.name)
	}

	team := &Session{ID: "ws:chan", Metadata: map[string]interface{}{}}
	_ = setMode(team, "team")
	_ = pinModel(team, "openrouter:deepseek/deepseek-v4.1-flash", "price", "")
	resp, ok := ar.teamTurn(ctx, "fix the scan outside git", team, "r2")
	if !ok || resp == nil {
		t.Fatal("a team-mode turn is handled as a team turn")
	}
	if ops.name != "team-solve" || ops.prompt != "fix the scan outside git" || ops.model != "openrouter:deepseek/deepseek-v4.1-flash price" {
		t.Fatalf("started %q with model %q and prompt %q", ops.name, ops.model, ops.prompt)
	}
	if !strings.Contains(resp.Text, "Team mode") || !strings.Contains(resp.Text, "reports here") {
		t.Fatalf("the turn says the team is on it and will report: %q", resp.Text)
	}

	// The window's "@coder " before the message is not part of the task.
	ctx = context.WithValue(ctx, "buddy_agent_name", "coder") //nolint:staticcheck // the key the runtime reads
	if _, ok := ar.teamTurn(ctx, "@coder fix the scan outside git", team, "r4"); !ok || ops.prompt != "fix the scan outside git" {
		t.Fatalf("the agent mention is dropped from the task: %q", ops.prompt)
	}
	if got := teamPrompt("@coderx keep it", "coder"); got != "@coderx keep it" {
		t.Fatalf("only the agent's own mention is dropped: %q", got)
	}

	ops.name = ""
	step := &Session{ID: "workflow:team-solve:p:solve", Metadata: map[string]interface{}{}}
	_ = setMode(step, "team")
	if _, ok := ar.teamTurn(ctx, "solve", step, "r3"); ok || ops.name != "" {
		t.Fatal("a workflow step's turn never starts a team")
	}
}

// THE TEAM THINKS AT THE WINDOW'S EFFORT (#87): the effort chosen with Ctrl+T
// goes on the run's model line, with or without a pinned model, and an agent
// step reads it. Mutation check: drop the effort from teamModelLine, or the
// no-id branch of pinTaskModel, and this fails.
func TestTheTeamThinksAtTheWindowsEffort(t *testing.T) {
	s := &Session{ID: "ws:chan", Metadata: map[string]interface{}{}}
	if got := teamModelLine(s); got != "" {
		t.Fatalf("nothing chosen, nothing on the line: %q", got)
	}
	_ = setEffort(s, "low")
	if got := teamModelLine(s); got != "thinking low" {
		t.Fatalf("an effort and no pin: %q", got)
	}
	_ = pinModel(s, "openrouter:deepseek/deepseek-v4.1-flash", "price", "")
	if got := teamModelLine(s); got != "openrouter:deepseek/deepseek-v4.1-flash price thinking low" {
		t.Fatalf("a pin and an effort: %q", got)
	}

	step := &Session{ID: "workflow:team-solve:p:fix", Metadata: map[string]interface{}{}}
	if err := pinTaskModel(step, "thinking low", "nobody"); err != nil {
		t.Fatalf("a line with no id is the model that answers here: %v", err)
	}
	if e := sessionString(step, sessionEffortKey); e != "low" || sessionModel(step) != "" {
		t.Fatalf("the step thinks at %q on %q, want low on no pin", e, sessionModel(step))
	}
}
