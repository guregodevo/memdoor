package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/infra"
	"memdoor/pkg/authorization"
)

// TEAM MODE (#86): Shift+Tab flips the mode, auto and team (the thinking
// effort is its own setting, #87). A turn in team mode is
// team-solve on the conversation's project and model, with the person's
// prompt, as typed, as its task; the run reports back into the conversation when it ends, the way a
// workflow the agent starts does.
//
// Why a mode and not the default (2026-11-05, #84): on deepseek-v4.1-flash a
// single turn fixed two scoped tasks for a tenth of the team's cost and time,
// and lost its way on an open-ended one (48 calls, no review written) that
// team-solve passed. The person picks the team for the task that needs it.

// teamWorkflow is the workflow a team turn runs.
const teamWorkflow = "team-solve"

// sessionTeam reports whether the conversation is in team mode.
func sessionTeam(s *Session) bool { return s != nil && sessionString(s, sessionTeamKey) != "" }

// sessionMode is the conversation's mode as the route shows it: auto or team.
func sessionMode(s *Session) string {
	if sessionTeam(s) {
		return "team"
	}
	return "auto"
}

// setMode holds the conversation's mode (Shift+Tab): auto, one turn as
// always, or team. The mode is not the reasoning effort: each is its own
// setting (the maintainer, 2026-11-05: "thinking and mode are 2 different
// parameters").
func setMode(s *Session, mode string) error {
	switch m := strings.ToLower(strings.TrimSpace(mode)); m {
	case "auto", "":
		s.SetMetadata(sessionTeamKey, "")
	case "team":
		s.SetMetadata(sessionTeamKey, "on")
	default:
		return fmt.Errorf("mode is auto or team (got %q)", mode)
	}
	return nil
}

// teamModelLine is the run's --model: the conversation's pinned model with its
// host preference and the effort chosen with Ctrl+T, so the team answers on
// the model and effort the footer shows. No pin leaves each step to the
// person's kept pin or the ladder; no effort chosen leaves each step on auto.
func teamModelLine(s *Session) string {
	var parts []string
	if m := sessionModel(s); m != "" {
		parts = append(parts, m)
		if sort := sessionString(s, sessionSortKey); sort != "" {
			parts = append(parts, sort)
		}
		if order := sessionString(s, sessionOrderKey); order != "" {
			parts = append(parts, "order", order)
		}
	}
	if e := sessionString(s, sessionEffortKey); e != "" {
		parts = append(parts, "thinking", e)
	}
	return strings.Join(parts, " ")
}

// teamTurn starts team-solve for this turn when the conversation is in team
// mode, and answers with the run it started. ok is false when the turn is an
// ordinary one: not in team mode, not a codebase agent, no project, no
// workflow engine, or the turn is itself a workflow step (a team never starts
// a team).
func (ar *AgentRuntime) teamTurn(ctx context.Context, userMessage string, session *Session, runID string) (*AgentResponse, bool) {
	if !sessionTeam(session) || ar.workflows == nil || strings.HasPrefix(session.ID, "workflow:") || !ar.agentWorksOnCodebase(ctx) {
		return nil, false
	}
	dir := turnWorkdir(ctx)
	if dir == "" {
		return nil, false
	}
	actor := string(authorization.GetActorID(ctx))
	agent, _ := ctx.Value("buddy_agent_name").(string)
	var text string
	run, err := ar.workflows.startWorkflow(dir, teamWorkflow, session.ID, "", actor, "", teamModelLine(session), teamPrompt(userMessage, agent), 0)
	if err != nil {
		text = fmt.Sprintf("Team mode: %s did not start: %v. Shift+Tab leaves team mode.", teamWorkflow, err)
	} else {
		snap := run.snapshot()
		// No step list here: it was every step "waiting" and never changed,
		// pushing the window's live run view off the screen (the demo take,
		// 2026-11-05). The live view draws the run; the end reports here.
		text = fmt.Sprintf("▶ Team mode: %s is on it as %s, %d steps, each role a fresh context. "+
			"It reports here when it ends, with the verdict. Shift+Tab leaves team mode.",
			teamWorkflow, snap.ID, snap.Total)
	}
	if ar.events != nil {
		ar.events.EmitEvent(runID, infra.EventStreamAssistant, session.ID, map[string]interface{}{"event": "text", "text": text})
		ar.events.EmitEvent(runID, infra.EventStreamLifecycle, session.ID, map[string]interface{}{"event": "complete", "success": err == nil, "tools_executed": 0})
		ar.events.ClearRun(runID)
	}
	return &AgentResponse{Text: text}, true
}

// teamPrompt is the person's prompt as the team's task: as typed, without the
// "@coder " the window puts before a message to its own agent (a tmux run on
// 2026-11-05: team-solve's task.md opened "@coder scripts/scan.sh must…").
func teamPrompt(message, agent string) string {
	m := strings.TrimSpace(message)
	if agent != "" {
		if rest, ok := strings.CutPrefix(m, "@"+agent); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\n') {
			m = strings.TrimSpace(rest)
		}
	}
	return m
}

// teamReportDiffLines is how much of the diff a team run's end shows; the
// rest is in changes.diff.
const teamReportDiffLines = 60

// teamReport is what a team-solve run's end adds to its note in the
// conversation (#94): the verdicts, what keep kept, the Scope report
// (`changed: scripts/scan.sh +1 -1`) and the diff, so the person sees the
// result without opening a file. "" for any other workflow.
func teamReport(run *workflowRun) string {
	if run.Workflow != teamWorkflow {
		return ""
	}
	rel := filepath.Join(".memdoor", "runs", teamWorkflow+"-"+run.Partition)
	r := filepath.Join(run.Dir, rel)
	if _, err := os.Stat(r); err != nil {
		return ""
	}
	var b strings.Builder
	var verdicts []string
	for _, f := range []struct{ who, file string }{{"judge", "verdict.md"}, {"review", "review.md"}} {
		if v := fileVerdict(filepath.Join(r, f.file)); v != "" {
			verdicts = append(verdicts, f.who+" "+v)
		}
	}
	if len(verdicts) > 0 {
		b.WriteString("\n\nverdict: " + strings.Join(verdicts, " · "))
	}
	if kept := firstLineOf(filepath.Join(r, "keep.txt")); kept != "" {
		b.WriteString("\n" + kept)
	}
	if scope, err := os.ReadFile(filepath.Join(r, "scope.txt")); err == nil && strings.TrimSpace(string(scope)) != "" {
		b.WriteString("\n\nScope:\n    " + strings.Join(strings.Split(strings.TrimRight(string(scope), "\n"), "\n"), "\n    "))
	}
	if diff, err := os.ReadFile(filepath.Join(r, "changes.diff")); err == nil && strings.TrimSpace(string(diff)) != "" {
		lines := strings.Split(strings.TrimRight(string(diff), "\n"), "\n")
		more := ""
		if len(lines) > teamReportDiffLines {
			more = fmt.Sprintf("\n… %d more lines in %s", len(lines)-teamReportDiffLines, filepath.Join(rel, "changes.diff"))
			lines = lines[:teamReportDiffLines]
		}
		// Indented, not fenced: the window prints a note's fence marks literally.
		b.WriteString("\n\nThe diff:\n    " + strings.Join(lines, "\n    ") + more)
	}
	return b.String()
}

// firstLineOf is a file's first line, trimmed; "" when it is missing or empty.
func firstLineOf(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	l, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimSpace(l)
}
