package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/message"
)

func TestHandleGetThreadReplies_Empty(t *testing.T) {
	cs := newTestChatServer()
	parent := &message.Message{ChannelID: "c1"}
	id, err := cs.messageService.GetRepository().Save(context.Background(), parent)
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/messages/%d/replies", id)

	// Anonymous: nothing (the replies were anyone's, by sequential id, until 2026-10-30).
	anon := httptest.NewRecorder()
	cs.handleGetThreadReplies(anon, httptest.NewRequest("GET", path, nil))
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: status = %d, want 401", anon.Code)
	}
	// A message that does not exist has no thread to read.
	ghost := httptest.NewRecorder()
	cs.handleGetThreadReplies(ghost, authenticatedRequest("GET", "/api/messages/4242/replies", nil, "human:alice-uuid"))
	if ghost.Code != http.StatusNotFound {
		t.Fatalf("unknown parent: status = %d, want 404", ghost.Code)
	}

	// Request replies for a message (no replies exist)
	req := authenticatedRequest("GET", path, nil, "human:alice-uuid")
	rr := httptest.NewRecorder()

	cs.handleGetThreadReplies(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	replies, ok := resp["replies"].([]interface{})
	if !ok {
		t.Fatalf("expected replies array in response")
	}
	if len(replies) != 0 {
		t.Errorf("expected 0 replies, got %d", len(replies))
	}
}
