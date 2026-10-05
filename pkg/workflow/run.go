package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/guregodevo/mario/engine"
	"github.com/guregodevo/mario/factory"
	mariolog "github.com/guregodevo/mario/logger"
	"github.com/guregodevo/mario/scheduler"
	"github.com/guregodevo/mario/static"
	"github.com/guregodevo/mario/tasks"
	mario "github.com/guregodevo/mario/workflow"
)

// Events is what a host watches: every transition a task makes, by workflow
// and task name, in the order it happened. mario sends them on a channel;
// this package reads it and calls the host's listener, so the host declares
// nothing of mario's. The TUI draws the DAG from these.
type Events interface {
	TaskStarted(workflow, task string)
	TaskDone(workflow, task string)
	TaskSkipped(workflow, task string) // its target was already there
	TaskFailed(workflow, task string, err error, willRetry bool)
}

// Logger is where mario's engine logs for this run — the four methods every
// Go structured logger has (slog's), declared here so the host hands in its
// own without this package knowing it.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// Status is a task's state, read from mario's record of its execution.
type Status string

const (
	StatusWaiting  Status = "waiting"  // not started: a requirement is not done, an external target is missing, or the run ended first
	StatusRunning  Status = "running"  // its run is going
	StatusRetrying Status = "retrying" // failed, and mario will try it again
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusSkipped  Status = "skipped" // its target already existed; nothing ran
)

// Result is the run's state: at the end, or at any moment through Execution.Status.
type Result struct {
	Workflow  string
	Partition string
	Tasks     map[string]TaskResult // by short name
}

// TaskResult is one task's record — mario's execution, read for a person.
type TaskResult struct {
	Status   Status
	Error    string
	Started  time.Time
	Ended    time.Time
	Took     time.Duration
	Attempt  int // 1 on the first try
	External bool
}

// Failed answers whether any task failed.
func (r Result) Failed() bool {
	for _, t := range r.Tasks {
		if t.Status == StatusFailed {
			return true
		}
	}
	return false
}

