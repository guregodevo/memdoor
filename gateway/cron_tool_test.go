package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/config"
	sharedctx "memdoor/pkg/shared/context"
)

type fakeScheduler struct {
	added   []*config.CronJob
	removed []string
}

func (f *fakeScheduler) AddJob(j *config.CronJob) error { f.added = append(f.added, j); return nil }
func (f *fakeScheduler) RemoveJob(id string) error      { f.removed = append(f.removed, id); return nil }
func (f *fakeScheduler) Jobs() []*config.CronJob        { return f.added }

func cronCall(t *testing.T, ar *AgentRuntime, in string) (string, error) {
	t.Helper()
	tool := ar.cronTool("workspace:w:channel:c", "/tmp/proj", "coder")
	return tool.Function(json.RawMessage(in))
}

// Every poll an agent starts is bounded: a count, an expiry, an interval that
// is not a busy loop, and it runs where the turn ran and answers where it asked.
func TestTheCronToolBoundsWhatItSchedules(t *testing.T) {
	f := &fakeScheduler{}
	ar := &AgentRuntime{scheduler: f}

	out, err := cronCall(t, ar, `{"action":"every","every":"30s","task":"is CI green?","times":3}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.added) != 1 {
		t.Fatalf("want one job, got %d", len(f.added))
	}
	j := f.added[0]
	if j.Schedule != "@every 30s" || j.MaxRuns != 3 || j.Workdir != "/tmp/proj" || j.SessionKey != "workspace:w:channel:c" || j.AgentID != "coder" {
		t.Fatalf("job = %+v", j)
	}
	if j.ExpiresAt.IsZero() || j.ExpiresAt.After(time.Now().Add(cronMaxLife+time.Minute)) {
		t.Fatalf("every job expires within %s, got %v", cronMaxLife, j.ExpiresAt)
	}
	if !strings.Contains(out, j.ID) || !strings.Contains(out, "End your turn now") {
		t.Fatalf("the answer names the job and tells the agent to stop waiting: %q", out)
	}

	for _, bad := range []string{
		`{"action":"every","every":"1s","task":"x"}`,  // a busy loop
		`{"action":"every","every":"48h","task":"x"}`, // longer than a job may live
		`{"action":"every","every":"30s"}`,            // nothing to run
		`{"action":"every","every":"soon","task":"x"}`,
		`{"action":"stop"}`,
		`{"action":"dance"}`,
	} {
		if _, err := cronCall(t, ar, bad); err == nil {
			t.Errorf("%s must be refused", bad)
		}
	}
	if len(f.added) != 1 {
		t.Fatalf("a refused call must schedule nothing, got %d jobs", len(f.added))
	}

	if _, err := cronCall(t, ar, `{"action":"every","every":"1m","task":"x","times":500}`); err != nil {
		t.Fatal(err)
	}
	if got := f.added[1].MaxRuns; got != cronMaxRuns {
		t.Fatalf("times is capped at %d, got %d", cronMaxRuns, got)
	}

	if _, err := cronCall(t, ar, `{"action":"stop","id":"`+j.ID+`"}`); err != nil || len(f.removed) != 1 || f.removed[0] != j.ID {
		t.Fatalf("stop: err=%v removed=%v", err, f.removed)
	}
	list, _ := cronCall(t, ar, `{"action":"list"}`)
	if !strings.Contains(list, "@every 30s") || !strings.Contains(list, "(run 0 of 3)") {
		t.Fatalf("list shows the schedule and the bound: %q", list)
	}
}

func TestTheCronToolSaysWhenThereIsNoScheduler(t *testing.T) {
	ar := &AgentRuntime{}
	if _, err := cronCall(t, ar, `{"action":"list"}`); err == nil {
		t.Fatal("no scheduler must be an error the agent can read, not a nil deref")
	}
}

// The job runs as the turn's agent, read from the context — not as the shared
// runtime's default profile, which has no shell.
func TestTheCronToolSchedulesAsTheTurnsAgent(t *testing.T) {
	f := &fakeScheduler{}
	ar := &AgentRuntime{scheduler: f, agentConfig: &config.AgentConfig{ID: "main"}}
	ctx := context.WithValue(context.Background(), sharedctx.AgentIDKey, "coder")
	ctx = context.WithValue(ctx, sharedctx.BuddyToolsKey, []string{"bash", "cron"})
	ctx = context.WithValue(ctx, sharedctx.WorkdirKey, "/tmp/proj")
	ctx = context.WithValue(ctx, sharedctx.SessionIDKey, "sess-1")
	tool := ar.turnCronTool(ctx)
	if tool == nil {
		t.Fatal("a palette with cron must get the tool")
	}
	if _, err := tool.Function(json.RawMessage(`{"action":"every","every":"20s","task":"check"}`)); err != nil {
		t.Fatal(err)
	}
	if got := f.added[0]; got.AgentID != "coder" || got.Workdir != "/tmp/proj" || got.SessionKey != "sess-1" {
		t.Fatalf("job must carry the turn's agent, dir and session: %+v", got)
	}
	// A TUI chat turn carries the agent under the buddy-chat key instead.
	chat := context.WithValue(context.Background(), "buddy_agent_name", "coder")
	chat = context.WithValue(chat, sharedctx.BuddyToolsKey, []string{"bash", "cron"})
	if _, err := ar.turnCronTool(chat).Function(json.RawMessage(`{"action":"every","every":"20s","task":"check"}`)); err != nil {
		t.Fatal(err)
	}
	if got := f.added[1]; got.AgentID != "coder" {
		t.Fatalf("a chat turn's job must run as its agent, got %q", got.AgentID)
	}
	noCron := context.WithValue(ctx, sharedctx.BuddyToolsKey, []string{"bash"})
	if ar.turnCronTool(noCron) != nil {
		t.Fatal("a palette without cron must not get the tool")
	}
}
