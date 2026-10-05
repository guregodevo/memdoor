package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"memdoor/gateway/flow"
	"memdoor/gateway/logs"
	"memdoor/gateway/subagents"
	"memdoor/pkg/coding/buildscope"
)

// flowStepTimeout bounds how long a single flow step's coder turn may run before
// the flow treats it as stalled. A normal step finishes in well under a minute; a
// decode loop on the model's post-write turn would otherwise hang the whole flow.
const flowStepTimeout = 180 * time.Second

// StartFlow implements tools.FlowStarter: the task_flow tool calls this to start
// a managed flow owned by the calling session. The flow registry then drives the
// steps deterministically.
func (s *Server) StartFlow(ownerKey, goal string, steps []string, parentMessageID int64) (string, error) {
	if s.flowRegistry == nil {
		return "", fmt.Errorf("flow registry not initialized")
	}
	rec, err := s.flowRegistry.StartManaged(ownerKey, goal, steps, "coder", coderWorkdir(), parentMessageID)
	if err != nil {
		return "", err
	}
	return rec.FlowID, nil
}

// DispatchStep implements flow.Dispatcher: it runs one flow step on the coder using
// OpenClaw child-session CONTINUITY — every step of a flow reuses the SAME stable
// child session key, so the coder's conversation persists across steps. The coder
// remembers what it built, so "now add drop()" is it continuing its own work; there
// is no eager state injection. Re-running the (already-finalized) session requires
// reactivation first (see ReactivateChildSession) or the lifecycle stalls.
func (s *Server) DispatchStep(p flow.DispatchParams) (string, error) {
	// Stable per-flow child session key — reused across steps (continuity).
	childSessionKey := fmt.Sprintf("agent:%s:subagent:%s", p.AgentID, p.FlowID)

	if s.resolveAgentConfig == nil {
		return "", fmt.Errorf("cannot dispatch flow step: agent config resolver not wired")
	}
	agentTools, prompt := s.resolveAgentConfig(p.AgentID)
	if len(agentTools) == 0 {
		return "", fmt.Errorf("cannot dispatch flow step: agent %q palette could not be resolved", p.AgentID)
	}

	// Reactivation: clear the prior finalized run bound to this session so re-running
	// it starts a clean lifecycle instead of stalling on a shadowed/deduped run.
	s.subagentRegistry.ReactivateChildSession(childSessionKey)

	runID := uuid.New().String()
	record := &subagents.SubagentRunRecord{
		RunID:               runID,
		ChildSessionKey:     childSessionKey,
		RequesterSessionKey: p.OwnerKey,
		RequesterDisplayKey: "TaskFlow " + p.FlowID,
		Task:                p.Task,
		Cleanup:             "keep", // reused across steps; the flow cleans up on finish
		Label:               "flow-step",
		ParentMessageID:     p.ParentMessageID,
		CreatedAt:           time.Now(),
		ArchiveAtMs:         time.Now().Add(24 * time.Hour).UnixMilli(),
	}
	if err := s.subagentRegistry.Register(record); err != nil {
		return "", fmt.Errorf("register flow step run: %w", err)
	}

	// The coder remembers prior steps via the session (continuity), so the message
	// is just the new instruction — or, on a compile-failure retry, the error to
	// fix. No re-stated context or directives are injected.
	message := p.Task
	if p.BuildError != "" {
		message = "That change does not compile. Fix it:\n" + p.BuildError
	}

	// A flow step runs in the server default directory, as it did before spawn
	// learned to carry the requester's (2026-09-02); threading the owner's turn
	// directory through the flow is a follow-up.
	if err := s.queueManager.EnqueueSubagentJob(runID, childSessionKey, p.AgentID, message, agentTools, prompt, "", flowStepTimeout); err != nil {
		return "", fmt.Errorf("enqueue flow step: %w", err)
	}

	// Port of OpenClaw waitForSubagentCompletion (subagent-registry-run-manager.ts):
	// fire a background waiter right after the run is registered so a stalled step
	// times out and drives completion instead of hanging the flow.
	go s.waitForFlowStepCompletion(runID, childSessionKey)
	return childSessionKey, nil
}

