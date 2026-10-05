package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGetSingleMessage_NotFound(t *testing.T) {
	cs := newTestChatServer()

	req := authenticatedRequest("GET", "/api/messages/99999", nil, "human:alice-uuid")
	rr := httptest.NewRecorder()

	// handleMessagesWithID routes GET to handleGetSingleMessage
	cs.handleMessagesWithID(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}
