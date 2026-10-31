package oidc

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RedirectOf is the redirect_uri a sign-in link carries, when it is a
// loopback one (http://127.0.0.1:<port>/<path> or localhost): the address the
// browser comes back to.
func RedirectOf(authURL string) (*url.URL, error) {
	u, err := url.Parse(authURL)
	if err != nil {
		return nil, fmt.Errorf("not a link: %w", err)
	}
	r, err := url.Parse(u.Query().Get("redirect_uri"))
	if err != nil || r.Scheme != "http" || r.Port() == "" {
		return nil, errors.New("the link has no loopback redirect_uri")
	}
	switch r.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return r, nil
	}
	return nil, fmt.Errorf("the redirect %s is not a loopback address", r.Host)
}

// Relay catches a browser's return on THIS machine for a sign-in that runs on
// another (a gateway over ssh, or on a box): it listens on the loopback port
// the link's redirect_uri names and hands the whole address it receives to
// deliver, which passes it on the way a pasted one goes. The sign-in's own
// state check stays where the sign-in is. Stop closes it.
type Relay struct {
	srv  *http.Server
	done chan struct{}
}

const relayPage = `<!doctype html><meta charset="utf-8"><title>Memdoor</title>
<body style="font-family:system-ui;max-width:32rem;margin:15vh auto;text-align:center">
<h2>%s</h2><p>%s</p></body>`

// StartRelay listens for authURL's redirect. It fails when the link has no
// loopback redirect or its port is taken here: the caller then asks for the
// address to be pasted.
func StartRelay(authURL string, deliver func(address string) error) (*Relay, error) {
	redirect, err := RedirectOf(authURL)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", redirect.Port()))
	if err != nil {
		return nil, fmt.Errorf("port %s is taken here: %w", redirect.Port(), err)
	}
	path := redirect.Path
	if path == "" {
		path = "/"
	}
	r := &Relay{done: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if req.URL.Path != path || req.URL.RawQuery == "" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, relayPage, "Not a sign-in", "Start again from Memdoor.")
			return
		}
		back := *redirect
		back.RawQuery = req.URL.RawQuery
		if err := deliver(back.String()); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, relayPage, "Not signed in", html.EscapeString(err.Error())+". Start again from Memdoor.")
			return
		}
		if e := req.URL.Query().Get("error"); e != "" {
			fmt.Fprintf(w, relayPage, "Sign-in refused", html.EscapeString(strings.TrimSpace(e+" "+req.URL.Query().Get("error_description")))+". You can close this tab.")
			return
		}
		fmt.Fprintf(w, relayPage, "Signed in to Memdoor", "You can close this tab and go back to the terminal.")
	})
	r.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = r.srv.Serve(ln); close(r.done) }()
	return r, nil
}

// Stop closes the listener; a nil Relay is a no-op.
func (r *Relay) Stop() {
	if r == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = r.srv.Shutdown(ctx)
	<-r.done
}
