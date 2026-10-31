package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
	"memdoor/pkg/chatgpt"
)

// SIGN IN WITH CHATGPT (2026-10-10): a browser sign-in (provider_signin.go).
// On success the plan is probed the way /connect probes a key — its model
// list, one call — so the ✓ line says what it found.
//
//   POST /api/providers/chatgpt {"action":"login"}            → {id, auth_url}
//   POST …                      login/wait, login/paste, login/cancel (provider_signin.go)
//   POST …                      {"action":"logout"}
//   GET  /api/providers/chatgpt                                → {signed_in, email}

func (s *Server) handleProvidersChatGPT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ok := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	fail := func(status int, err error) { http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), status) }
	// The gateway's ChatGPT sign-in is its provider: the operator's alone
	// (anyone read its email and could sign it out or swap it until 2026-10-30).
	if !s.gatewayOperator(w, r) {
		return
	}
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
		id := startSignIn(login.Deliver, func(ctx context.Context) signInOutcome {
			if _, err := login.Wait(ctx); err != nil {
				return signInOutcome{err: err, again: errors.Is(err, chatgpt.ErrSignInAgain)}
			}
			return signInOutcome{finish: func(ctx context.Context) any {
				out := chatgptProbe(ctx)
				out.Done = true
				if out.OK {
					// The plan answers by default now, and a pin kept on the
					// OpenAI key for a model the plan serves moves to it
					// (providers/chatgpt_engine.go).
					out.Notes = pinsToPlan(out.list)
					s.selectModelEngine()
					if re := providers.ActiveRemoteEngine(); re != nil && re.IsChatGPTPlan() {
						out.Default = re.Model
					}
				}
				return out
			}}
		})
		ok(map[string]string{"id": id, "auth_url": login.AuthURL})
	case "login/wait", "login/paste", "login/cancel":
		signInStep(w, r, req.Action, req.ID, req.Value)
	case "logout":
		if err := providers.ChatGPTStore().Delete(); err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		providers.ForgetModels(providers.ChatGPTID)
		s.selectModelEngine()
		ok(map[string]bool{"ok": true})
	default:
		fail(http.StatusBadRequest, fmt.Errorf("action must be login, login/wait, login/paste, login/cancel or logout"))
	}
}

// chatgptProbeResult is the connect result with the sign-in's own fields.
type chatgptProbeResult struct {
	connectResult
	Done    bool     `json:"done"`
	Email   string   `json:"email,omitempty"`
	Default string   `json:"default,omitempty"` // the plan's model that answers by default now
	Notes   []string `json:"notes,omitempty"`   // pins moved from the OpenAI key to the plan
	list    []providers.Model
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
	res.Models, res.list = len(list), list
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
