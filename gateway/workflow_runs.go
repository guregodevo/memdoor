package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/gateway/infra"
	"memdoor/gateway/providers"
	"memdoor/pkg/authorization"
	"memdoor/pkg/llm"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/guregodevo/mario/sqlite"
	"github.com/guregodevo/mario/tasks"
	mario "github.com/guregodevo/mario/workflow"

	"memdoor/gateway/broadcast"
	"memdoor/gateway/logs"
	"memdoor/gateway/queue"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/pkg/workflow"
)

// THE ORCHESTRATOR (ADR-0019; Greg, 2026-10-02: "cron is the schedule.
// workflow + mario is the orchestrator"). A workflow is .memdoor/workflows/<name>.yaml
// in the project the TUI was launched from: a DAG of tasks, each one bounded
// turn of an agent, each done when its target exists. pkg/workflow runs it
// on mario's engine; this file is what the gateway adds — the Runner (a task
// is one turn through the queue, as the task's agent, in the project), the
// events into the window that asked, and the HTTP surface the TUI and CLI
// drive: run, status, stop, approve, list, files.

// workflowRuns is every run this gateway knows, newest last. In memory: a
// run is bounded (hours at most) and the receipt is the log and the window;
// a run table comes with the second workflow.
type workflowRuns struct {
	mu   sync.Mutex
	runs map[string]*workflowRun
}

type workflowRun struct {
	ID        string
	Workflow  string
	Dir       string
	Session   string // the TUI conversation that asked
	Workspace string // whose windows see the run's events (BroadcastToWorkspace)
	Actor     string // who started it: its tasks start on that person's kept pin
	WF        workflow.Workflow
	Partition string // mario's partition: fresh per run (its start time); naming an earlier one resumes it
	cancel    context.CancelFunc
	timeout   time.Duration
	mu        sync.Mutex
	exe       *workflow.Execution // mario's record of the current trigger: every task's state is read from it
	last      workflow.Result     // the last trigger's end, for a run between triggers (waiting at a gate)
	state     string              // running, waiting (at an external target), done, failed, stopped — the RUN's, which mario has no word for
	started   time.Time
	ended     time.Time
	finalErr  string
	// receipts is each agent task's receipt line (turn_done.go), for the run
	// view: what proved the step, not only that it is done. Display only —
	// the task's state stays mario's.
	receipts map[string]string
	// asked is each agent task's last TaskRun as mario rendered it, so a
	// reviewer's change request can rerun the step with its own prompt.
	asked map[string]workflow.TaskRun
	// base is the commit the run started from, so a gate can show what the
	// run changed (the review loop, 2026-10-03); "" outside a git repository.
	base string
}

