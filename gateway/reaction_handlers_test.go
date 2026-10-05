package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleReactions_Add(t *testing.T) {
	cs := newTestChatServer()

	body := `{"emoji": "👍"}`
	// Reactions endpoint uses path: /api/messages/:id/reactions
	req := authenticatedRequest("POST", "/api/messages/42/reactions", strings.NewReader(body), "human:alice-uuid")
	rr := httptest.NewRecorder()

	cs.handleReactions(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["emoji"] != "👍" {
		t.Errorf("expected emoji '👍', got %v", resp["emoji"])
	}
	if resp["user_id"] != "human:alice-uuid" {
		t.Errorf("expected user_id 'human:alice-uuid', got %v", resp["user_id"])
	}
}

func TestHandleReactions_Unauthenticated(t *testing.T) {
	cs := newTestChatServer()

	body := `{"emoji": "👍"}`
	req := httptest.NewRequest("POST", "/api/messages/42/reactions", strings.NewReader(body))
	rr := httptest.NewRecorder()

	cs.handleReactions(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