// waitForFlowStepCompletion waits for a flow step's run to end; if it stalls past
// flowStepTimeout (e.g. the model decode-looping on its post-write turn), it cancels
// the hung coder and drives completion anyway, so the flow verifies what's on disk
// and advances or retries rather than hanging. Port of OpenClaw's
// waitForSubagentCompletion: wait-for-run → on timeout, end the run and complete it.
func (s *Server) waitForFlowStepCompletion(runID, childSessionKey string) {
	if snap := s.subagentRegistry.WaitForRun(runID, flowStepTimeout); snap != nil {
		return // ended normally; inline handleSubagentCompletion already drove the flow
	}
	// Timed out. Distinguish a genuine stall from a completed-then-reactivated run:
	// if the record is gone (reactivation deleted it on the next dispatch) or already
	// marked ended, the step completed — nothing to do.
	if run, ok := s.subagentRegistry.Get(runID); !ok || run.EndedAt != nil {
		return
	}
	log := logs.New("TaskFlow")
	log.Warn("flow step stalled — timing out and driving completion",
		slog.String("run_id", runID), slog.String("child_session", childSessionKey))
	// Cancel the hung coder turn (stops the loop, frees the session), mark the run
	// ended, then drive the flow's completion path (verify → advance/retry).
	s.cancelRun(childSessionKey)
	s.subagentRegistry.MarkEnded(childSessionKey)
	if err := s.handleSubagentCompletion(context.Background(), childSessionKey, &AgentResponse{}); err != nil {
		log.WithError(err).Debug("drive completion after flow-step timeout failed")
	}
}

// Snapshot implements flow.Workspace: a content hash of the workdir's code files,
// so the registry can tell whether a step actually changed anything (no-op guard).
func (s *Server) Snapshot(workdir string) string {
	files := goFiles(workdir)
	h := sha256.New()
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(workdir, name))
		if err != nil {
			continue
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Build implements flow.Workspace: does the workspace still compile? A step that
// leaves the code broken is treated as not-landed and retried.
//
// The build is scoped to the package(s) the step actually changed (via git), so
// on a large multi-package repo the coder sees only errors from what it touched,
// not the whole module. A flat single-dir workspace (the Tetris case) has no git
// change set, so it falls back to `go build ./...` — unchanged behavior.
func (s *Server) Build(workdir string) (bool, string) {
	if len(goFiles(workdir)) == 0 {
		return false, "no .go files in workspace"
	}
	cmd := exec.Command("go", scopedBuildArgs(workdir)...)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = err.Error()
		}
		if len(detail) > 400 {
			detail = detail[:400]
		}
		return false, detail
	}
	return true, ""
}

// scopedBuildArgs returns the `go` args to build workdir, scoped to the Go
// packages changed there (derived from git) so the verify step's errors stay
// focused. Falls back to `build ./...` when the change set can't be derived
// (not a git repo, no changes, or too many packages — buildscope's own guard).
func scopedBuildArgs(workdir string) []string {
	changed := changedGoFiles(workdir)
	if len(changed) == 0 {
		return []string{"build", "./..."}
	}
	return buildscope.Derive(workdir, changed).BuildArgs()
}

// changedGoFiles returns the repo-relative .go files that git reports as changed
// (modified, added, or untracked) in workdir, or nil when workdir isn't a git
// repo or nothing changed.
func changedGoFiles(workdir string) []string {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = workdir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		// Porcelain: "XY <path>" (or "XY <old> -> <new>" for renames).
		path := strings.TrimSpace(line[3:])
		if i := strings.LastIndex(path, " -> "); i >= 0 {
			path = path[i+len(" -> "):]
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
	}
	return files
}

// goFiles returns the sorted names of .go files directly in workdir.
func goFiles(workdir string) []string {
	if workdir == "" {
		return nil
	}
	entries, err := os.ReadDir(workdir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}
