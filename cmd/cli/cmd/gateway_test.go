package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// TestProbeGatewayHealth covers the four states the probe distinguishes:
// (1) port closed / no server, (2) a non-memdoor server, (3) a real
// memdoor gateway, (4) HTTP error response. Pins the bug fix: pre-fix,
// `memdoor gateway` would crash with EADDRINUSE even when a healthy
// gateway was already serving; the probe lets the command short-circuit
// to "already running, nothing to do."
func TestProbeGatewayHealth(t *testing.T) {
	t.Run("no server → not running", func(t *testing.T) {
		// Find a free port + close the listener so the probe hits a
		// dead address. testing.go's freeport pattern.
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("freeport: %v", err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()

		running, _ := probeGatewayHealth("127.0.0.1", port)
		if running {
			t.Fatalf("expected probe to report no gateway on closed port %d", port)
		}
	})

	t.Run("non-memdoor 200 → not running", func(t *testing.T) {
		// A bare HTTP server on the port replies 200 but doesn't carry
		// the memdoor-shaped fields. The probe must NOT mistake it for
		// an memdoor gateway — otherwise the install script would skip
		// starting our gateway just because nginx happens to be on 18789.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"ok":true}`)
		}))
		defer srv.Close()
		host, port := splitURL(t, srv.URL)
		running, _ := probeGatewayHealth(host, port)
		if running {
			t.Fatalf("expected probe to reject non-memdoor 200 response")
		}
	})

	t.Run("memdoor health → running", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version": "test-0.0.0",
				"uptime":  "1m",
				"agents":  3,
			})
		}))
		defer srv.Close()
		host, port := splitURL(t, srv.URL)
		running, info := probeGatewayHealth(host, port)
		if !running {
			t.Fatalf("expected probe to detect memdoor-shaped /health response")
		}
		if !strings.Contains(info, "version=test-0.0.0") {
			t.Fatalf("expected version in info, got %q", info)
		}
	})

	t.Run("HTTP 500 → not running", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()
		host, port := splitURL(t, srv.URL)
		running, _ := probeGatewayHealth(host, port)
		if running {
			t.Fatalf("expected probe to reject 500 response")
		}
	})
}

// TestIsAddrInUseErr verifies the typed-error path so we catch
// EADDRINUSE regardless of the surrounding Go stdlib wording.
func TestIsAddrInUseErr(t *testing.T) {
	t.Run("typed net.OpError EADDRINUSE → true", func(t *testing.T) {
		err := &net.OpError{Op: "listen", Net: "tcp", Err: syscall.EADDRINUSE}
		if !isAddrInUseErr(err) {
			t.Fatalf("expected isAddrInUseErr to match wrapped EADDRINUSE")
		}
	})

	t.Run("plain syscall.EADDRINUSE → true", func(t *testing.T) {
		if !isAddrInUseErr(syscall.EADDRINUSE) {
			t.Fatalf("expected isAddrInUseErr to match bare EADDRINUSE")
		}
	})

	t.Run("unrelated error → false", func(t *testing.T) {
		if isAddrInUseErr(fmt.Errorf("something else")) {
			t.Fatalf("isAddrInUseErr should not match unrelated errors")
		}
	})

	t.Run("nil → false", func(t *testing.T) {
		if isAddrInUseErr(nil) {
			t.Fatalf("isAddrInUseErr(nil) should be false")
		}
	})
}

// splitURL parses an httptest.Server URL into (host, port). Used by
// the probe tests to avoid hard-coding which random port httptest
// picked.
func splitURL(t *testing.T, raw string) (string, int) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	host := u.Hostname()
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port from %q: %v", raw, err)
	}
	return host, port
}
