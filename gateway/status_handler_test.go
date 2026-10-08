package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestStatusAnswersWithTheVersion(t *testing.T) {
	rec := httptest.NewRecorder()
	handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("body %q", rec.Body.String())
	}
	if _, ok := body["version"]; !ok {
		t.Fatal("the version is part of the answer")
	}
	if pid, _ := body["pid"].(float64); int(pid) != os.Getpid() {
		t.Fatalf("the pid tells a gateway Memdoor started from one run by hand: %v", body["pid"])
	}
}