// newID names a run by its workflow and the second it started; two in the
// same second get a suffix (found live 2026-10-02: an ad-hoc run and a slow
// one started together and one overwrote the other). Called with w.mu held.
func (w *workflowRuns) newID(workflow string) string {
	base := workflow + "-" + time.Now().Format("20060102-150405")
	id := base
	for n := 2; ; n++ {
		if _, taken := w.runs[id]; !taken {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

func (s *Server) workflows() *workflowRuns {
	s.workflowOnce.Do(func() { s.workflowRunsState = &workflowRuns{runs: map[string]*workflowRun{}} })
	return s.workflowRunsState
}

// ---- progress: what a task's turn is doing, relayed into the window ---------

// The agent runtime emits every event of every turn through one emitter
// (infra.EventEmitter, a callback seam); a run listens for the tool calls
// of ITS tasks' turns — a turn runs under "workflow:<wf>:<partition>:<task>"
// — and relays them into the window as a progress line under the task, so a
// running task is not a frozen ▶ for two minutes (Greg: "more events
// emitted and updates in real time").
func (s *Server) listen(run *workflowRun) func() {
	if s.broadcaster == nil || s.agent == nil || s.agent.events == nil {
		return func() {}
	}
	return s.agent.events.OnEvent(func(ev infra.AgentEvent) {
		if ev.Stream != infra.EventStreamTool {
			return
		}
		task := run.taskOfSession(ev.SessionID)
		if task == "" {
			return
		}
		tool, _ := ev.Data["tool"].(string)
		if tool == "" {
			tool, _ = ev.Data["name"].(string)
		}
		if tool == "" {
			return
		}
		line := "⏺ " + tool
		if secs, ok := ev.Data["seconds"].(int); ok && secs >= 2 {
			line += fmt.Sprintf(" · %ds", secs)
		}
		s.sendRunEvent(run, map[string]interface{}{"run": run.ID, "workflow": run.Workflow, "task": task, "state": "progress", "text": line})
	})
}

// taskSession is the session a task's turn runs under; taskOfSession reads it back.
func (r *workflowRun) taskSession(task string) string {
	return "workflow:" + r.Workflow + ":" + r.Partition + ":" + task
}

func (r *workflowRun) taskOfSession(session string) string {
	prefix := "workflow:" + r.Workflow + ":" + r.Partition + ":"
	if strings.HasPrefix(session, prefix) {
		return strings.TrimPrefix(session, prefix)
	}
	return ""
}

// ---- the task types this gateway offers ------------------------------------

// workflowTypes is what a DAG on this gateway may be made of: an agent turn
// (Memdoor's own type, run through the queue), mario's command, and
// mario's llm on this gateway's model. The YAML names the type; the
// factory behind it builds the task.
func (s *Server) workflowTypes(runner workflow.Runner) []tasks.Base {
	return []tasks.Base{workflow.Agent(runner), tasks.Command(), tasks.LLM(workflowCompleter{s: s})}
}

// workflowCompleter is one model call through the provider (factory.go):
// what an llm task is behind the YAML. It takes the same road as /handoff
// and /model check, so the model's reasoning is configured, the LLM log line
// written and a reasoning-only reply read back.
type workflowCompleter struct{ s *Server }

// workflowModelAgent is whose ladder an llm task climbs when its YAML names
// no model: the coder's cheapest rung, as a summary does.
const workflowModelAgent = "coder"

func (c workflowCompleter) Complete(ctx context.Context, model, prompt string) (string, error) {
	if c.s.clientFactory == nil {
		return "", fmt.Errorf("no model client on this gateway")
	}
	ctx = cheapestRung(ctx)
	if model = strings.TrimSpace(model); model != "" {
		ctx = context.WithValue(ctx, sharedctx.ModelKey, model)
	}
	ctx = context.WithValue(ctx, "buddy_agent_name", workflowModelAgent) //nolint:staticcheck // the key the agent runtime reads
	client, err := c.s.clientFactory.GetClientFor(ctx, workflowModelAgent)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(llm.WithUtilityCall(ctx), 10*time.Minute)
	defer cancel()
	resp, err := client.Messages().New(ctx, llm.MessageNewParams{
		Model:     llm.Model(providers.ModelName),
		Messages:  []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock(prompt))},
		MaxTokens: maxOutputTokens(model),
		Agent:     "workflow",
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, part := range resp.Content {
		if part.Type == "text" {
			b.WriteString(part.Text)
		}
	}
	if text := strings.TrimSpace(b.String()); text != "" {
		return text, nil
	}
	if resp.StopReason == llm.StopReasonMaxTokens {
		return "", fmt.Errorf("the model used its whole reply on reasoning and wrote nothing")
	}
	return "", fmt.Errorf("the model wrote nothing")
}

// ---- the Runner: a task is one bounded agent turn ---------------------------

// workflowRunner satisfies workflow.Runner for one run.
type workflowRunner struct {
	s   *Server
	run *workflowRun
}

func (r *workflowRunner) Run(ctx context.Context, t workflow.TaskRun) (string, error) {
	log := logs.New("Workflow")
	r.run.mu.Lock()
	if r.run.asked == nil {
		r.run.asked = map[string]workflow.TaskRun{}
	}
	if _, kept := r.run.asked[t.Task.Name]; !kept || !strings.Contains(t.Prompt, changesRequestedMark) {
		r.run.asked[t.Task.Name] = t
	}
	r.run.mu.Unlock()
	agentID := t.Task.Agent
	if agentID == "" {
		agentID = "coder"
	}
	var buddyTools []string
	if r.s.repoFactory != nil {
		if buddy, err := r.s.repoFactory.Buddies().GetByName(ctx, agentID); err == nil && buddy != nil {
			buddyTools = append([]string(nil), buddy.Tools...)
		}
	}
	if len(buddyTools) == 0 {
		return "", fmt.Errorf("no agent named %q on this gateway (memdoor agent list)", agentID)
	}

	// The prompt names the workflow and what proves the task, so the agent
	// works toward the target rather than toward a sentence.
	var b strings.Builder
	fmt.Fprintf(&b, "Workflow %s, task %s.\n\n%s\n", t.Workflow, t.Task.Name, strings.TrimSpace(t.Prompt))
	switch {
	case strings.HasPrefix(t.Task.Target, "file "):
		fmt.Fprintf(&b, "\nThis task is done when the file %s exists in the project. Produce it.", strings.TrimPrefix(t.Task.Target, "file "))
	case strings.HasPrefix(t.Task.Target, "command: "):
		fmt.Fprintf(&b, "\nThis task is done when this command exits 0 in the project: %s\nMake that true.", strings.TrimPrefix(t.Task.Target, "command: "))
	default:
		b.WriteString("\nYour answer is kept as this task's result; the tasks after it read it.")
	}

	// The task's target is the turn's definition of done (turn_done.go
	// doneWhen): the turn checks the artifact instead of judging its
	// receipts, and continues while it is missing. `model:` on the task pins
	// its session, as /model does a conversation (providers/registry.go).
	if r.s.sessions != nil {
		if sess, err := r.s.sessions.GetOrCreateSession(r.run.taskSession(t.Task.Name), "main"); err == nil {
			if strings.HasPrefix(t.Task.Target, "file ") || strings.HasPrefix(t.Task.Target, "command: ") {
				// The target as mario checks it: its template rendered with
				// the partition (live 2026-10-03: "reports/peer-comps-
				// {{.partition}}.md missing" beside the file mario found).
				sess.SetMetadata(doneWhenKey, renderPartition(t.Task.Target, t.Partition))
			}
			if t.Task.Budget > 0 {
				sess.SetMetadata(turnBudgetKey, t.Task.Budget)
			}
			if err := pinTaskModel(sess, t.Task.Model, r.run.Actor); err != nil {
				return "", fmt.Errorf("task %s: %w", t.Task.Name, err)
			}
		}
	}

	if t.Task.OutputSchema != "" {
		// A TYPED ANSWER (output_schema, borrowed from Goose recipes'
		// response.json_schema, 2026-10-04): the steps after this one read
		// data, not prose.
		fmt.Fprintf(&b, "\n\nEnd your reply with one JSON object in a ```json block that matches this JSON Schema; it is this task's output and the next steps read it:\n%s", t.Task.OutputSchema)
	}

	turn := func(message string) (*AgentResponse, error) {
		responseChan := make(chan *AgentResponse, 1)
		errorChan := make(chan error, 1)
		job := &queue.AgentJob{
			SessionKey:  r.run.taskSession(t.Task.Name),
			Message:     message,
			GlobalLane:  queue.LaneMain,
			EnqueueTime: time.Now(),
			Context:     ctx,
			AgentID:     agentID,
			BuddyTools:  buddyTools,
			Workdir:     t.Workdir,
			ResponseWriter: func(resp interface{}) error {
				switch v := resp.(type) {
				case *AgentResponse:
					responseChan <- v
				case error:
					errorChan <- v
				default:
					errorChan <- fmt.Errorf("invalid response type: %T", resp)
				}
				return nil
			},
		}
		log.Info("Workflow task turn",
			slog.String("workflow", t.Workflow), slog.String("partition", t.Partition), slog.String("task", t.Task.Name),
			slog.String("agent", agentID), slog.String("workdir", t.Workdir))
		if err := r.s.queueManager.EnqueueJob(job); err != nil {
			return nil, fmt.Errorf("could not start the turn: %w", err)
		}
		select {
		case resp := <-responseChan:
			if resp.Error != "" {
				return nil, fmt.Errorf("%s", resp.Error)
			}
			return resp, nil
		case err := <-errorChan:
			return nil, err
		case <-ctx.Done():
			// The run's clock or a stop: the turn keeps the same ctx and
			// ends on its own; this task is over.
			return nil, ctx.Err()
		case <-time.After(jobWaitTimeout()):
			return nil, fmt.Errorf("the turn did not answer in time")
		}
	}

	resp, err := turn(b.String())
	if err != nil {
		return "", err
	}
	answer := resp.Text
	if t.Task.OutputSchema != "" {
		// The JSON is checked against the schema; a reply that does not
		// match is sent back with the validator's own reasons, twice at most.
		var verr error
		for attempt := 0; ; attempt++ {
			var doc string
			doc, verr = matchOutputSchema(t.Task.OutputSchema, resp.Text)
			if verr == nil {
				answer = doc
				break
			}
			if attempt == outputSchemaRetries {
				return "", fmt.Errorf("task %s: its answer does not match output_schema: %v", t.Task.Name, verr)
			}
			r.run.note(r.s, fmt.Sprintf("⚖ %s — its JSON does not match output_schema (%s); asking again", t.Task.Name, oneLine(verr.Error(), 120)))
			if resp, err = turn("Your JSON output does not match the schema: " + verr.Error() + "\nReply with only the corrected JSON object in a ```json block."); err != nil {
				return "", err
			}
		}
	}
	if receipt := receiptOf(resp.Text); receipt != "" {
		r.run.mu.Lock()
		if r.run.receipts == nil {
			r.run.receipts = map[string]string{}
		}
		r.run.receipts[t.Task.Name] = receipt
		r.run.mu.Unlock()
		r.run.note(r.s, fmt.Sprintf("▸ %s — %s", t.Task.Name, receipt))
	} else {
		r.run.note(r.s, fmt.Sprintf("▸ %s — %s", t.Task.Name, oneLine(answer, 120)))
	}
	return answer, nil
}

// ---- events: mario's callbacks, into the log and the window that asked ----

type workflowEvents struct {
	s   *Server
	run *workflowRun
}

func (e workflowEvents) TaskStarted(wf, task string) { e.run.emit(e.s, task, "started", "", false) }
func (e workflowEvents) TaskDone(wf, task string)    { e.run.emit(e.s, task, "done", "", false) }
func (e workflowEvents) TaskSkipped(wf, task string) { e.run.emit(e.s, task, "skipped", "", false) }
func (e workflowEvents) TaskFailed(wf, task string, err error, willRetry bool) {
	state := "failed"
	if willRetry {
		state = "retrying"
	}
	e.run.emit(e.s, task, state, err.Error(), willRetry)
}

// emit sends one transition to the log and the window, with the run's whole
// state as mario records it; the TUI redraws the DAG from that.
func (r *workflowRun) emit(s *Server, task, state, errText string, retry bool) {
	// A task killed because the person stopped the run did not fail.
	if state == "failed" {
		r.mu.Lock()
		if r.state == "stopping" || r.state == "stopped" {
			state, errText = "stopped", ""
		}
		r.mu.Unlock()
	}
	glyph := map[string]string{"started": "▶", "done": "✓", "skipped": "✓", "failed": "✗", "stopped": "■", "retrying": "↻"}[state]
	line := fmt.Sprintf("%s %s.%s %s", glyph, r.Workflow, task, state)
	if errText != "" {
		line += " — " + oneLine(errText, 160)
	}
	logs.New("Workflow").Info(line, slog.String("run", r.ID), slog.String("task", task), slog.String("state", state))
	s.sendRunEvent(r, map[string]interface{}{
		"run": r.ID, "workflow": r.Workflow, "task": task, "state": state,
		"error": errText, "retry": retry, "text": line,
		"status": r.snapshot(),
	})
}

// sendRunEvent delivers a run's event to every window of the run's workspace,
// with its project; each window keeps the runs of the project it was
// launched in. Not to the starting window's session alone — that window may
// be gone, closed or restarted, and the run then reported to nobody (dogfood
// 2026-10-04) — and not to every window either, which on a shared gateway
// showed one user's runs to another (review, 2026-10-04).
func (s *Server) sendRunEvent(r *workflowRun, data map[string]interface{}) {
	if s.broadcaster == nil {
		return
	}
	data["dir"] = r.Dir
	ev := broadcast.Event{Type: "workflow_event", Data: data}
	if r.Workspace != "" {
		s.broadcaster.BroadcastToWorkspace(r.Workspace, ev)
		return
	}
	if r.Session != "" {
		s.broadcaster.BroadcastToSession(r.Session, ev)
	}
}

// runWorkspace is the workspace a run reports to: the one named, else the
// starting window's.
func runWorkspace(workspace, session string) string {
	if w := strings.TrimSpace(workspace); w != "" {
		return w
	}
	return shared.ParseSessionID(session).WorkspaceID
}

// note is a sentence for the window, not a transition.
func (r *workflowRun) note(s *Server, text string) {
	s.sendRunEvent(r, map[string]interface{}{"run": r.ID, "workflow": r.Workflow, "state": "note", "text": text, "status": r.snapshot()})
}

// ---- the JSON the TUI and CLI read -----------------------------------------

type workflowTaskJSON struct {
	Name     string     `json:"name"`
	Agent    string     `json:"agent"`
	Requires []string   `json:"requires"`
	Target   string     `json:"target"`
	External bool       `json:"external"`
	State    string     `json:"state"`
	Error    string     `json:"error,omitempty"`
	Receipt  string     `json:"receipt,omitempty"` // what proved it: the turn's receipt line
	Attempt  int        `json:"attempt,omitempty"`
	Took     string     `json:"took,omitempty"`
	Started  *time.Time `json:"started,omitempty"` // the window ticks a running task's time from it
}

type workflowRunJSON struct {
	ID        string             `json:"id"`
	Workflow  string             `json:"workflow"`
	Dir       string             `json:"dir"`
	Partition string             `json:"partition"`
	State     string             `json:"state"`
	Started   time.Time          `json:"started"`
	Ended     *time.Time         `json:"ended,omitempty"`
	Error     string             `json:"error,omitempty"`
	Tasks     []workflowTaskJSON `json:"tasks"`
	Done      int                `json:"done"`
	Total     int                `json:"total"`
	// Review is what a person at a gate looks at: the commits and the files
	// the run changed since it started (git), empty when not waiting.
	Review string `json:"review,omitempty"`
	// Results are the files the run's tasks produced (their file targets,
	// in the workflow's order), once it is done: what the person came for
	// (2026-10-04: "we dont see the result").
	Results []string `json:"results,omitempty"`
}

// status is the run's state as mario records it: the current trigger's
// when one is going, the last trigger's end otherwise.
func (r *workflowRun) status() workflow.Result {
	r.mu.Lock()
	exe := r.exe
	last := r.last
	r.mu.Unlock()
	if exe != nil {
		return exe.Status()
	}
	return last
}

func (r *workflowRun) snapshot() workflowRunJSON {
	res := r.status()
	r.mu.Lock()
	out := workflowRunJSON{ID: r.ID, Workflow: r.Workflow, Dir: r.Dir, Partition: r.Partition, State: r.state, Started: r.started, Error: r.finalErr, Total: len(r.WF.Tasks)}
	if !r.ended.IsZero() {
		e := r.ended
		out.Ended = &e
	}
	if out.State == "waiting" {
		out.Review = reviewOf(r.Dir, r.base, r.reviewSince())
	}
	if out.State == "done" {
		out.Results = resultFiles(r.Dir, r.Partition, r.WF)
	}
	receipts := make(map[string]string, len(r.receipts))
	for k, v := range r.receipts {
		receipts[k] = v
	}
	r.mu.Unlock()
	for _, t := range r.WF.Tasks {
		tj := workflowTaskJSON{Name: t.Name, Agent: t.Agent, Requires: t.Requires, External: t.External, State: "waiting", Target: t.Target, Receipt: receipts[t.Name]}
		if tr, ok := res.Tasks[t.Name]; ok {
			tj.State, tj.Error, tj.Attempt = string(tr.Status), tr.Error, tr.Attempt
			if tr.Took > 0 {
				tj.Took = tr.Took.Round(time.Second).String()
			}
			if !tr.Started.IsZero() {
				st := tr.Started
				tj.Started = &st
			}
		}
		if tj.State == "failed" && (out.State == "stopped" || out.State == "stopping") {
			// Killed by the stop the person asked for, not a failure (a
			// stopped run read "✗ long failed — signal: killed", 2026-10-04).
			tj.State, tj.Error = "stopped", ""
		}
		if tj.State == "done" || tj.State == "skipped" {
			out.Done++
		}
		out.Tasks = append(out.Tasks, tj)
	}
	return out
}

// ---- starting, stopping, approving ------------------------------------------

// startWorkflow loads a DAG and runs it in the project at dir; events go to
// the session (the TUI conversation that asked). name is a workflow of the
// project (.memdoor/workflows/<name>/) or a path to any directory of task YAML — one
// a person committed, or one an agent just wrote for this run.
// workflowPartition names the partition a run is for. A run is fresh by
// default (Greg: "fresh is the default") — its own partition, the moment it
// started, so nothing from an earlier run is taken as done. Naming an
// earlier run's partition resumes it: what it finished is skipped, what it
// did not runs. "today" is the day, for a DAG meant to run once a day.
func workflowPartition(asked string) string {
	switch strings.TrimSpace(asked) {
	case "", "fresh", "new":
		return time.Now().Format("2006-01-02T150405")
	case "today":
		return time.Now().Format("2006-01-02")
	}
	return strings.TrimSpace(asked)
}

// LOCAL RUNS ARE FREE (Greg, 2026-10-04: "everyone can run schedule with
// local cron"; Pro is the hosted scheduler — schedules with the laptop off
// and external dependencies across workspaces). The 2026-10-02 seat check
// on starting a run is gone.

func (s *Server) startWorkflow(dir, name, session, workspace, actor, partition string, timeout time.Duration) (*workflowRun, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("a workflow runs in a project: give its absolute directory")
	}
	path, err := workflow.Find(dir, name)
	if err != nil {
		return nil, err
	}
	spec, err := workflow.Load(path, workflow.Registry(s.workflowTypes(nil)...))
	if err != nil {
		return nil, err
	}
	// Where the state lives is settled before the run exists: a Pro run
	// whose store does not answer is refused here, in words, not recorded.
	if _, err := s.runRepository(dir); err != nil {
		return nil, err
	}
	w := s.workflows()
	w.mu.Lock()
	for _, r := range w.runs {
		if r.Workflow == spec.Name && r.Dir == dir && (r.state == "running" || r.state == "waiting") {
			w.mu.Unlock()
			return nil, fmt.Errorf("%s is already running here as %s — stop it or wait for it", spec.Name, r.ID)
		}
	}
	run := &workflowRun{
		ID: w.newID(spec.Name), Workflow: spec.Name, Dir: dir, Session: session, Workspace: runWorkspace(workspace, session), Actor: actor,
		WF: spec, Partition: workflowPartition(partition), timeout: timeout, started: time.Now(),
		base: gitHead(dir),
	}
	w.runs[run.ID] = run
	w.mu.Unlock()
	s.triggerWorkflow(run)
	run.note(s, fmt.Sprintf("▶ %s · %d tasks · %s", spec.Name, len(spec.Tasks), run.ID))
	return run, nil
}

// triggerWorkflow runs the DAG once: every task whose target is missing and
// reachable runs; the run ends done, failed, or WAITING when it reached an
// external target nobody has made yet — then approving (or any other way
// the target appears) triggers it again, and what exists is skipped.
func (s *Server) triggerWorkflow(run *workflowRun) {
	ctx, cancel := context.WithCancel(context.Background())
	var exe *workflow.Execution
	repo, err := s.runRepository(run.Dir)
	if err == nil {
		exe, err = workflow.Start(ctx, run.WF, s.workflowTypes(&workflowRunner{s: s, run: run}), workflow.Options{
			Workdir: run.Dir, Partition: run.Partition, Events: workflowEvents{s: s, run: run}, Timeout: run.timeout,
			Logger:     logs.New("mario"), // mario's lines land in `memdoor logs query`
			Repository: repo,
		})
	}
	run.mu.Lock()
	run.cancel = cancel
	if err != nil {
		run.state, run.finalErr, run.ended = "failed", err.Error(), time.Now()
		run.mu.Unlock()
		cancel()
		run.note(s, fmt.Sprintf("■ %s failed — %s", run.Workflow, err))
		return
	}
	run.exe, run.state, run.ended, run.finalErr = exe, "running", time.Time{}, ""
	run.mu.Unlock()
	stopListening := s.listen(run)
	go func() {
		defer cancel()
		res := exe.Wait()
		stopListening()
		run.mu.Lock()
		run.exe, run.last, run.ended = nil, res, time.Now()
		blocked := res.Blocked()
		switch {
		case ctx.Err() != nil && run.state == "stopping":
			run.state = "stopped"
		case res.Failed():
			run.state = "failed"
		case len(blocked) > 0:
			run.state = "waiting"
		default:
			run.state = "done"
		}
		state := run.state
		run.mu.Unlock()
		// note carries the run's whole state, so the block reads the end
		// (done, failed, waiting) with the same message as the line.
		if state == "waiting" {
			run.note(s, fmt.Sprintf("⏸ %s waiting for %s — **a** in /workflow, or `memdoor workflow approve %s %s`",
				run.Workflow, strings.Join(blocked, ", "), run.ID, blocked[0]))
		} else if state == "done" {
			run.note(s, doneNote(run))
		} else {
			run.note(s, fmt.Sprintf("■ %s %s", run.Workflow, state))
		}
		logs.New("Workflow").Info("Workflow run "+state, slog.String("run", run.ID), slog.String("workflow", run.Workflow))
	}()
}

// workflowStatus finds a run by id.
func (s *Server) workflowStatus(id string) (*workflowRun, error) {
	w := s.workflows()
	w.mu.Lock()
	run, found := w.runs[id]
	w.mu.Unlock()
	if !found {
		return nil, fmt.Errorf("no run %s", id)
	}
	return run, nil
}

// workflowList is every run this gateway knows, newest first, and the
// workflows the project at dir declares.
func (s *Server) workflowList(dir string) ([]workflowRunJSON, []string) {
	w := s.workflows()
	w.mu.Lock()
	runs := make([]workflowRunJSON, 0, len(w.runs))
	for _, run := range w.runs {
		// This project's runs only: a window in one project listed another
		// project's (a scratch selftest's) runs (dogfood 2026-10-04).
		if dir != "" && !sameProject(run.Dir, dir) {
			continue
		}
		runs = append(runs, run.snapshot())
	}
	w.mu.Unlock()
	sort.Slice(runs, func(i, j int) bool { return runs[i].Started.After(runs[j].Started) })
	return runs, workflowFiles(dir)
}

// resumeWorkflow continues an earlier run — one that failed and was fixed,
// one stopped — as a new run on the SAME partition: what it finished is
// skipped, what it did not runs. The person names the run; the partition
// is the engine's word.
func (s *Server) resumeWorkflow(id, session, dir, workspace, actor string) (*workflowRun, error) {
	prev, err := s.workflowStatus(id)
	if err != nil {
		// A RUN FROM BEFORE A RESTART (2026-10-04: "fix resume after a gateway
		// restart"): this gateway's memory is empty, mario's run table is not.
		// The id names the workflow and its start; the table gives that
		// run's partition, and a fresh trigger on it skips what is done.
		if dir == "" {
			return nil, err
		}
		name, partition, ok := s.runFromTable(dir, id)
		if !ok {
			if name != "" {
				if parts := s.partitionsOf(dir, name); len(parts) > 0 {
					return nil, fmt.Errorf("no run %s in this gateway, and its partition cannot be told from the run table for sure — rerun the one you mean: memdoor workflow run %s --partition <p> (resuming what it finished). Partitions of %s: %s", id, name, name, strings.Join(parts, ", "))
				}
			}
			return nil, fmt.Errorf("no run %s in this gateway or in %s's run table", id, dir)
		}
		return s.startWorkflow(dir, name, session, workspace, actor, partition, 0)
	}
	prev.mu.Lock()
	state, dir, name, partition, timeout := prev.state, prev.Dir, prev.WF.Dir, prev.Partition, prev.timeout
	prev.mu.Unlock()
	if state == "running" || state == "stopping" {
		return prev, fmt.Errorf("run %s is still %s — stop it, or wait", id, state)
	}
	if state == "waiting" {
		return prev, fmt.Errorf("run %s is waiting for an approval: approve it, it goes on by itself", id)
	}
	if session == "" {
		session = prev.Session
	}
	if workspace == "" {
		workspace = prev.Workspace
	}
	if actor == "" {
		actor = prev.Actor
	}
	return s.startWorkflow(dir, name, session, workspace, actor, partition, timeout)
}

func (s *Server) stopWorkflow(id string) (*workflowRun, error) {
	w := s.workflows()
	w.mu.Lock()
	run, ok := w.runs[id]
	w.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no run %s", id)
	}
	run.mu.Lock()
	switch run.state {
	case "running":
		run.state = "stopping"
		run.mu.Unlock()
		run.cancel()
	case "waiting":
		run.state, run.ended = "stopped", time.Now()
		run.mu.Unlock()
		run.note(s, "■ "+run.Workflow+" stopped while waiting")
	default:
		st := run.state
		run.mu.Unlock()
		return run, fmt.Errorf("run %s is already %s", id, st)
	}
	return run, nil
}

// approveWorkflowTask completes an external task by producing its file
// target — a person's yes, written where the DAG looks for it. A command
// target cannot be approved: it is proven by the command, not by a person.
func (s *Server) approveWorkflowTask(id, task string) (*workflowRun, error) {
	w := s.workflows()
	w.mu.Lock()
	run, ok := w.runs[id]
	w.mu.Unlock()
	if !ok {
		// After a gateway restart the run lives only in the project's table:
		// resume brings it back to this gate under a new id (2026-10-04).
		return nil, fmt.Errorf("no run %s in this gateway — if it restarted, `/workflow resume %s` (or `memdoor workflow resume %s`) brings the run back to its gate", id, id, id)
	}
	t, found := run.WF.Task(task)
	if !found {
		return run, fmt.Errorf("%s has no task %s", run.Workflow, task)
	}
	if !t.External {
		return run, fmt.Errorf("%s is not a gate: the run does it itself, nobody approves it", task)
	}
	p := workflow.OutputPath(run.Dir, run.Partition, t.Full)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return run, err
	}
	if err := os.WriteFile(p, []byte(fmt.Sprintf("approved %s\n", time.Now().UTC().Format(time.RFC3339))), 0o644); err != nil {
		return run, err
	}
	run.mu.Lock()
	if tr, ok := run.last.Tasks[t.Name]; ok {
		tr.Status = workflow.StatusSkipped // the target is there now; the next trigger reads it from mario
		run.last.Tasks[t.Name] = tr
	}
	waiting := run.state == "waiting"
	run.mu.Unlock()
	run.emit(s, t.Name, "done", "", false)
	if waiting {
		s.triggerWorkflow(run)
	}
	return run, nil
}

