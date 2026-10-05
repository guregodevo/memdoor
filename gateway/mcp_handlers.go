package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"memdoor/pkg/mcp"
)

// /api/mcp: the TUI's /mcp panel and `memdoor mcp` drive the person's MCP
// servers through these; the gateway runs the servers and holds the
// sign-ins. Every call names the project directory it is about.

// mcpStatusJSON is a server as the panel shows it.
type mcpStatusJSON struct {
	Name      string   `json:"name"`
	Scope     string   `json:"scope"`
	Transport string   `json:"transport"`
	Target    string   `json:"target"`
	State     string   `json:"state"`
	Error     string   `json:"error,omitempty"`
	Tools     []string `json:"tools,omitempty"`
	Server    string   `json:"server,omitempty"` // name and version the server gave
	SignedIn  bool     `json:"signed_in,omitempty"`
	// Saved is set by an add: false means the server was not kept.
	Saved bool `json:"saved,omitempty"`
}

func mcpStatusOut(st mcp.Status) mcpStatusJSON {
	out := mcpStatusJSON{Name: st.Name, Scope: string(st.Scope), Transport: st.Transport, Target: st.Target, State: string(st.State), Error: st.Err, Saved: st.Saved}
	for _, t := range st.Tools {
		out.Tools = append(out.Tools, t.Name)
	}
	if st.Info.Name != "" {
		out.Server = strings.TrimSpace(st.Info.Name + " " + st.Info.Version)
	}
	if st.Transport == "http" && mcpAuth != nil {
		out.SignedIn = mcpAuth.SignedIn(st.Target)
	}
	return out
}

type mcpRequest struct {
	Dir   string `json:"dir"`
	Name  string `json:"name"`
	Input string `json:"input"`
	Scope string `json:"scope"`
	On    bool   `json:"on"`
	ID    string `json:"id"`
	Value string `json:"value"`
}

// mcpLogins are the sign-ins waiting for a browser, by id.
var mcpLogins = struct {
	sync.Mutex
	m map[string]*mcpLogin
}{m: map[string]*mcpLogin{}}

