package gateway

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"memdoor/pkg/filelist"
)

// THE WINDOW ON A LAPTOP, THE PROJECT ON A BOX (option 1, 2026-10-23). The
// files panel, the @-mentions and /dir read the project's directory. When
// the gateway is on another machine that directory is there, not where the
// window runs, so the window asks the gateway: GET /api/fs?op=stat|list|read.
// What is answered is what the coder can read anyway (reads go anywhere,
// 2026-10-05), for the gateway's own workspace (requireGatewayMember: any
// sign-up read /etc and ~/.memdoor here until 2026-10-30), with the same
// walk, ranking and preview as the local panel (pkg/filelist).
func (cs *ChatServer) handleFS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !requireGatewayMember(w, r, cs.repoFactory) {
		return
	}
	q := r.URL.Query()
	switch q.Get("op") {
	case "stat":
		abs, isDir, exists := filelist.Stat(q.Get("path"))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"path": abs, "dir": isDir, "exists": exists})
	case "list":
		root, isDir, exists := filelist.Stat(q.Get("root"))
		if !exists || !isDir {
			http.Error(w, `{"error":"no such directory on this machine: `+jsonSafe(root)+`"}`, http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"root": root, "files": filelist.List(root)})
	case "read":
		root, isDir, exists := filelist.Stat(q.Get("root"))
		if !exists || !isDir {
			http.Error(w, `{"error":"no such directory on this machine: `+jsonSafe(root)+`"}`, http.StatusNotFound)
			return
		}
		rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(q.Get("path"))))
		if rel == "." || rel == "" || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			http.Error(w, `{"error":"path must be relative to root"}`, http.StatusBadRequest)
			return
		}
		if strings.HasSuffix(q.Get("path"), "/") {
			rel += "/"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"lines": filelist.Preview(root, rel, q.Get("diff") == "1")})
	default:
		http.Error(w, `{"error":"op must be stat, list or read"}`, http.StatusBadRequest)
	}
}

// jsonSafe is a path inside a hand-written JSON error.
func jsonSafe(s string) string {
	b, _ := json.Marshal(s)
	return strings.Trim(string(b), `"`)
}