// REQUEST CHANGES AT A GATE (Greg, 2026-10-03: "Workflow need hil"). A
// reviewer who will not approve says why; the step they name runs again
// with its own prompt and the comment, then every agent step after it that
// leads to the gate, and the run comes back to the same gate. Nothing in
// mario changes: the gate's target is still absent, so the run is still
// waiting, and each rerun turn is held to its step's target as before. Found
// live: issue-to-pr's fix passed its own tests and parsed CSV line by line,
// and approve was the only answer the gate had.

const changesRequestedMark = "The reviewer asked for changes before approving"

// requestChanges reruns task and the agent steps after it, in order, with
// the reviewer's comment, while the run waits at its gate.
func (s *Server) requestChanges(id, task, comment string) (*workflowRun, error) {
	w := s.workflows()
	w.mu.Lock()
	run, ok := w.runs[id]
	w.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no run %s", id)
	}
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return run, fmt.Errorf("say what to change: the comment is what the step reruns with")
	}
	run.mu.Lock()
	state := run.state
	_, asked := run.asked[task]
	run.mu.Unlock()
	if state != "waiting" {
		return run, fmt.Errorf("%s is %s: changes are requested at a gate, while the run waits for approval", id, state)
	}
	if _, found := run.WF.Task(task); !found {
		return run, fmt.Errorf("%s has no task %s", run.Workflow, task)
	}
	if !asked {
		return run, fmt.Errorf("%s is not an agent step this run ran: name the step whose work should change", task)
	}
	chain := changeChain(run.WF, task)
	run.mu.Lock()
	run.state = "running"
	run.mu.Unlock()
	run.note(s, fmt.Sprintf("↺ changes requested on %s: %s — rerunning %s", task, oneLine(comment, 120), strings.Join(chain, " → ")))
	go func() {
		runner := &workflowRunner{s: s, run: run}
		for i, name := range chain {
			run.mu.Lock()
			tr, ok := run.asked[name]
			run.mu.Unlock()
			if !ok {
				continue // not an agent step this run ran (a command, an external)
			}
			if i == 0 {
				tr.Prompt += "\n\n" + changesRequestedMark + ":\n" + comment + "\nMake that change, keep the step's target true, and say what you changed."
			} else {
				tr.Prompt += "\n\n" + changesRequestedMark + " on " + task + " (\"" + oneLine(comment, 300) + "\"), and it was redone. Redo your part so it matches."
			}
			run.emit(s, name, "running", "", false)
			if _, err := runner.Run(context.Background(), tr); err != nil {
				run.emit(s, name, "failed", err.Error(), false)
				run.mu.Lock()
				run.state = "waiting"
				run.mu.Unlock()
				run.note(s, fmt.Sprintf("✗ %s failed after the change request: %s — still waiting for review", name, oneLine(err.Error(), 160)))
				return
			}
			run.emit(s, name, "done", "", false)
		}
		run.mu.Lock()
		run.state = "waiting"
		blocked := run.last.Blocked()
		run.mu.Unlock()
		gate := "the gate"
		if len(blocked) > 0 {
			gate = blocked[0]
		}
		run.note(s, fmt.Sprintf("⏸ back at %s after the changes — **a** in /workflow, or `memdoor workflow approve %s %s`", gate, run.ID, gate))
	}()
	return run, nil
}

