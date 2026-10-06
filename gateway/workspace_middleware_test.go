package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"memdoor/pkg/shared"
)

// TestOnlyAnExistingWorkspaceIsAURLPrefix: /acme/api/x is the workspace acme
// when acme exists; /zzz and /cyberlaw/legislation stay what they are, so the
// catch-all can answer 404 instead of the home page.
func TestOnlyAnExistingWorkspaceIsAURLPrefix(t *testing.T) {
	var gotPath, gotSlug string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSlug = ""
		if ec := shared.GetExecutionContext(r.Context()); ec != nil {
			gotSlug = ec.WorkspaceSlug
		}
	})
	h := workspaceMiddleware(inner, func(s string) bool { return s == "acme" })
	for _, c := range []struct{ in, path, slug string }{
		{"/acme/api/x", "/api/x", "acme"},
		{"/acme", "/", "acme"},
		{"/zzz", "/zzz", ""},
		{"/cyberlaw/legislation", "/cyberlaw/legislation", ""},
		{"/pricing", "/pricing", ""},
		{"/features", "/features", ""},
	} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, c.in, nil))
		if gotPath != c.path || gotSlug != c.slug {
			t.Errorf("%s → path %q slug %q, want %q %q", c.in, gotPath, gotSlug, c.path, c.slug)
		}
	}
}
