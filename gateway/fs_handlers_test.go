package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// A window on a laptop reads the project on the gateway's machine through
// /api/fs: a stat for /dir, a listing for the panel, a preview for its
// right column. Unauthenticated, nothing; outside the root, nothing.
func TestFSAnswersStatListAndReadForTheAuthenticated(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := &ChatServer{}
	call := func(actor string, q url.Values) (int, map[string]interface{}) {
		var req *http.Request
		if actor == "" {
			req = httptest.NewRequest("GET", "/api/fs?"+q.Encode(), nil)
		} else {
			req = authenticatedRequest("GET", "/api/fs?"+q.Encode(), nil, actor)
		}
		rec := httptest.NewRecorder()
		cs.handleFS(rec, req)
		var body map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	if code, _ := call("", url.Values{"op": {"stat"}, "path": {root}}); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", code)
	}
	// Someone outside the gateway's own workspace reads nothing (any
	// sign-up read /etc and ~/.memdoor here until 2026-10-30).
	if code, _ := call("human:stranger", url.Values{"op": {"read"}, "root": {root}, "path": {"pkg/a.go"}}); code != http.StatusForbidden {
		t.Fatalf("a stranger: %d", code)
	}
	code, body := call("system:internal", url.Values{"op": {"stat"}, "path": {root}})
	if code != 200 || body["dir"] != true || body["exists"] != true {
		t.Fatalf("stat: %d %v", code, body)
	}
	code, body = call("system:internal", url.Values{"op": {"list"}, "root": {root}})
	if code != 200 {
		t.Fatalf("list: %d %v", code, body)
	}
	files, _ := body["files"].([]interface{})
	if len(files) != 2 || files[0] != "pkg/" || files[1] != "pkg/a.go" {
		t.Fatalf("list files: %v", files)
	}
	code, body = call("system:internal", url.Values{"op": {"read"}, "root": {root}, "path": {"pkg/a.go"}})
	lines, _ := body["lines"].([]interface{})
	if code != 200 || len(lines) != 2 || lines[0] != "   1  package pkg" {
		t.Fatalf("read: %d %v", code, body)
	}
	if code, _ := call("system:internal", url.Values{"op": {"read"}, "root": {root}, "path": {"../etc/passwd"}}); code != http.StatusBadRequest {
		t.Fatalf("read outside root: %d", code)
	}
	if code, _ := call("system:internal", url.Values{"op": {"list"}, "root": {filepath.Join(root, "nowhere")}}); code != http.StatusNotFound {
		t.Fatalf("list missing root: %d", code)
	}
}
