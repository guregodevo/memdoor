// Package flow implements a deterministic multi-step task-flow driver.
//
// Pattern: OpenClaw Task Flow "managed" sync mode
// (openclaw/src/tasks/task-flow-registry.types.ts + runtime-taskflow.ts).
//
// A managed flow OWNS a multi-step sequence end-to-end: deterministic code holds
// the ordered steps + current index, dispatches each step as one ephemeral doer
// subagent (e.g. the coder), waits for that child to complete, then advances to
// the next step — until the sequence finishes. The LLM only ever executes a
// single step; the loop itself is code, so a small model that can't reliably
// drive a multi-step plan never has to.
package flow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"memdoor/gateway/logs"
)

// SyncModeManaged is the only mode we implement: the flow owns the lifecycle.
const SyncModeManaged = "managed"

// maxStepAttempts bounds how many times a single step is re-dispatched when it
// fails to land (no-op or leaves the workspace broken) before the flow fails.
// The coder sometimes burns an attempt narrating before it writes, and a genuine
// compile fix can take a couple of turns, so give the fix-forward loop real room.
const maxStepAttempts = 6

// Status is the flow-level state machine (mirror of TaskFlowStatus).
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusWaiting   Status = "waiting" // dispatched a step, waiting on its child subagent
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Record is a durable, revision-tracked managed flow. Mirrors OpenClaw's
// TaskFlowRecord; StateJSON's role (the plan) is made concrete here as
// Steps + Index since our flows are fixed ordered sequences.
type Record struct {
	FlowID          string     `json:"flowId"`
	SyncMode        string     `json:"syncMode"`
	OwnerKey        string     `json:"ownerKey"` // requester session (the one channel)
	ControllerID    string     `json:"controllerId,omitempty"`
	Revision        int        `json:"revision"` // bumped on every mutation (conflict detection)
	Status          Status     `json:"status"`
	Goal            string     `json:"goal"`
	CurrentStep     string     `json:"currentStep,omitempty"`
	BlockedTaskID   string     `json:"blockedTaskId,omitempty"` // child session key we're waiting on
	Steps           []string   `json:"steps"`
	Index           int        `json:"index"`
	StepAttempts    int        `json:"stepAttempts"`             // retries used on the current step
	PreStepHash     string     `json:"preStepHash,omitempty"`    // workspace hash before the current step
	LastBuildError  string     `json:"lastBuildError,omitempty"` // compile error to feed the retry
	Workdir         string     `json:"workdir,omitempty"`
	AgentID         string     `json:"agentId"` // doer agent, e.g. "coder"
	ParentMessageID int64      `json:"parentMessageId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	EndedAt         *time.Time `json:"endedAt,omitempty"`
}

// DispatchParams is what the registry hands the Dispatcher to run one step.
type DispatchParams struct {
	FlowID          string
	OwnerKey        string
	AgentID         string
	Task            string
	Workdir         string
	ParentMessageID int64
	// BuildError, when set on a retry, is the compile error from the previous
	// attempt — the dispatcher feeds it to the coder so it can fix the CODE
	// (OpenClaw's read-error → fix → re-run).
	BuildError string
	// Attempt is 0 on the first dispatch of a step, >0 on retries.
	Attempt int
}

// Dispatcher spawns the doer subagent for a single step and returns the child
// session key it will complete under. Duck-typed and implemented by the gateway
// Server (which owns the queue + subagent registry) to avoid a circular import.
type Dispatcher interface {
	DispatchStep(p DispatchParams) (childSessionKey string, err error)
}

// Workspace lets the registry verify that a step actually landed before advancing.
// Snapshot returns a content hash of the workspace (to detect a no-op = no change);
// Build reports whether it still compiles (to reject a step that broke the code).
// Implemented by the gateway Server. Optional: a nil Workspace disables the gate.
type Workspace interface {
	Snapshot(workdir string) string
	Build(workdir string) (ok bool, detail string)
}

// Registry holds all flows and drives the managed advance loop.
type Registry struct {
	mu         sync.Mutex
	flows      map[string]*Record
	storeDir   string
	dispatcher Dispatcher
	workspace  Workspace
	log        *logs.EventLogger
}

// NewRegistry creates a flow registry that persists each flow as JSON under
// storeDir and loads any existing flows so managed state survives a restart.
// workspace may be nil to disable the verify-and-retry gate.
func NewRegistry(storeDir string, dispatcher Dispatcher, workspace Workspace) *Registry {
	r := &Registry{
		flows:      make(map[string]*Record),
		storeDir:   storeDir,
		dispatcher: dispatcher,
		workspace:  workspace,
		log:        logs.New("TaskFlow"),
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		r.log.Warn("flow store dir create failed", slog.String("dir", storeDir), slog.String("error", err.Error()))
	}
	r.load()
	return r
}

// StartManaged creates a managed flow and dispatches its first step. The steps
// are a fixed ordered plan; each must be self-contained (the doer subagents are
// isolated, so a step references shared state on disk, not prior conversation).
func (r *Registry) StartManaged(ownerKey, goal string, steps []string, agentID, workdir string, parentMsgID int64) (*Record, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("task_flow requires at least one step")
	}
	if agentID == "" {
		agentID = "coder"
	}
	now := time.Now()
	rec := &Record{
		FlowID:          "flow_" + uuid.New().String(),
		SyncMode:        SyncModeManaged,
		OwnerKey:        ownerKey,
		ControllerID:    "taskflow",
		Revision:        1,
		Status:          StatusQueued,
		Goal:            goal,
		Steps:           steps,
		Index:           0,
		Workdir:         workdir,
		AgentID:         agentID,
		ParentMessageID: parentMsgID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	r.mu.Lock()
	r.flows[rec.FlowID] = rec
	r.mu.Unlock()
	r.persist(rec)
	r.log.Info("flow created",
		slog.String("flow_id", rec.FlowID),
		slog.String("goal", goal),
		slog.Int("steps", len(steps)),
		slog.String("agent", agentID))
	if err := r.advance(rec); err != nil {
		return rec, err
	}
	return rec, nil
}

// advance dispatches the current step and moves the flow to waiting.
func (r *Registry) advance(rec *Record) error {
	// Capture the workspace state BEFORE the step so OnStepComplete can tell whether
	// the step actually changed anything (no-op guard).
	var preHash string
	if r.workspace != nil {
		preHash = r.workspace.Snapshot(rec.Workdir)
	}

	r.mu.Lock()
	step := rec.Steps[rec.Index]
	stepNo := rec.Index + 1
	total := len(rec.Steps)
	rec.Status = StatusRunning
	rec.CurrentStep = step
	rec.PreStepHash = preHash
	rec.Revision++
	rec.UpdatedAt = time.Now()
	r.mu.Unlock()

	r.log.Info("flow dispatching step",
		slog.String("flow_id", rec.FlowID),
		slog.Int("step", stepNo),
		slog.Int("total", total),
		slog.Int("attempt", rec.StepAttempts+1))

	childKey, err := r.dispatcher.DispatchStep(DispatchParams{
		FlowID:          rec.FlowID,
		OwnerKey:        rec.OwnerKey,
		AgentID:         rec.AgentID,
		Task:            step,
		Workdir:         rec.Workdir,
		ParentMessageID: rec.ParentMessageID,
		BuildError:      rec.LastBuildError,
		Attempt:         rec.StepAttempts,
	})
	if err != nil {
		r.fail(rec, fmt.Sprintf("dispatch step %d failed: %v", stepNo, err))
		return err
	}

	r.mu.Lock()
	rec.BlockedTaskID = childKey
	rec.Status = StatusWaiting
	rec.Revision++
	rec.UpdatedAt = time.Now()
	r.mu.Unlock()
	r.persist(rec)
	r.log.Info("flow waiting on step",
		slog.String("flow_id", rec.FlowID),
		slog.Int("step", stepNo),
		slog.Int("total", total),
		slog.String("child", childKey))
	return nil
}

// OnStepComplete is the completion hook: given the child session key that just
// finished, it finds the managed flow waiting on it and advances deterministically
// (next step, or finish).
//
//   - handled  is true if a flow owned this completion (the caller then skips the
//     generic announce-back); false if no flow was waiting on this key.
//   - finished is true only when that completion ended the flow (succeeded/failed).
//     The flow reuses one coder session across steps, so the caller keeps the
//     session alive between steps and cleans it up only when finished.
func (r *Registry) OnStepComplete(childSessionKey, resultText string) (handled bool, finished bool) {
	r.mu.Lock()
	var rec *Record
	for _, f := range r.flows {
		if f.Status == StatusWaiting && f.BlockedTaskID == childSessionKey {
			rec = f
			break
		}
	}
	if rec == nil {
		r.mu.Unlock()
		return false, false
	}
	stepNo := rec.Index + 1
	total := len(rec.Steps)
	workdir := rec.Workdir
	preHash := rec.PreStepHash
	rec.BlockedTaskID = ""
	r.mu.Unlock()

	// Verify-and-retry gate: a step only counts as done if it actually LANDED —
	// it changed the workspace AND left it compiling. A no-op (coder narrated
	// instead of writing) or a break re-dispatches the SAME step, bounded by
	// maxStepAttempts, then fails the flow cleanly. This is what makes the flow
	// robust regardless of coder flakiness. (OpenClaw managed fail/retry.)
	if r.workspace != nil {
		afterHash := r.workspace.Snapshot(workdir)
		built, detail := r.workspace.Build(workdir)
		changed := afterHash != preHash
		if !changed || !built {
			r.mu.Lock()
			rec.StepAttempts++
			attempts := rec.StepAttempts
			// Feed the compile error into the next attempt so the coder fixes the
			// CODE; clear it when the failure is a pure no-op (nothing to fix yet).
			if !built {
				rec.LastBuildError = detail
			} else {
				rec.LastBuildError = ""
			}
			rec.Revision++
			rec.UpdatedAt = time.Now()
			r.mu.Unlock()

			reason := "no change to workspace"
			if changed && !built {
				reason = "does not compile: " + detail
			} else if !changed && !built {
				reason = "no change and does not compile"
			}
			if attempts >= maxStepAttempts {
				r.fail(rec, fmt.Sprintf("step %d/%d did not land after %d attempts (%s)", stepNo, total, attempts, reason))
				return true, true
			}
			r.log.Warn("flow step did not land — retrying",
				slog.String("flow_id", rec.FlowID),
				slog.Int("step", stepNo),
				slog.Int("attempt", attempts),
				slog.String("reason", reason))
			// With child-session continuity the coder keeps its conversation, so the
			// retry is a follow-up turn (the compile error is fed via BuildError). No
			// rollback needed — the coder fixes forward on its own code.
			r.persist(rec)
			if err := r.advance(rec); err != nil { // re-dispatch SAME step (index unchanged)
				r.log.Warn("flow retry dispatch failed", slog.String("flow_id", rec.FlowID), slog.String("error", err.Error()))
			}
			return true, false
		}
	}

	// Step landed → advance to the next step.
	r.mu.Lock()
	rec.Index++
	rec.StepAttempts = 0
	rec.LastBuildError = ""
	rec.Revision++
	rec.UpdatedAt = time.Now()
	done := rec.Index >= len(rec.Steps)
	r.mu.Unlock()

	r.log.Info("flow step landed",
		slog.String("flow_id", rec.FlowID),
		slog.Int("step", stepNo),
		slog.Int("total", total))
	r.persist(rec)

	if done {
		r.finish(rec)
		return true, true
	}
	if err := r.advance(rec); err != nil {
		r.log.Warn("flow advance failed",
			slog.String("flow_id", rec.FlowID),
			slog.String("error", err.Error()))
	}
	return true, false
}

func (r *Registry) finish(rec *Record) {
	now := time.Now()
	r.mu.Lock()
	rec.Status = StatusSucceeded
	rec.CurrentStep = ""
	rec.EndedAt = &now
	rec.Revision++
	rec.UpdatedAt = now
	r.mu.Unlock()
	r.persist(rec)
	r.log.Info("flow succeeded",
		slog.String("flow_id", rec.FlowID),
		slog.String("goal", rec.Goal),
		slog.Int("steps", len(rec.Steps)))
}

func (r *Registry) fail(rec *Record, reason string) {
	now := time.Now()
	r.mu.Lock()
	rec.Status = StatusFailed
	rec.EndedAt = &now
	rec.Revision++
	rec.UpdatedAt = now
	r.mu.Unlock()
	r.persist(rec)
	r.log.Warn("flow failed",
		slog.String("flow_id", rec.FlowID),
		slog.String("reason", reason))
}

// Get returns a flow by id.
func (r *Registry) Get(flowID string) (*Record, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.flows[flowID]
	return rec, ok
}

// List returns all flows (newest first is not guaranteed; caller may sort).
func (r *Registry) List() []*Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Record, 0, len(r.flows))
	for _, f := range r.flows {
		out = append(out, f)
	}
	return out
}

func (r *Registry) persist(rec *Record) {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		r.log.Warn("flow marshal failed", slog.String("flow_id", rec.FlowID), slog.String("error", err.Error()))
		return
	}
	path := filepath.Join(r.storeDir, rec.FlowID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		r.log.Warn("flow persist failed", slog.String("flow_id", rec.FlowID), slog.String("error", err.Error()))
	}
}

func (r *Registry) load() {
	entries, err := os.ReadDir(r.storeDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.storeDir, e.Name()))
		if err != nil {
			continue
		}
		var rec Record
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		r.flows[rec.FlowID] = &rec
	}
	if len(r.flows) > 0 {
		r.log.Debug("flows loaded", slog.Int("count", len(r.flows)))
	}
}
