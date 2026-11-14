package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// No test in this package writes the developer's ~/.memdoor (2026-11-14): a
// test posting a rung to /api/route as "human:p" kept that pin in the real
// ~/.memdoor/model.json, and the person's conversations answered on it.
func TestARoutePinInATestStaysInTheTestHome(t *testing.T) {
	if testHome == "" || !strings.HasPrefix(persistentPinPath(), testHome+string(filepath.Separator)) {
		t.Fatalf("the kept pin is written outside the test home %q: %s", testHome, persistentPinPath())
	}
	server, cleanup := createTestServer(t)
	defer cleanup()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := shared.WithExecutionContext(r.Context(), &shared.ExecutionContext{ActorID: "human:p", WorkspaceID: "w"})
		server.handleRoute(w, r.WithContext(authorization.WithActorID(ctx, "human:p")))
	}))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/api/route", "application/json", bytes.NewBufferString(`{"workspace":"w","channel_id":"c","agent":"coder","tier":2}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	t.Cleanup(func() { _ = writePersistentPin("human:p", nil) })
	if p := readPersistentPin("human:p"); p == nil || p.Rung != 2 {
		t.Fatalf("the rung is kept in the test home: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(testHome, shared.MemdoorDirName, persistentPinFile)); err != nil {
		t.Fatalf("the pin file is in the test home: %v", err)
	}
}
