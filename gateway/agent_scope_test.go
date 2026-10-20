package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/domain"

	"github.com/google/uuid"
)

func agentIn(name, workspace string) *domain.Buddy {
	return &domain.Buddy{ID: uuid.New(), Name: name, IsActive: true, WorkspaceScoped: domain.WorkspaceScoped{WorkspaceID: workspace}}
}

// Every workspace has its own agents: a person lists theirs and no other's;
// the gateway's own caller sees all (one user, one workspace, 2026-10-20).
func TestAPersonListsOnlyTheAgentsOfTheirWorkspace(t *testing.T) {
	cs := newTestChatServer()
	repo := cs.buddyRepo.(*mockBuddyRepo)
	repo.buddies = append(repo.buddies, agentIn("coder", "hackernews"), agentIn("secret-agent", "hackernews"), agentIn("coder", "bob"))
	list := func(actor, workspace string) []string {
		rr := httptest.NewRecorder()
		cs.handleAgents(rr, requestIn("GET", "/api/agents", actor, workspace))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s lists: %d %s", actor, rr.Code, rr.Body.String())
		}
		var resp struct {
			Agents []struct{ Name string } `json:"agents"`
		}
		_ = json.NewDecoder(rr.Body).Decode(&resp)
		var names []string
		for _, a := range resp.Agents {
			names = append(names, a.Name)
		}
		return names
	}
	if got := list("human:bob", "bob"); len(got) != 1 || got[0] != "coder" {
		t.Fatalf("bob sees his coder only: %v", got)
	}
	if got := list("human:greg", "hackernews"); len(got) != 2 {
		t.Fatalf("greg sees his two: %v", got)
	}
	if got := list("system:internal", "default"); len(got) != 3 {
		t.Fatalf("the gateway's own caller sees all: %v", got)
	}
}
