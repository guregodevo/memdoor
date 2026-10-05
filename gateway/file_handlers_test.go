package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleFileUpload_MethodNotAllowed(t *testing.T) {
	cs := newTestChatServer()

	// GET is not allowed on the upload endpoint
	req := authenticatedRequest("GET", "/api/files", nil, "human:alice-uuid")
	rr := httptest.NewRecorder()

	cs.handleFileUpload(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
	}
}

func TestHandleFileUpload_NoService(t *testing.T) {
	cs := newTestChatServer()
	cs.fileService = nil // explicitly nil

	req := authenticatedRequest("POST", "/api/files", nil, "human:alice-uuid")
	rr := httptest.NewRecorder()

	cs.handleFileUpload(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
}
