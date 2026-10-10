package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
	"memdoor/pkg/chatgpt"
)

// SIGN IN WITH CHATGPT (2026-10-10): the browser step runs here, in the
// gateway, like the MCP sign-in (mcp_handlers.go): the loopback listener is
// the gateway's, the CLI or the window opens the link and polls. On
// success the plan is probed the way /connect probes a key — its model
// list, one call — so the ✓ line says what it found.
//
//   POST /api/providers/chatgpt {"action":"login"}            → {id, auth_url}
//   POST …                      {"action":"login/wait","id"}  → {done, …connect result} | {pending} | {error}
//   POST …                      {"action":"login/paste","id","value"}
//   POST …                      {"action":"login/cancel","id"}
//   POST …                      {"action":"logout"}
//   GET  /api/providers/chatgpt                                → {signed_in, email}

var chatgptLogins = struct {
	sync.Mutex
	m map[string]*chatgptLogin
}{m: map[string]*chatgptLogin{}}

type chatgptLogin struct {
	login  *chatgpt.Login
	cancel context.CancelFunc
	done   chan chatgptLoginResult
}

type chatgptLoginResult struct {
	cred *chatgpt.Credential
	err  error
}

func (s *Server) handleProvidersChatGPT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ok := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	fail := func(status int, err error) { http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), status) }
	if r.Method == http.MethodGet {
		email := providers.ChatGPTSignedIn()
		ok(map[string]any{"signed_in": email != "", "email": email})
		return
	}
	if r.Method != http.MethodPost {
		fail(http.StatusMethodNotAllowed, fmt.Errorf("GET or POST"))
		return
	}
	var req struct {
		Action string `json:"action"`
		ID     string `json:"id"`
		Value  string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(http.StatusBadRequest, fmt.Errorf("a JSON body: action, id, value"))
		return
	}
	switch req.Action {
	case "login":
		lctx, lcancel := context.WithTimeout(context.Background(), 30*time.Second)
		login, err := chatgpt.BeginLogin(lctx, chatgpt.Default(), providers.ChatGPTStore(), nil)
		lcancel()
		if err != nil {
			fail(http.StatusBadGateway, err)
			return
		}
		wctx, cancel := context.WithCancel(context.Background())
		l := &chatgptLogin{login: login, cancel: cancel, done: make(chan chatgptLoginResult, 1)}
		id := fmt.Sprintf("%d", time.Now().UnixNano())
		chatgptLogins.Lock()
		chatgptLogins.m[id] = l
		chatgptLogins.Unlock()
		go func() { c, err := login.Wait(wctx); l.done <- chatgptLoginResult{c, err} }()
		ok(map[string]string{"id": id, "auth_url": login.AuthURL})
	case "login/wait", "login/paste", "login/cancel":
		chatgptLogins.Lock()
		l := chatgptLogins.m[req.ID]
		chatgptLogins.Unlock()
		if l == nil {
			fail(http.StatusNotFound, fmt.Errorf("no sign-in %q in progress", req.ID))
			return
		}
		switch req.Action {
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
			case res := <-l.done:
				chatgptLogins.Lock()
				delete(chatgptLogins.m, req.ID)
				chatgptLogins.Unlock()
				l.cancel()
				if res.err != nil {
					ok(map[string]string{"error": res.err.Error()})
					return
				}
				out := chatgptProbe(r.Context())
				out.Done = true
				ok(out)
			case <-time.After(25 * time.Second):
				ok(map[string]bool{"pending": true})
			case <-r.Context().Done():
			}
		}
	case "logout":
		if err := providers.ChatGPTStore().Delete(); err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		providers.ForgetModels(providers.ChatGPTID)
		ok(map[string]bool{"ok": true})
	default:
		fail(http.StatusBadRequest, fmt.Errorf("action must be login, login/wait, login/paste, login/cancel or logout"))
	}
}

// chatgptProbeResult is the connect result with the sign-in's own fields.
type chatgptProbeResult struct {
	connectResult
	Done  bool   `json:"done"`
	Email string `json:"email,omitempty"`
}

// chatgptProbe is /connect's probe on the plan just signed in: the models
// it lists, then one call on the first.
func chatgptProbe(ctx context.Context) chatgptProbeResult {
	res := chatgptProbeResult{connectResult: connectResult{ID: providers.ChatGPTID, API: providers.APIChatGPT, Saved: true}, Email: providers.ChatGPTSignedIn()}
	providers.ForgetModels(providers.ChatGPTID)
	p, found := providers.FindProvider(providers.ChatGPTID)
	if !found || !p.Connected() {
		res.Error = "the sign-in was not kept"
		return res
	}
	list, err := providers.ModelsOf(ctx, p)
	if err != nil {
		res.Error = oneLine(err.Error(), 200)
		return res
	}
	if len(list) == 0 {
		res.Error = "the ChatGPT plan lists no model for this app"
		return res
	}
	res.Models = len(list)
	for i, m := range list {
		if i == 5 {
			break
		}
		res.Sample = append(res.Sample, m.ID)
	}
	answer, err := probeCall(ctx, p, list[0].ID)
	if err != nil {
		res.Error = oneLine(err.Error(), 200)
		return res
	}
	if answer == "" {
		answer = "(a reply with no text)"
	}
	res.OK, res.Tested, res.Answer = true, list[0].ID, answer
	logs.New("Providers").Info("ChatGPT plan connected", "models", len(list), "tested", list[0].ID)
	return res
}
