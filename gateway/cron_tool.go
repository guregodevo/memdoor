package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memdoor/gateway/config"
	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// THE AGENT SCHEDULES ITS OWN WAKE-UP (Greg, 2026-10-01: "we should have cron
// tool so that an agent can repeat a check for example every x seconds", "like
// the polling cron"). Waiting inside a turn burns the turn: a sleep holds the
// model, the window, and the person. Scheduling a check hands the time back —
// the turn ends, and the gateway starts a fresh one when the moment comes.
//
// The bounds are not optional. Every account of a runaway agent loop ends with
// the same fix, a counter rather than a better prompt, so a poll says how many
// times at the moment it is created, and the scheduler stops it there.
const (
	cronToolName = "cron"

	cronMinInterval = 10 * time.Second // below this a poll is a busy loop
	cronMaxInterval = 24 * time.Hour
	cronDefaultRuns = 10
	cronMaxRuns     = 60
	cronMaxLife     = 12 * time.Hour // nothing an agent starts outlives the day
)

// cronScheduler is what the tool needs of the scheduler, declared where it is
// used so a test can stand in for it.
type cronScheduler interface {
	AddJob(job *config.CronJob) error
	RemoveJob(jobID string) error
	Jobs() []*config.CronJob
}

type cronToolInput struct {
	Action string `json:"action"`
	Every  string `json:"every"`
	Task   string `json:"task"`
	Times  int    `json:"times"`
	ID     string `json:"id"`
}

// turnCronTool is the cron tool for THIS turn, or nil when the gateway runs no
// scheduler or the agent's palette does not carry it.
func (ar *AgentRuntime) turnCronTool(ctx context.Context) *tools.ToolDefinition {
	if ar.scheduler == nil || !ar.paletteHas(ctx, cronToolName) {
		return nil
	}
	workdir, _ := ctx.Value(sharedctx.WorkdirKey).(string)
	session, _ := ctx.Value(sharedctx.SessionIDKey).(string)
	// The agent is the TURN's, from the context: one runtime serves every
	// agent, and its agentConfig is the default profile ("main", no shell).
	// A job scheduled under that name ran with no bash and answered "I can't
	// run shell commands" four times (live 2026-10-01).
	agentID, _ := ctx.Value(sharedctx.AgentIDKey).(string)
	if agentID == "" {
		// A TUI chat turn names its agent under this key (server_jobs.go, the
		// buddy-chat branch), not under AgentIDKey.
		agentID, _ = ctx.Value("buddy_agent_name").(string)
	}
	if agentID == "" && ar.agentConfig != nil {
		agentID = ar.agentConfig.ID
	}
	t := ar.cronTool(session, workdir, agentID)
	return &t
}

// paletteHas answers whether this turn's palette carries a tool name. The
// coder's palette rides on the context (BuddyToolsKey) — the same source the
// request filter reads — so that is checked first; a runtime built with a
// config palette is checked after.
func (ar *AgentRuntime) paletteHas(ctx context.Context, name string) bool {
	if names, _ := ctx.Value(sharedctx.BuddyToolsKey).([]string); len(names) > 0 {
		for _, t := range names {
			if t == name {
				return true
			}
		}
		return false
	}
	if ar.agentConfig == nil || ar.agentConfig.Tools == nil {
		return false
	}
	for _, t := range ar.agentConfig.Tools.Allow {
		if t == name {
			return true
		}
	}
	return false
}

