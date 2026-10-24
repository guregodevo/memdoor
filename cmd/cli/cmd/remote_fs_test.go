package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A turn on another machine with no --dir works in the same path as here
// when that machine has it, else in its home; never in this laptop's path.
func TestARemoteTurnWithNoDirHasAGoodDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	proj := filepath.Join(home, "Dev", "app")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	boxHas := map[string]bool{"~": true}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("path")
		abs := strings.Replace(p, "~", "/home/box", 1)
		w.Header().Set("Content-Type", "application/json")
		if boxHas[p] {
			_, _ = w.Write([]byte(`{"path":"` + abs + `","dir":true,"exists":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"path":"` + abs + `","dir":false,"exists":false}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_ = os.Chdir(proj)
	abs, why, err := remoteDefaultDir(c)
	if err != nil || abs != "/home/box" || why != "its home" {
		t.Fatalf("the box has no Dev/app: home. got %q %q %v", abs, why, err)
	}
	boxHas["~/Dev/app"] = true
	abs, why, err = remoteDefaultDir(c)
	if err != nil || abs != "/home/box/Dev/app" || why != "the same path as here" {
		t.Fatalf("the box has Dev/app: the same path. got %q %q %v", abs, why, err)
	}
	_ = os.Chdir(os.TempDir())
	if abs, _, _ = remoteDefaultDir(c); abs != "/home/box" {
		t.Fatalf("outside home here: the box's home, got %q", abs)
	}
}