// changeChain is task and every task after it that depends on it, in an
// order where each comes after what it requires; externals are left out.
func changeChain(wf workflow.Workflow, task string) []string {
	in := map[string]bool{task: true}
	for grew := true; grew; {
		grew = false
		for _, t := range wf.Tasks {
			if in[t.Name] || t.External {
				continue
			}
			for _, r := range t.Requires {
				if in[r] {
					in[t.Name], grew = true, true
					break
				}
			}
		}
	}
	var out []string
	done := map[string]bool{}
	for len(out) < len(in) {
		progressed := false
		for _, t := range wf.Tasks {
			if !in[t.Name] || done[t.Name] {
				continue
			}
			ready := true
			for _, r := range t.Requires {
				if in[r] && !done[r] {
					ready = false
				}
			}
			if ready {
				out = append(out, t.Name)
				done[t.Name], progressed = true, true
			}
		}
		if !progressed {
			break
		}
	}
	return out
}

// workflowFiles lists the workflows a project declares: every directory
// under .memdoor/workflows/ that holds task YAML.
func workflowFiles(dir string) []string { return workflow.Names(dir) }

// workflowEntries is what the list carries beside the names: where each
// workflow comes from and what its README says.
func workflowEntries(dir string) []map[string]string {
	var out []map[string]string
	for _, e := range workflow.Describe(dir) {
		out = append(out, map[string]string{"name": e.Name, "source": e.Source, "description": e.Description})
	}
	return out
}