// Blocked names the external tasks whose target is still missing: the run
// stopped at them, and goes on when it is triggered again after someone
// made the target. In name order.
func (r Result) Blocked() []string {
	var names []string
	for n, t := range r.Tasks {
		if t.External && t.Status == StatusWaiting {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// Done counts the tasks that are done or skipped.
func (r Result) Done() int {
	n := 0
	for _, t := range r.Tasks {
		if t.Status == StatusDone || t.Status == StatusSkipped {
			n++
		}
	}
	return n
}

// Summary is one line per task, in name order.
func (r Result) Summary() string {
	names := make([]string, 0, len(r.Tasks))
	for n := range r.Tasks {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		t := r.Tasks[n]
		fmt.Fprintf(&b, "%-8s %s", t.Status, n)
		if t.Took > 0 {
			fmt.Fprintf(&b, "  (%s)", t.Took.Round(time.Millisecond))
		}
		if t.Error != "" {
			fmt.Fprintf(&b, "  — %s", t.Error)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// Options bound a run and name where it happens.
type Options struct {
	Logger    Logger        // mario's log for this run (optional; mario's default otherwise)
	Workdir   string        // the project: every target, output and turn is relative to it
	Partition string        // mario's partition: the run's own by default (its start time); an earlier one resumes it
	Events    Events        // optional
	Retry     time.Duration // how long a failed task waits before its retry (default 2s)
	// Timeout bounds the WHOLE run: past it nothing more starts and the
	// running task is told. Default 2h, never more than 24h.
	Timeout time.Duration
	// Repository is where this run's state is kept. Left nil, every run
	// builds the in-memory one: the run is real while it lasts and
	// nothing is readable afterwards. A host that wants a run table
	// passes a persistent repository — mario's sqlite one, opened at a
	// path of its choosing (mario/sqlite.NewWorkflowRepositoryAt) — and
	// the state outlives the process that ran it.
	Repository mario.WorkflowRepository
}

const (
	defaultRunTimeout = 2 * time.Hour
	maxRunTimeout     = 24 * time.Hour
	component         = "memdoor"
)

// Registry builds the component a DAG is validated and run with: the task
// types the host offers, unbound. The YAML names a type; the factory behind
// it builds the task.
func Registry(types ...tasks.Base) factory.Component {
	reg := factory.NewComponent()
	for i := range types {
		reg.Add(&types[i])
	}
	return reg
}

// Execution is a run in progress on mario: its state is read from mario's
// repository at any moment, and Wait returns when the scheduler is done and
// every event has reached the host.
type Execution struct {
	wf        Workflow
	workdir   string
	partition string
	outputs   tasks.Outputs
	repo      mario.WorkflowRepository
	cancel    context.CancelFunc
	done      chan struct{}
}

// Run executes a workflow on mario and waits for it: Start, then Wait.
func Run(ctx context.Context, wf Workflow, types []tasks.Base, opts Options) (Result, error) {
	exe, err := Start(ctx, wf, types, opts)
	if err != nil {
		return Result{}, err
	}
	return exe.Wait(), nil
}

// Start executes a workflow on mario: the host's task types are bound to
// the run, BuildDAG wires the definitions and their requirements into a
// repository, the local scheduler is triggered on the sinks and backfills
// what they need, every task is built by the factory of its type, and the
// run ends when every reachable task is done, failed past its retries, or
// waiting on an external target nobody has made yet. Events arrive on
// mario's channel and reach opts.Events in order.
func Start(ctx context.Context, wf Workflow, types []tasks.Base, opts Options) (*Execution, error) {
	if len(types) == 0 {
		return nil, fmt.Errorf("a workflow needs at least one task type")
	}
	if opts.Workdir == "" {
		return nil, fmt.Errorf("a workflow needs a working directory")
	}
	// A run happens at the PROJECT's root, wherever the TUI was opened: every
	// task — a command, a target, an agent's turn — works there, and the
	// outputs are kept there (Greg, 2026-10-02: "make the agent task run at
	// the project root too").
	opts.Workdir = ProjectRoot(opts.Workdir)
	if opts.Partition == "" {
		opts.Partition = time.Now().Format("2006-01-02T150405") // a fresh run; local time, the person's
	}
	if opts.Retry <= 0 {
		opts.Retry = 2 * time.Second
	}
	if opts.Timeout <= 0 || opts.Timeout > maxRunTimeout {
		opts.Timeout = defaultRunTimeout
	}
	if opts.Logger != nil {
		// mario's logger is engine-wide; the host's is handed in before the
		// DAG is built so building and running both log through it.
		mariolog.Log = opts.Logger
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	version := wf.Name + "@" + opts.Partition
	outputs := tasks.Outputs{Dir: OutputsDir(opts.Workdir)}
	EnsureIgnored(opts.Workdir)

	reg := factory.NewComponent()
	for _, t := range types {
		b := t.Bind(wf.Defs, opts.Workdir, outputs, opts.Partition, version)
		b.Externals = wf.siblings // a requirement on another workflow is checked by that task's target
		reg.Add(b)
	}
	// The host's repository, or the in-memory one: a run that must be
	// readable after this process is gone is what sqlite is for, and a
	// host that is happy with a run's state living only while it runs
	// passes nothing.
	repo := opts.Repository
	if repo == nil {
		repo = static.NewWorkflowRepository()
	}
	ids, err := factory.BuildDAG(version, opts.Partition, component, repo, wf.Defs, reg)
	if err != nil {
		cancel()
		return nil, err
	}

	// backfill=true: triggering the sinks pulls the whole graph; loop=true
	// runs the queues until WaitForCompletion closes them.
	n := len(wf.Tasks)
	s := scheduler.NewLocalScheduler(true, true, static.NewChannelQueue(n*4+16), static.NewChannelQueue(n*4+16),
		repo, tasks.Mixed(reg, wf.Defs), engine.NewLocalExecutor(), opts.Retry)
	s.SetContext(ctx)
	forwarded := make(chan struct{})
	if opts.Events != nil {
		ch := make(chan scheduler.Event, n*8+16)
		s.SetEvents(ch)
		go forward(ch, wf, opts.Events, forwarded)
	} else {
		close(forwarded)
	}
	for _, sink := range wf.Sinks() {
		if err := s.Trigger(ids[sink]); err != nil {
			cancel()
			return nil, fmt.Errorf("could not start %s: %w", sink, err)
		}
	}
	exe := &Execution{wf: wf, workdir: opts.Workdir, partition: opts.Partition, outputs: outputs, repo: repo, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer cancel()
		_ = s.WaitForCompletion() // closes the event channel
		<-forwarded               // every event has reached the host
		close(exe.done)
	}()
	return exe, nil
}

// forward reads mario's events in order and calls the host's listener.
func forward(ch <-chan scheduler.Event, wf Workflow, to Events, done chan<- struct{}) {
	defer close(done)
	short := func(full string) string {
		if t, ok := wf.Task(full); ok {
			return t.Name
		}
		return full
	}
	for ev := range ch {
		switch ev.Kind {
		case scheduler.Started:
			to.TaskStarted(wf.Name, short(ev.Name))
		case scheduler.Done:
			to.TaskDone(wf.Name, short(ev.Name))
		case scheduler.Skipped:
			to.TaskSkipped(wf.Name, short(ev.Name))
		case scheduler.Failed:
			to.TaskFailed(wf.Name, short(ev.Name), ev.Err, ev.WillRetry)
		}
	}
}

// Wait blocks until the scheduler is done and every event delivered, and
// answers the final state.
func (e *Execution) Wait() Result {
	<-e.done
	return e.Status()
}

// Done tells without blocking.
func (e *Execution) Done() bool {
	select {
	case <-e.done:
		return true
	default:
		return false
	}
}

// Status reads every task's record from mario's repository, now.
func (e *Execution) Status() Result {
	res := Result{Workflow: e.wf.Name, Partition: e.partition, Tasks: map[string]TaskResult{}}
	for _, t := range e.wf.Tasks {
		tr := TaskResult{Status: StatusWaiting, External: t.External}
		if exe, ok := e.repo.Fetch(mario.InstanceIdOf(t.Full, e.partition)); ok {
			tr.Started, tr.Ended, tr.Error = exe.StartDate, exe.EndDate, exe.Error
			if !exe.StartDate.IsZero() {
				tr.Attempt = int(exe.DRetries) + 1
			}
			switch exe.Status {
			case mario.Started:
				tr.Status = StatusRunning
				tr.Took = time.Since(exe.StartDate)
			case mario.Done:
				tr.Status = StatusDone
				if exe.StartDate.IsZero() {
					tr.Status = StatusSkipped // mario found its target and never ran it
				} else {
					tr.Took = exe.EndDate.Sub(exe.StartDate)
				}
			case mario.Failed:
				tr.Status = StatusFailed
				if exe.DRetries < exe.DMaxRetries && !e.Done() {
					tr.Status = StatusRetrying
				}
				if !exe.EndDate.IsZero() {
					tr.Took = exe.EndDate.Sub(exe.StartDate)
				}
			}
		}
		// A task mario never visited — nothing downstream needed it this
		// time, or it is external — is skipped when its proof exists.
		if tr.Status == StatusWaiting && e.Done() && e.proof(t).Exists() {
			tr.Status = StatusSkipped
		}
		res.Tasks[t.Name] = tr
	}
	return res
}

// proof is a task's endpoint as its type's factory builds it: the target it
// names, or its output.
func (e *Execution) proof(t Task) mario.DataEndpoint {
	if d := e.wf.definition(t.Full); d != nil && tasks.HasTarget(d) {
		if d.Target.File != "" {
			return tasks.FileEndpoint(t.Full, e.workdir, d.Target.File)
		}
		return tasks.CommandEndpoint(t.Full, e.workdir, d.Target.Command)
	}
	return e.outputs.Endpoint(t.Full, e.partition)
}
