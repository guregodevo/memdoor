package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memdoor/pkg/authorization"
	"memdoor/pkg/domain"

	"github.com/google/uuid"
)

func TestHandleAgents_GET(t *testing.T) {
	cs := newTestChatServer()

	// Seed 2 buddies
	buddyRepo := cs.buddyRepo.(*mockBuddyRepo)
	buddyRepo.buddies = []*domain.Buddy{
		// ModelName was removed from domain.Buddy in the byok refactor —
		// the per-agent model setting now lives in workspace_settings.
		{
			ID:       uuid.New(),
			Name:     "writer",
			IsActive: true,
			Tools:    []string{"web_search"},
			Timestamps: domain.Timestamps{
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		{
			ID:       uuid.New(),
			Name:     "coder",
			IsActive: true,
			Tools:    []string{"read_file"},
			Timestamps: domain.Timestamps{
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	req := authenticatedRequest("GET", "/api/agents", nil, "human:alice-uuid")
	rr := httptest.NewRecorder()

	cs.handleAgents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	agents, ok := resp["agents"].([]interface{})
	if !ok {
		t.Fatalf("expected agents array in response")
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestHandleCreateAgent_Success(t *testing.T) {
	cs := newTestChatServer()

	body := `{
		"name": "test-bot",
		"model_name": "claude-3",
		"tools": ["web_search"],
		"sandbox_scope": "user"
	}`
	// Use system:internal to bypass admin check (authzService is nil)
	req := authenticatedRequest("POST", "/api/agents", strings.NewReader(body), "system:internal")
	rr := httptest.NewRecorder()

	cs.handleAgents(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["name"] != "test-bot" {
		t.Errorf("expected name 'test-bot', got %v", resp["name"])
	}
	if resp["status"] != "online" {
		t.Errorf("expected status 'online', got %v", resp["status"])
	}
}

// withRealAuthz swaps in a real authorization.Service so the admin-gate code
// path actually runs. IsWorkspaceAdmin short-circuits on the actor prefix
// (system:internal => admin, anything not "human:" => not admin) without
// touching the authRepo, so nil repos are fine for these tests.
func withRealAuthz(cs *ChatServer) {
	cs.authzService = authorization.NewService(
		&mockMembershipRepo{}, &mockChannelRepo{}, &mockBuddyRepo{}, nil,
	)
}

// A non-admin caller (e.g. a guest token) must not be able to touch the
// system-wide secrets store. Regression test for the missing admin gate on
// handleSecrets — the sibling handleAgentSecrets had it, this one didn't.
func TestHandleSecrets_NonAdminForbidden(t *testing.T) {
	for _, method := range []string{"GET", "POST", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			cs := newTestChatServer()
			withRealAuthz(cs)
			// Seed a secret so a missing gate would actually leak/destroy it.
			secretRepo := cs.agentSecretRepo.(*mockAgentSecretRepo)
			_ = secretRepo.Set(nil, "system", "openai_api_key", "sk-supersecret", "admin")

			body := strings.NewReader(`{"name":"openai_api_key","value":"sk-evil"}`)
			req := authenticatedRequest(method, "/api/secrets/openai_api_key", body, "guest:mallory")
			rr := httptest.NewRecorder()

			cs.handleSecrets(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusForbidden, rr.Body.String())
			}
			// The secret must be untouched/unreadable.
			v, _ := secretRepo.Get(nil, "system", "openai_api_key")
			if v != "sk-supersecret" {
				t.Errorf("secret was modified by non-admin: got %q", v)
			}
		})
	}
}

// An admin (system:internal short-circuits to admin) passes the gate and
// reaches the handler body.
func TestHandleSecrets_AdminAllowed(t *testing.T) {
	cs := newTestChatServer()
	withRealAuthz(cs)

	req := authenticatedRequest("GET", "/api/secrets", nil, "system:internal")
	rr := httptest.NewRecorder()

	cs.handleSecrets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

// Unauthenticated callers are rejected before the admin check.
func TestHandleSecrets_Unauthenticated(t *testing.T) {
	cs := newTestChatServer()
	withRealAuthz(cs)

	req := httptest.NewRequest("GET", "/api/secrets", nil)
	rr := httptest.NewRecorder()

	cs.handleSecrets(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

// Mutating an agent (PUT can repoint it at an attacker-controlled remote
// endpoint; DELETE removes it) must be admin-only. Regression for the
// unguarded handleAgentByName mutation paths.
func TestHandleAgentByName_MutationsRequireAdmin(t *testing.T) {
	seed := func(cs *ChatServer) {
		cs.buddyRepo.(*mockBuddyRepo).buddies = []*domain.Buddy{
			{ID: uuid.New(), Name: "writer", IsActive: true},
		}
	}

	t.Run("PUT denied for non-admin", func(t *testing.T) {
		cs := newTestChatServer()
		withRealAuthz(cs)
		seed(cs)
		body := strings.NewReader(`{"endpoint":"http://evil.example/agent"}`)
		req := authenticatedRequest("PUT", "/api/agents/writer", body, "guest:mallory")
		rr := httptest.NewRecorder()
		cs.handleAgentByName(rr, req, "writer")
		if rr.Code != http.StatusForbidden {
			t.Fatalf("PUT status = %d, want 403; body = %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("DELETE denied for non-admin", func(t *testing.T) {
		cs := newTestChatServer()
		withRealAuthz(cs)
		seed(cs)
		req := authenticatedRequest("DELETE", "/api/agents/writer", nil, "guest:mallory")
		rr := httptest.NewRecorder()
		cs.handleAgentByName(rr, req, "writer")
		if rr.Code != http.StatusForbidden {
			t.Fatalf("DELETE status = %d, want 403; body = %s", rr.Code, rr.Body.String())
		}
		// The agent must still exist.
		if len(cs.buddyRepo.(*mockBuddyRepo).buddies) != 1 {
			t.Error("agent was deleted by a non-admin")
		}
	})

	t.Run("admin (system:internal) is allowed through the gate", func(t *testing.T) {
		cs := newTestChatServer()
		withRealAuthz(cs)
		seed(cs)
		req := authenticatedRequest("DELETE", "/api/agents/writer", nil, "system:internal")
		rr := httptest.NewRecorder()
		cs.handleAgentByName(rr, req, "writer")
		if rr.Code != http.StatusOK {
			t.Fatalf("admin DELETE status = %d, want 200; body = %s", rr.Code, rr.Body.String())
		}
	})
}

// Creating/deleting a cron job runs an agent on a schedule — admin only.
func TestHandleCron_MutationsRequireAdmin(t *testing.T) {
	t.Run("POST create denied for non-admin", func(t *testing.T) {
		cs := newTestChatServer()
		withRealAuthz(cs)
		body := strings.NewReader(`{"id":"j1","schedule":"* * * * *"}`)
		req := authenticatedRequest("POST", "/api/cron", body, "guest:mallory")
		rr := httptest.NewRecorder()
		cs.handleCronJobs(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rr.Code, rr.Body.String())
		}
		if len(cs.cronRepo.(*mockCronJobRepo).jobs) != 0 {
			t.Error("a non-admin created a cron job")
		}
	})

	t.Run("DELETE denied for non-admin", func(t *testing.T) {
		cs := newTestChatServer()
		withRealAuthz(cs)
		req := authenticatedRequest("DELETE", "/api/cron/j1", nil, "guest:mallory")
		rr := httptest.NewRecorder()
		cs.handleCronJob(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rr.Code, rr.Body.String())
		}
	})
}

func TestHandleAgentByName_NotFound(t *testing.T) {
	cs := newTestChatServer()

	req := authenticatedRequest("GET", "/api/agents/nonexistent", nil, "human:alice-uuid")
	rr := httptest.NewRecorder()

	cs.handleAgentByName(rr, req, "nonexistent")

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}