// ---- HTTP ------------------------------------------------------------------

type workflowRequest struct {
	Dir       string `json:"dir"`
	Name      string `json:"name"`
	Workspace string `json:"workspace"`
	Channel   string `json:"channel"`
	RunID     string `json:"run_id"`
	Task      string `json:"task"`
	Timeout   string `json:"timeout"`
	Partition string `json:"partition"` // "" = fresh; an earlier run's partition resumes it; "today" = once a day
	Comment   string `json:"comment"`   // changes: what the reviewer wants changed
	Limit     int    `json:"limit"`     // history: how many runs back (0 = the default)
}

func (s *Server) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/workflow"), "/")
	var req workflowRequest
	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		q := r.URL.Query()
		req = workflowRequest{Dir: q.Get("dir"), RunID: q.Get("run_id")}
	}
	ok := func(v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	fail := func(code int, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}
	session := ""
	if req.Channel != "" {
		session = shared.NewChannelSessionID(strings.TrimSpace(req.Workspace), req.Channel)
	}
	switch action {
	case "run":
		timeout := 0 * time.Second
		if req.Timeout != "" {
			d, err := time.ParseDuration(req.Timeout)
			if err != nil {
				fail(http.StatusBadRequest, fmt.Errorf("timeout: %w", err))
				return
			}
			timeout = d
		}
		run, err := s.startWorkflow(req.Dir, req.Name, session, strings.TrimSpace(req.Workspace), string(authorization.GetActorID(r.Context())), req.Partition, timeout)
		if err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		ok(run.snapshot())
	case "resume":
		run, err := s.resumeWorkflow(req.RunID, session, req.Dir, strings.TrimSpace(req.Workspace), string(authorization.GetActorID(r.Context())))
		if err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		ok(run.snapshot())
	case "status":
		run, err := s.workflowStatus(req.RunID)
		if err != nil {
			fail(http.StatusNotFound, err)
			return
		}
		ok(run.snapshot())
	case "stop":
		run, err := s.stopWorkflow(req.RunID)
		if err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		ok(run.snapshot())
	case "changes":
		run, err := s.requestChanges(req.RunID, req.Task, req.Comment)
		if err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		ok(run.snapshot())
	case "diff":
		run, err := s.workflowStatus(req.RunID)
		if err != nil {
			fail(http.StatusNotFound, err)
			return
		}
		run.mu.Lock()
		dir, base, since := run.Dir, run.base, run.reviewSince()
		run.mu.Unlock()
		ok(map[string]string{"diff": diffOf(dir, base, since)})
	case "approve":
		run, err := s.approveWorkflowTask(req.RunID, req.Task)
		if err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		ok(run.snapshot())
	case "history":
		// What the workflow did, read from the run table rather than from
		// this gateway's memory: a run this process never saw — one from
		// before a restart — is here, which is the point of keeping it.
		name := req.Name
		if name == "" {
			fail(http.StatusBadRequest, fmt.Errorf("history needs the workflow's name (name=)"))
			return
		}
		runs, err := s.workflowHistory(req.Dir, name, req.Limit)
		if err != nil {
			fail(http.StatusBadGateway, err)
			return
		}
		ok(map[string]interface{}{"runs": runs})
	case "", "list":
		runs, files := s.workflowList(req.Dir)
		ok(map[string]interface{}{"runs": runs, "files": files, "entries": workflowEntries(req.Dir)})
	default:
		fail(http.StatusNotFound, fmt.Errorf("no such action %q", action))
	}
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

// THE RUN TABLE (Greg, 2026-10-03: "runs you can see later"). A run is
// readable after the gateway that ran it is gone: mario's sqlite repository,
// one per project, at .memdoor/runs.db beside the run outputs. A person who
// runs a workflow on Monday reads what it did on Friday.
//
// The handle is opened once per project and kept: opening it again for every
// trigger would be a file open and a schema check per task, and the run's
// state is read from it while the run is going.
//
// A PRO ACCOUNT'S RUNS LIVE ON MEMDOOR.AI (hosted_state.go): the hosted
// repository comes first, and an error from it is the run not starting.
func (s *Server) runRepository(dir string) (mario.WorkflowRepository, error) {
	if repo, err := s.hostedState.repository(); err != nil {
		return nil, err
	} else if repo != nil {
		return repo, nil
	}
	root := workflow.ProjectRoot(dir)
	if root == "" {
		return nil, nil // no project: the in-memory repository, as before
	}
	s.runReposMu.Lock()
	defer s.runReposMu.Unlock()
	if repo, ok := s.runRepos[root]; ok {
		return repo, nil
	}
	if s.runRepos == nil {
		s.runRepos = map[string]mario.WorkflowRepository{}
	}
	// .memdoor/ holds the workflows and the runs; the table goes beside
	// them, and EnsureRunsIgnored keeps it out of a commit.
	workflow.EnsureIgnored(dir)
	repo, err := sqlite.OpenWorkflowRepositoryAt(filepath.Join(root, shared.MemdoorDirName, "runs.db"), nil)
	if err != nil {
		// A run table that cannot be opened is not worth a dead gateway:
		// the run goes on with mario's in-memory repository, and the
		// reason is in the log for whoever wonders why history is empty.
		logs.New("Workflow").Error("could not open the run table — runs will not survive a restart",
			slog.String("dir", root), slog.String("error", err.Error()))
		return nil, nil
	}
	s.runRepos[root] = repo
	return repo, nil
}

// workflowHistoryJSON is one run as the history shows it.
type workflowHistoryJSON struct {
	Workflow  string `json:"workflow"`
	Partition string `json:"partition"`
	State     string `json:"state"`
	Error     string `json:"error,omitempty"`
	Started   string `json:"started,omitempty"`
	Ended     string `json:"ended,omitempty"`
	Tasks     int    `json:"tasks"`
}

// workflowHistory is what a workflow did, newest first, read from the
// project's run table (.memdoor/runs.db) instead of this gateway's memory:
// a run that finished before this process started is still here. That is
// the difference from workflowList, which lists only the runs this gateway
// has in hand.
func (s *Server) workflowHistory(dir, name string, limit int) ([]workflowHistoryJSON, error) {
	repo, err := s.runRepository(dir)
	if err != nil {
		// The hosted table did not answer: say so rather than show an
		// empty local one as the history (review finding, 2026-10-05).
		return nil, err
	}
	lister, ok := repo.(interface {
		Runs(string, int) []sqlite.RunSummary
	})
	if !ok {
		return nil, nil
	}
	summaries := lister.Runs(name, limit)
	out := make([]workflowHistoryJSON, 0, len(summaries))
	for _, run := range summaries {
		h := workflowHistoryJSON{
			Workflow:  run.Name,
			Partition: run.Partition,
			State:     runStatusWord(run.Status),
			Error:     run.Error,
			Tasks:     run.Executions,
		}
		if !run.Started.IsZero() {
			h.Started = run.Started.Format(time.RFC3339)
		}
		if !run.Ended.IsZero() {
			h.Ended = run.Ended.Format(time.RFC3339)
		}
		out = append(out, h)
	}
	return out, nil
}

// runStatusWord is mario's status in the words the window already uses
// ("done", "failed", "running") rather than mario's own ("Done", "Failed").
func runStatusWord(status mario.Status) string {
	switch status {
	case mario.Done:
		return "done"
	case mario.Failed:
		return "failed"
	case mario.Started:
		return "running"
	default:
		return "waiting"
	}
}

// receiptOf is the receipt line a codebase turn ends on (turn_done.go
// line), or "".
func receiptOf(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-3; i-- {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "✓ ") || strings.HasPrefix(l, "⚠ ") {
			return l
		}
	}
	return ""
}

