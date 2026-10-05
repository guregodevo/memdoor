package gateway

import (
	"context"
	"testing"

	"memdoor/gateway/queue"
)

func sessionWith(meta map[string]interface{}) *Session {
	s := &Session{ID: "workspace:w:channel:c", Metadata: map[string]interface{}{}}
	for k, v := range meta {
		s.Metadata[k] = v
	}
	return s
}

// resolvePostingAgentID picks the agent that actually ran, so its channel
// message is attributed correctly — never the old hardcoded "writer".
func TestResolvePostingAgentID(t *testing.T) {
	ctxWithName := func(name string) context.Context {
		return context.WithValue(context.Background(), "buddy_agent_name", name)
	}

	tests := []struct {
		name    string
		ctx     context.Context
		job     *queue.AgentJob
		session *Session
		want    string
	}{
		{
			name:    "job.AgentID wins over everything",
			ctx:     ctxWithName("coder"),
			job:     &queue.AgentJob{AgentID: "planner", SessionKey: "agent:archivist:x"},
			session: sessionWith(map[string]interface{}{"transcript_agent": "chief"}),
			want:    "planner",
		},
		{
			name:    "buddy_agent_name from ctx when no job.AgentID",
			ctx:     ctxWithName("chief"),
			job:     &queue.AgentJob{SessionKey: "workspace:w:channel:c"},
			session: sessionWith(map[string]interface{}{"transcript_agent": "coder"}),
			want:    "chief",
		},
		{
			name:    "transcript_agent from session when ctx has none",
			ctx:     context.Background(),
			job:     &queue.AgentJob{SessionKey: "workspace:w:channel:c"},
			session: sessionWith(map[string]interface{}{"transcript_agent": "researcher"}),
			want:    "researcher",
		},
		{
			name:    "agent session key as last resort",
			ctx:     context.Background(),
			job:     &queue.AgentJob{SessionKey: "agent:coder:step-1"},
			session: sessionWith(nil),
			want:    "coder",
		},
		{
			name:    "unresolvable → empty (never fabricate an author)",
			ctx:     context.Background(),
			job:     &queue.AgentJob{SessionKey: "workspace:w:channel:c"},
			session: sessionWith(nil),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePostingAgentID(tt.ctx, tt.job, tt.session)
			if got != tt.want {
				t.Errorf("resolvePostingAgentID = %q, want %q", got, tt.want)
			}
			if got == "writer" {
				t.Error("regression: resolver produced the hardcoded 'writer'")
			}
		})
	}
}
