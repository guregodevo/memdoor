package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// APPROVAL MODE (Greg, 2026-10-02). Memdoor is always-auto: no permission
// modes, the picker is the only interruption. A company that approves
// coding agents forbids exactly that — "avoid the default yolo mode" — so
// this is the one mode beside it: before a tool that changes or runs
// something, the person is asked, through the same picker the agent uses
// for its own questions. The answer "always" keeps that tool quiet for the
// rest of the session; "no" ends the call with a result the model can act
// on. Nothing else about the turn changes.
//
// On by the workspace setting `approve` or the environment variable
// MEMDOOR_APPROVE (a company's preset wins over the setting): "changes" asks
// before bash, every file write and every MCP tool; "off" (the default)
// asks nothing. Read per call, so switching it needs no restart.
const (
	settingApprove  = "approve"
	approveChanges  = "changes"
	approveOff      = "off"
	approvalTimeout = 5 * time.Minute
)

const (
	approveYes    = "Yes"
	approveAlways = "Yes, and don't ask again for this tool this session"
	approveNo     = "No"
)

// approvalTools are the tools that change or run something; a tool named
// mcp__<server>__<tool> is one too (it reaches outside the project).
var approvalTools = map[string]bool{"bash": true, "apply_patch": true, "write_file": true, "edit_file": true}

func needsApproval(tool string) bool {
	return approvalTools[tool] || strings.HasPrefix(tool, "mcp__")
}

// approvalMode is the mode in force: the environment first (a company's
// golden preset), then the workspace setting.
func (ar *AgentRuntime) approvalMode() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("MEMDOOR_APPROVE"))); v != "" {
		return v
	}
	if ar.workspaceSetting != nil {
		return strings.ToLower(strings.TrimSpace(ar.workspaceSetting(settingApprove)))
	}
	return approveOff
}

// approvals remembers "always" per session and tool. Sessions are
// long-lived; the map is small and lives with the runtime.
type approvals struct {
	mu     sync.Mutex
	always map[string]map[string]bool // session -> tool -> true
}

func (a *approvals) granted(session, tool string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.always != nil && a.always[session][tool]
}

func (a *approvals) grant(session, tool string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.always == nil {
		a.always = map[string]map[string]bool{}
	}
	if a.always[session] == nil {
		a.always[session] = map[string]bool{}
	}
	a.always[session][tool] = true
}

var patchFileLine = regexp.MustCompile(`(?m)^\*\*\* (?:Add|Update|Delete) File: (.+)$`)

// approvalSummary is what the person is asked about: the command, the
// files a patch touches, the path a write goes to, the MCP tool's input.
func approvalSummary(tool string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	str := func(k string) string { s, _ := in[k].(string); return strings.TrimSpace(s) }
	switch tool {
	case "bash":
		return "Run: " + oneLine(str("command"), 160)
	case "apply_patch":
		var files []string
		for _, m := range patchFileLine.FindAllStringSubmatch(str("input"), -1) {
			files = append(files, strings.TrimSpace(m[1]))
		}
		if len(files) == 0 {
			return "Apply a patch"
		}
		return "Change: " + oneLine(strings.Join(files, ", "), 160)
	case "write_file", "edit_file":
		if p := str("path"); p != "" {
			return "Write: " + p
		}
		return "Write a file"
	}
	if strings.HasPrefix(tool, "mcp__") {
		parts := strings.SplitN(tool, "__", 3)
		if len(parts) == 3 {
			return fmt.Sprintf("Call %s on %s: %s", parts[2], parts[1], oneLine(string(input), 120))
		}
	}
	return tool + ": " + oneLine(string(input), 120)
}

// askApproval is the gate: nil means go ahead. It asks through the picker
// (the "question" event the TUI already renders) and waits for the answer
// the WebSocket hands back; no answer in five minutes is a no, so a turn
// with no one watching does not run the change unasked.
func (ar *AgentRuntime) askApproval(ctx context.Context, toolUse *llm.ToolUseBlock, runID string, session *Session, log *logs.EventLogger) error {
	if ar.approvalMode() != approveChanges || !needsApproval(toolUse.Name) || ar.approvals.granted(session.ID, toolUse.Name) {
		return nil
	}
	summary := approvalSummary(toolUse.Name, toolUse.Input)
	qid := generateID("q")
	ch, cancel := ar.questions.register(qid)
	defer cancel()
	ar.events.EmitEvent(runID, "question", session.ID, map[string]interface{}{
		"question_id": qid,
		"question":    summary + " — allow?",
		"options":     []string{approveYes, approveAlways, approveNo},
		"default":     approveYes,
		"approval":    toolUse.Name,
	})
	log.Info("Approval asked", slog.String("tool", toolUse.Name), slog.String("what", truncateForLog(summary, 120)))
	var answer string
	select {
	case answer = <-ch:
	case <-ctx.Done():
		return fmt.Errorf("%s was not run: the turn ended before the person could allow it", toolUse.Name)
	case <-time.After(approvalTimeout):
		log.Warn("Approval not answered", slog.String("tool", toolUse.Name))
		return fmt.Errorf("%s was not run: nobody allowed it within %s (approval mode is on). Stop and tell the person what you wanted to do", toolUse.Name, approvalTimeout)
	}
	switch answer {
	case approveAlways:
		ar.approvals.grant(session.ID, toolUse.Name)
		fallthrough
	case approveYes:
		log.Info("Approval given", slog.String("tool", toolUse.Name), slog.Bool("always", answer == approveAlways))
		return nil
	}
	log.Info("Approval refused", slog.String("tool", toolUse.Name))
	return fmt.Errorf("%s was not run: the person said no to \"%s\". Do not retry it; ask what they want instead, or do it another way", toolUse.Name, summary)
}