// renderPartition fills a target's partition placeholder the way mario's
// template does ("{{.partition}}", "{{ .partition }}").
func renderPartition(target, partition string) string {
	for _, ph := range []string{"{{.partition}}", "{{ .partition }}"} {
		target = strings.ReplaceAll(target, ph, partition)
	}
	return target
}

// gitHead is the commit HEAD names in dir, or "".
func gitHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// reviewOf is what changed in dir since base: the commits made on top of it
// and the diff stat of the working tree against it, for a person at a gate.
func reviewOf(dir, base string, since time.Time) string {
	if base == "" {
		return ""
	}
	var b strings.Builder
	if out, err := exec.Command("git", "-C", dir, "log", "--oneline", "--no-decorate", base+"..HEAD").Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		b.WriteString("commits since the run started:\n" + strings.TrimRight(string(out), "\n") + "\n")
	}
	if out, err := exec.Command("git", "-C", dir, "diff", "--stat", base).Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		b.WriteString("changes:\n" + strings.TrimRight(string(out), "\n") + "\n")
	}
	if made := newUntracked(dir, since); len(made) > 0 {
		b.WriteString("new files:\n " + strings.Join(made, "\n "))
	}
	return strings.TrimSpace(b.String())
}

// newUntracked are the files the run made that git does not track yet: not
// ignored, written since the run started. Most of what a workflow produces is
// exactly this — reports, plans, scripts — and a diff of tracked files alone
// showed "no changes" at a gate where a dozen files were new (dogfood,
// 2026-10-04).
func newUntracked(dir string, since time.Time) []string {
	out, err := exec.Command("git", "-C", dir, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil
	}
	var made []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		if st, err := os.Stat(filepath.Join(dir, f)); err == nil && !st.ModTime().Before(since.Add(-time.Second)) {
			made = append(made, f)
		}
	}
	return made
}

