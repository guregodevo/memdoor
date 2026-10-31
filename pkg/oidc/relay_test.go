package oidc

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return port
}

func linkTo(redirect string) string {
	return "https://auth.example/authorize?" + url.Values{"redirect_uri": {redirect}, "state": {"S"}}.Encode()
}

func get(t *testing.T, u string) (int, string) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// Only a loopback http redirect with a port can be caught here.
func TestRedirectOfTakesOnlyALoopback(t *testing.T) {
	if r, err := RedirectOf(linkTo("http://127.0.0.1:1455/callback")); err != nil || r.Port() != "1455" || r.Path != "/callback" {
		t.Fatalf("a loopback redirect: %v %v", r, err)
	}
	for _, bad := range []string{
		"https://auth.example/authorize?state=S", // no redirect_uri
		linkTo("https://127.0.0.1:1455/callback"),
		linkTo("http://evil.example:1455/callback"),
		linkTo("http://127.0.0.1/callback"), // no port
		"://not a link",
	} {
		if _, err := RedirectOf(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}

// The browser's return reaches deliver whole, the sign-in's page answers in
// words, and nothing but the redirect's path with a query is passed on (the
// mutation check: deliver on any path and the stray request is passed on).
func TestRelayPassesTheReturnOn(t *testing.T) {
	port := freePort(t)
	redirect := "http://127.0.0.1:" + port + "/callback"
	var got []string
	refuse := false
	r, err := StartRelay(linkTo(redirect), func(address string) error {
		got = append(got, address)
		if refuse {
			return errors.New("that link belongs to another sign-in")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()

	if code, body := get(t, redirect+"?code=C&state=S"); code != 200 || !strings.Contains(body, "Signed in to Memdoor") {
		t.Fatalf("the return: %d %s", code, body)
	}
	if len(got) != 1 || got[0] != redirect+"?code=C&state=S" {
		t.Fatalf("delivered %q", got)
	}
	if code, _ := get(t, "http://127.0.0.1:"+port+"/other?code=C&state=S"); code != 404 {
		t.Fatalf("another path: %d", code)
	}
	if code, _ := get(t, redirect); code != 404 {
		t.Fatalf("no query: %d", code)
	}
	if len(got) != 1 {
		t.Fatalf("only the return is passed on: %q", got)
	}
	if code, body := get(t, redirect+"?error=access_denied&state=S"); code != 200 || !strings.Contains(body, "Sign-in refused") || !strings.Contains(body, "access_denied") {
		t.Fatalf("a refusal is passed on and said: %d %s", code, body)
	}
	refuse = true
	if code, body := get(t, redirect+"?code=C&state=X"); code != 400 || !strings.Contains(body, "another sign-in") || strings.Contains(body, "Signed in") {
		t.Fatalf("a return the sign-in refuses: %d %s", code, body)
	}
}

// A port taken here fails the relay (the caller asks for a paste), and Stop
// gives the port back.
func TestRelayPortTakenAndStop(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redirect := "http://" + ln.Addr().String() + "/callback"
	if _, err := StartRelay(linkTo(redirect), func(string) error { return nil }); err == nil {
		t.Fatal("a taken port must fail the relay")
	}
	ln.Close()
	r, err := StartRelay(linkTo(redirect), func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	r.Stop()
	again, err := net.Listen("tcp", strings.TrimPrefix(strings.TrimSuffix(redirect, "/callback"), "http://"))
	if err != nil {
		t.Fatalf("Stop must free the port: %v", err)
	}
	again.Close()
	var none *Relay
	none.Stop()
}
