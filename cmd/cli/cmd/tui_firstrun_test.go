package cmd

import (
	"fmt"
	"memdoor/internal/testenv"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// #123: a port held by something that is not a gateway (a dev server) is
// refused in words that name what the CLI reads to go elsewhere. It said
// MEMDOOR_PORT, which only the installer and the ssh door read: run as told,
// the window went back to the same port.
func TestATakenPortNamesWhatTheCLIReads(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	dev := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html>a dev server</html>")
	}))
	defer dev.Close()
	gatewayAddr = dev.URL
	port := mustPort(t, dev.URL)

	err := machineReady()
	if err == nil {
		t.Fatal("a port held by a dev server must be refused")
	}
	next := strconv.Itoa(port + 1)
	for _, want := range []string{"port " + strconv.Itoa(port) + " is taken", "MEMDOOR_GATEWAY=http://localhost:" + next, "--gateway localhost:" + next} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "MEMDOOR_PORT") {
		t.Errorf("MEMDOOR_PORT is the installer's, the CLI never reads it: %v", err)
	}
}

func TestATakenPortAtTheTopSuggestsTheDefaultNeighbour(t *testing.T) {
	if err := portTaken("65535"); !strings.Contains(err.Error(), "localhost:18790") {
		t.Fatalf("no port above 65535: %v", err)
	}
}

func mustPort(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return p
}