// maxGateDiff caps the diff a gate hands over: past it the person reads the
// rest in git, and the cut says so.
const maxGateDiff = 256 << 10

// maxGateNewFile is the largest new file the gate's diff reads in full.
const maxGateNewFile = 1 << 20

// diffOf is the whole change since the run started, committed or not: what
// the summary in reviewOf counts, line by line.
func diffOf(dir, base string, since time.Time) string {
	if base == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", dir, "diff", "--no-color", base).Output()
	if err != nil {
		return ""
	}
	// The new files too, each as a diff from nothing (exit 1 is git's
	// "they differ", not a failure).
	for _, f := range newUntracked(dir, since) {
		if len(out) > maxGateDiff {
			break // the cap below cuts it anyway: no more files read
		}
		// A big new file (a dataset, a dump) is named, not read: diffing it
		// would hold the whole file in memory before the cap (review,
		// 2026-10-04).
		if st, err := os.Stat(filepath.Join(dir, f)); err == nil && st.Size() > maxGateNewFile {
			out = append(out, fmt.Sprintf("new file %s: %d KB, too big to show here\n", f, st.Size()>>10)...)
			continue
		}
		nf, _ := exec.Command("git", "-C", dir, "diff", "--no-color", "--no-index", "/dev/null", f).Output()
		out = append(out, nf...)
	}
	if len(out) > maxGateDiff {
		cut := out[:maxGateDiff]
		if i := strings.LastIndexByte(string(cut), '\n'); i > 0 {
			cut = cut[:i+1]
		}
		return string(cut) + fmt.Sprintf("… cut at %d KB of %d KB: git -C %s diff %s for the rest\n", maxGateDiff>>10, len(out)>>10, dir, base)
	}
	return string(out)
}

