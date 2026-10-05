package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetThreadReplies_Empty(t *testing.T) {
	cs := newTestChatServer()

	// Request replies for a message (no replies exist)
	req := authenticatedRequest("GET", "/api/messages/42/replies", nil, "human:alice-uuid")
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
