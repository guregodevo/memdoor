package gateway

import (
	"context"
	"testing"

	"memdoor/pkg/domain"
	"memdoor/pkg/shared"
)

// cronWorkspaceID must map the internal-service placeholder ("default") and an
// absent context to the real single-tenant workspace, so an agent-created job
// and a CLI `cron list` scope to the SAME workspace. This is the bug: the agent
// stored under "default", the CLI queried DefaultWorkspaceID, and the job was
// invisible.
func TestCronWorkspaceID(t *testing.T) {
	withWS := func(id string) context.Context {
		return shared.WithExecutionContext(context.Background(), &shared.ExecutionContext{WorkspaceID: id})
	}

	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"internal-service placeholder", withWS("default"), domain.DefaultWorkspaceID},
		{"empty workspace", withWS(""), domain.DefaultWorkspaceID},
		{"no execution context", context.Background(), domain.DefaultWorkspaceID},
		{"a real workspace passes through", withWS("acme-corp"), "acme-corp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cronWorkspaceID(tt.ctx); got != tt.want {
				t.Errorf("cronWorkspaceID = %q, want %q", got, tt.want)
			}
		})
	}

	// The two callers that used to disagree must now agree: an internal
	// (agent) request and a single-tenant token request resolve identically.
	agent := cronWorkspaceID(withWS("default"))
	cli := cronWorkspaceID(withWS(domain.DefaultWorkspaceID))
	if agent != cli {
		t.Fatalf("agent (%q) and CLI (%q) resolve to different workspaces — the invisible-job bug", agent, cli)
	}
}