// runIDShape is a run id: the workflow's name, its start (local time, to the
// second) and, for a second run in the same second, a counter.
var runIDShape = regexp.MustCompile(`^(.+)-(\d{8}-\d{6})(?:-\d+)?$`)

// runFromTable finds a run this gateway no longer holds in the project's run
// table, and never guesses. A fresh run's partition IS its start stamp, so the
// id names it exactly. A named partition (a slug, "today", a commit) is taken
// only when the id's stamp lies inside that partition's span in the table —
// first start to last movement. Otherwise there is no answer: resume refuses
// and names the partitions (2026-10-04: a "latest started before the stamp"
// fallback resumed building-effective-agents for a jeff run and reran it).
func (s *Server) runFromTable(dir, id string) (name, partition string, ok bool) {
	m := runIDShape.FindStringSubmatch(id)
	if m == nil {
		return "", "", false
	}
	stamp, err := time.ParseInLocation("20060102-150405", m[2], time.Local)
	if err != nil {
		return "", "", false
	}
	history, _ := s.workflowHistory(dir, m[1], 200)
	own := stamp.Format("2006-01-02T150405")
	for _, h := range history {
		if h.Partition == own {
			return m[1], own, true
		}
	}
	const slack = 3 * time.Second
	var inside []string
	for _, h := range history {
		started, err := time.Parse(time.RFC3339, h.Started)
		if err != nil || started.After(stamp.Add(slack)) {
			continue
		}
		ended := time.Now()
		if e, err := time.Parse(time.RFC3339, h.Ended); err == nil && h.State != "running" {
			ended = e
		}
		if !stamp.After(ended.Add(slack)) {
			inside = append(inside, h.Partition)
		}
	}
	if len(inside) == 1 {
		return m[1], inside[0], true
	}
	return m[1], "", false
}

// partitionsOf names a workflow's partitions in the run table, newest first,
// for a refusal that says what can be rerun instead.
func (s *Server) partitionsOf(dir, name string) []string {
	var out []string
	history, _ := s.workflowHistory(dir, name, 10)
	for _, h := range history {
		out = append(out, h.Partition+" ("+h.State+")")
	}
	return out
}

// resultFiles are the files a run's tasks name as their targets and that
// exist, in the workflow's task order: what the run produced.
func resultFiles(dir, partition string, wf workflow.Workflow) []string {
	var out []string
	for _, t := range wf.Tasks {
		if !strings.HasPrefix(t.Target, "file ") {
			continue
		}
		f := renderPartition(strings.TrimSpace(strings.TrimPrefix(t.Target, "file ")), partition)
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			out = append(out, f)
		}
	}
	return out
}

// resultPreviewLines is how much of the final file a done run shows.
const resultPreviewLines = 20

// doneNote is the line a finished run ends on: the files it made, and the
// opening of the last one a final task made — the answer, not just "done".
func doneNote(run *workflowRun) string {
	files := resultFiles(run.Dir, run.Partition, run.WF)
	var b strings.Builder
	b.WriteString("■ " + run.Workflow + " done")
	if len(files) == 0 {
		return b.String()
	}
	b.WriteString(" — results: " + strings.Join(files, " · "))
	final := ""
	for _, sink := range run.WF.Sinks() {
		if t, ok := run.WF.Task(sink); ok && strings.HasPrefix(t.Target, "file ") {
			f := renderPartition(strings.TrimSpace(strings.TrimPrefix(t.Target, "file ")), run.Partition)
			if _, err := os.Stat(filepath.Join(run.Dir, f)); err == nil {
				final = f
			}
		}
	}
	if final == "" {
		return b.String()
	}
	data, err := os.ReadFile(filepath.Join(run.Dir, final))
	if err != nil {
		return b.String()
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	more := ""
	if len(lines) > resultPreviewLines {
		more = fmt.Sprintf("\n… %d more lines in %s", len(lines)-resultPreviewLines, final)
		lines = lines[:resultPreviewLines]
	}
	// Indented, not fenced: the window prints a note's fence marks literally.
	b.WriteString("\n\n" + final + ":\n    " + strings.Join(lines, "\n    ") + more)
	return b.String()
}

// reviewSince is when the work a gate reviews began: a fresh run's partition
// is its first start (a resume after a restart is a new run with a later
// start, and files made before it are still this run's work); a named
// partition falls back to this run's own start. Caller holds r.mu.
func (r *workflowRun) reviewSince() time.Time {
	if t, err := time.ParseInLocation("2006-01-02T150405", r.Partition, time.Local); err == nil && t.Before(r.started) {
		return t
	}
	return r.started
}

// pinTaskModel picks the model a workflow step's turn answers on: the task's
// own `model:` first, else the person's kept pin (/model, ~/.memdoor/
// model.json), else the agent's ladder. The kept pin used to reach a step only
// through the window that started the run, so a run resumed after a restart
// or started from the shell answered on the ladder's first rung while the
// footer said "pinned by you" (dogfood 2026-10-04: karpathy on GLM, pin
// DeepSeek).
func pinTaskModel(sess *Session, model, actor string) error {
	if model != "" {
		return pinModel(sess, model, "", "")
	}
	applyPersistentPin(actor, sess)
	return nil
}

// sameProject compares two project directories after resolving symlinks
// (/tmp is /private/tmp on macOS).
func sameProject(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}