type mcpLogin struct {
	login  *mcp.Login
	dir    string
	name   string
	cancel context.CancelFunc
	done   chan error
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mgr := mcpManager()
	fail := func(code int, err error) {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}
	ok := func(v interface{}) { _ = json.NewEncoder(w).Encode(v) }

	var req mcpRequest
	if r.Method == http.MethodGet {
		req.Dir = r.URL.Query().Get("dir")
	} else if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(http.StatusBadRequest, fmt.Errorf("unreadable request: %w", err))
		return
	}
	action := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/mcp"), "/")
	if !strings.HasPrefix(action, "login/") && (req.Dir == "" || !filepath.IsAbs(req.Dir)) {
		fail(http.StatusBadRequest, fmt.Errorf("dir must be the project's absolute path"))
		return
	}
	ctx := r.Context()
	serverOf := func(name string) (mcp.Server, error) {
		servers, err := mcpStore.Servers(req.Dir)
		if err != nil {
			return mcp.Server{}, err
		}
		for _, sv := range servers {
			if sv.Name == name {
				return sv, nil
			}
		}
		return mcp.Server{}, fmt.Errorf("no server named %q", name)
	}

	// keepTrust runs a change the person made through Memdoor to the
	// project's file: a project that was trusted stays trusted (its servers
	// are still the ones allowed, or fewer). It never grants trust a project
	// did not have.
	keepTrust := func(change func() error) error {
		was, _ := mcpStore.Trusted(req.Dir)
		if err := change(); err != nil {
			return err
		}
		if was {
			return mcpStore.Trust(req.Dir)
		}
		return nil
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		sts, err := mgr.Status(ctx, req.Dir)
		if err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		trusted, _ := mcpStore.Trusted(req.Dir)
		out := struct {
			Servers []mcpStatusJSON `json:"servers"`
			Trusted bool            `json:"trusted"`
		}{Servers: []mcpStatusJSON{}, Trusted: trusted}
		for _, st := range sts {
			out.Servers = append(out.Servers, mcpStatusOut(st))
		}
		ok(out)

	case action == "add":
		parsed, err := mcp.ParseAdd(req.Input)
		if err != nil {
			fail(http.StatusBadRequest, err)
			return
		}
		if req.Name != "" {
			if len(parsed) != 1 {
				fail(http.StatusBadRequest, fmt.Errorf("a name can be given for one server, not %d", len(parsed)))
				return
			}
			parsed[0].Name = req.Name
		}
		scope := mcp.ScopeProject
		if req.Scope == string(mcp.ScopeUser) {
			scope = mcp.ScopeUser
		}
		var out []mcpStatusJSON
		for _, p := range parsed {
			// The person added it here, so it is theirs to run: the file is
			// trusted after the add if it was before, or held nothing else.
			// Adding to a cloned repo does not approve the repo's own servers.
			before, _ := mcpStore.Servers(req.Dir)
			othersInProject := false
			for _, sv := range before {
				if sv.Scope == mcp.ScopeProject && sv.Name != p.Name {
					othersInProject = true
				}
			}
			was, _ := mcpStore.Trusted(req.Dir)
			if err := mcpStore.Add(scope, req.Dir, p.Name, p.Entry); err != nil {
				fail(http.StatusBadRequest, err)
				return
			}
			if scope == mcp.ScopeProject && (was || !othersInProject) {
				_ = mcpStore.Trust(req.Dir)
			}
			mgr.Forget(req.Dir, p.Name)
			tctx, cancel := context.WithTimeout(ctx, 90*time.Second)
			st := mgr.Test(tctx, req.Dir, p.Name)
			cancel()
			// A SERVER THAT CANNOT RUN IS NOT KEPT. Saving it leaves a red row
			// in every /mcp from then on and a committed .mcp.json that fails
			// for everyone who clones the repo (live 2026-10-01: a stray
			// `"a": {"command": "a"}`). The exception is a server waiting for a
			// variable — there the entry is right and the environment is not,
			// which is how a registry result with ${API_KEY} is meant to land.
			if st.State == mcp.StateFailed {
				if _, missing := (mcp.Server{Name: p.Name, Entry: p.Entry}).Spec(req.Dir, os.LookupEnv); missing == nil {
					_ = mcpStore.Remove(scope, req.Dir, p.Name)
					mgr.Forget(req.Dir, p.Name)
					st.Saved = false
				}
			}
			out = append(out, mcpStatusOut(st))
		}
		ok(map[string]interface{}{"servers": out})

	case action == "test":
		mgr.Forget(req.Dir, req.Name)
		tctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		ok(mcpStatusOut(mgr.Test(tctx, req.Dir, req.Name)))

	case action == "remove":
		sv, err := serverOf(req.Name)
		if err != nil {
			fail(http.StatusNotFound, err)
			return
		}
		if err := keepTrust(func() error { return mcpStore.Remove(sv.Scope, req.Dir, req.Name) }); err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		mgr.Forget(req.Dir, req.Name)
		ok(map[string]string{"removed": req.Name, "from": sv.Path})

	case action == "enable":
		sv, err := serverOf(req.Name)
		if err != nil {
			fail(http.StatusNotFound, err)
			return
		}
		if err := keepTrust(func() error { return mcpStore.SetDisabled(sv.Scope, req.Dir, req.Name, !req.On) }); err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		mgr.Forget(req.Dir, req.Name)
		ok(map[string]bool{"on": req.On})

	case action == "trust":
		if err := mcpStore.Trust(req.Dir); err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		ok(map[string]bool{"trusted": true})

	case action == "login":
		sv, err := serverOf(req.Name)
		if err != nil {
			fail(http.StatusNotFound, err)
			return
		}
		if !sv.Entry.Remote() {
			fail(http.StatusBadRequest, fmt.Errorf("%s is a local command: there is nothing to sign in to", req.Name))
			return
		}
		sp, err := sv.Spec(req.Dir, os.LookupEnv)
		if err != nil && sp.URL == "" {
			fail(http.StatusBadRequest, err)
			return
		}
		lctx, lcancel := context.WithTimeout(context.Background(), 30*time.Second)
		login, err := mcpAuth.BeginLogin(lctx, sp.URL, "")
		lcancel()
		if err != nil {
			fail(http.StatusBadGateway, err)
			return
		}
		wctx, cancel := context.WithCancel(context.Background())
		l := &mcpLogin{login: login, dir: req.Dir, name: req.Name, cancel: cancel, done: make(chan error, 1)}
		id := fmt.Sprintf("%d", time.Now().UnixNano())
		mcpLogins.Lock()
		mcpLogins.m[id] = l
		mcpLogins.Unlock()
		go func() { l.done <- login.Wait(wctx) }()
		ok(map[string]string{"id": id, "auth_url": login.AuthURL})

	case action == "login/wait", action == "login/paste", action == "login/cancel":
		mcpLogins.Lock()
		l := mcpLogins.m[req.ID]
		mcpLogins.Unlock()
		if l == nil {
			fail(http.StatusNotFound, fmt.Errorf("no sign-in %q in progress", req.ID))
			return
		}
		switch action {
		case "login/paste":
			if err := l.login.Deliver(req.Value); err != nil {
				fail(http.StatusBadRequest, err)
				return
			}
			ok(map[string]bool{"delivered": true})
		case "login/cancel":
			l.cancel()
			ok(map[string]bool{"cancelled": true})
		default:
			select {
			case err := <-l.done:
				mcpLogins.Lock()
				delete(mcpLogins.m, req.ID)
				mcpLogins.Unlock()
				l.cancel()
				if err != nil {
					ok(map[string]string{"error": err.Error()})
					return
				}
				mgr.Forget(l.dir, l.name)
				tctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				ok(map[string]interface{}{"done": true, "server": mcpStatusOut(mgr.Test(tctx, l.dir, l.name))})
			case <-time.After(25 * time.Second):
				ok(map[string]bool{"pending": true})
			case <-ctx.Done():
			}
		}

	case r.Method == http.MethodGet && action == "search":
		q := r.URL.Query().Get("q")
		sctx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		found, err := mcp.SearchRegistry(sctx, nil, q, 30)
		if err != nil {
			fail(http.StatusBadGateway, err)
			return
		}
		type need struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
		}
		type result struct {
			Name        string    `json:"name"`
			Suggested   string    `json:"suggested"`
			Description string    `json:"description"`
			Version     string    `json:"version"`
			Via         string    `json:"via"`
			Needs       []need    `json:"needs,omitempty"`
			Entry       mcp.Entry `json:"entry"`
		}
		out := []result{}
		for _, f := range found {
			r := result{Name: f.Name, Suggested: f.Suggested, Description: f.Description, Version: f.Version, Via: f.Via, Entry: f.Entry}
			for _, n := range f.Needs {
				r.Needs = append(r.Needs, need{n.Name, n.Description})
			}
			out = append(out, r)
		}
		ok(map[string]interface{}{"servers": out})

	case r.Method == http.MethodGet && action == "prompts":
		// The project's servers connect as a turn would, then their prompts.
		mgr.Tools(ctx, req.Dir)
		type arg struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Required    bool   `json:"required,omitempty"`
		}
		type prompt struct {
			Server      string `json:"server"`
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Arguments   []arg  `json:"arguments,omitempty"`
		}
		out := []prompt{}
		for _, p := range mgr.Prompts(ctx, req.Dir) {
			pr := prompt{Server: p.Server, Name: p.Name, Description: p.Description}
			for _, a := range p.Arguments {
				pr.Arguments = append(pr.Arguments, arg{a.Name, a.Description, a.Required})
			}
			out = append(out, pr)
		}
		ok(map[string]interface{}{"prompts": out})

	case action == "prompt":
		var body struct {
			Server string            `json:"server"`
			Args   map[string]string `json:"args"`
		}
		_ = json.Unmarshal([]byte(req.Value), &body)
		msgs, err := mgr.GetPrompt(ctx, req.Dir, body.Server, req.Name, body.Args)
		if err != nil {
			fail(http.StatusBadGateway, err)
			return
		}
		var parts []string
		for _, m := range msgs {
			text, _ := mcp.ResultText(&mcp.ToolResult{Content: []mcp.Content{m.Content}})
			if m.Role == "assistant" {
				text = "(assistant) " + text
			}
			parts = append(parts, text)
		}
		ok(map[string]string{"text": strings.Join(parts, "\n\n")})

	case action == "logout":
		sv, err := serverOf(req.Name)
		if err != nil {
			fail(http.StatusNotFound, err)
			return
		}
		if err := mcpAuth.SignOut(sv.Entry.URL); err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		mgr.Forget(req.Dir, req.Name)
		ok(map[string]bool{"signed_out": true})

	default:
		fail(http.StatusNotFound, fmt.Errorf("no MCP action %q", action))
	}
}