// cronTool is the agent's view of the scheduler, scoped to the session and the
// directory the turn runs in.
func (ar *AgentRuntime) cronTool(sessionKey, workdir, agentID string) tools.ToolDefinition {
	return tools.ToolDefinition{
		Name: cronToolName,
		Description: "Check something again later instead of waiting now. `every` repeats a task on an interval " +
			"(10s–24h) — CI, a deploy, a long build, a file appearing — and answers here each time, so the turn can " +
			"end. `times` bounds it (default 10, max 60). `list` what is scheduled, `stop` cancels one by id — " +
			"the run that finds what it waited for should stop its own job.\nUse it instead of sleeping in bash.",
		InputSchema: llm.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"action": map[string]interface{}{"type": "string", "enum": []string{"every", "list", "stop"}},
				"every":  map[string]interface{}{"type": "string", "description": "interval: 30s, 5m, 1h"},
				"task":   map[string]interface{}{"type": "string", "description": "what to run each time, standing alone"},
				"times":  map[string]interface{}{"type": "integer", "description": "runs before it stops (default 10, max 60)"},
				"id":     map[string]interface{}{"type": "string", "description": "which job to stop"},
			},
			Required: []string{"action"},
		},
		Function: func(input json.RawMessage) (string, error) {
			var in cronToolInput
			if err := json.Unmarshal(input, &in); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			switch strings.ToLower(strings.TrimSpace(in.Action)) {
			case "every", "create", "add":
				return ar.cronSchedule(in, sessionKey, workdir, agentID)
			case "list":
				return ar.cronList()
			case "stop", "cancel", "remove":
				return ar.cronStop(in.ID)
			default:
				return "", fmt.Errorf("action must be every, list or stop")
			}
		},
	}
}

func (ar *AgentRuntime) cronSchedule(in cronToolInput, sessionKey, workdir, agentID string) (string, error) {
	if ar.scheduler == nil {
		return "", fmt.Errorf("scheduling is not available on this gateway")
	}
	task := strings.TrimSpace(in.Task)
	if task == "" {
		return "", fmt.Errorf("task is required: what should run each time?")
	}
	every, err := time.ParseDuration(strings.TrimSpace(in.Every))
	if err != nil {
		return "", fmt.Errorf("every must be a duration like 30s, 5m or 1h: %w", err)
	}
	if every < cronMinInterval {
		return "", fmt.Errorf("%s is too often — %s is the shortest interval", every, cronMinInterval)
	}
	if every > cronMaxInterval {
		return "", fmt.Errorf("%s is too long — %s is the longest interval", every, cronMaxInterval)
	}
	times := in.Times
	if times <= 0 {
		times = cronDefaultRuns
	}
	if times > cronMaxRuns {
		times = cronMaxRuns
	}

	job := &config.CronJob{
		ID:       fmt.Sprintf("poll-%d", time.Now().UnixNano()%1e7),
		Schedule: "@every " + every.String(),
		AgentID:  agentID,
		Message: task + "\n\nThis is a scheduled check. If nothing needs attention yet, answer exactly HEARTBEAT_OK " +
			"and nothing else — it is shown, and wakes nobody. Any other answer wakes the conversation that scheduled you, " +
			"as a turn of its own, to act on it.",
		Enabled:    true,
		Workdir:    workdir,
		SessionKey: sessionKey,
		MaxRuns:    times,
		ExpiresAt:  time.Now().Add(cronMaxLife),
	}
	if err := ar.scheduler.AddJob(job); err != nil {
		return "", fmt.Errorf("could not schedule it: %w", err)
	}
	return fmt.Sprintf("Scheduled %s: every %s, up to %d times, first run in %s. "+
		"Each run answers here; a run with nothing to report says HEARTBEAT_OK and you sleep on, any other answer "+
		"wakes you with it. End your turn now — do not wait for it. The run that finds what it is waiting for "+
		"should stop the job: cron(action:\"stop\", id:\"%s\").",
		job.ID, every, times, every, job.ID), nil
}

func (ar *AgentRuntime) cronList() (string, error) {
	if ar.scheduler == nil {
		return "", fmt.Errorf("scheduling is not available on this gateway")
	}
	jobs := ar.scheduler.Jobs()
	if len(jobs) == 0 {
		return "Nothing is scheduled.", nil
	}
	var b strings.Builder
	for _, j := range jobs {
		fmt.Fprintf(&b, "%s  %s  %s", j.ID, j.Schedule, truncateForLog(j.Message, 60))
		if j.MaxRuns > 0 {
			fmt.Fprintf(&b, "  (run %d of %d)", j.RunCount, j.MaxRuns)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (ar *AgentRuntime) cronStop(id string) (string, error) {
	if ar.scheduler == nil {
		return "", fmt.Errorf("scheduling is not available on this gateway")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("id is required: which job? (cron(action:\"list\") shows them)")
	}
	if err := ar.scheduler.RemoveJob(id); err != nil {
		return "", fmt.Errorf("could not stop %s: %w", id, err)
	}
	return "Stopped " + id + ".", nil
}
