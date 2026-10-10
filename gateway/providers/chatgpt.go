package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"memdoor/pkg/chatgpt"
	"memdoor/pkg/shared"
)

// A CHATGPT PLAN IS A PROVIDER (2026-10-10): a Plus or Pro subscriber signs
// in once in the browser (pkg/chatgpt) and their plan's allowance answers
// Memdoor's turns through the Responses API, no API key anywhere. It is the
// one free route for the largest group of developers who could not try a
// BYOK agent before; OpenCode, Pi, Kilo and Amp ship the same. The provider
// is built in: connected when the sign-in is on disk, listed in /model like
// any other, pinned as "chatgpt:<slug>".

const (
	// APIChatGPT is the wire: the Responses API under a plan's bearer token,
	// with the fields the preview refuses left out (responses.go).
	APIChatGPT = "chatgpt"
	// ChatGPTID is the provider's id.
	ChatGPTID = "chatgpt"
	// VendorChatGPT is the engine vendor a pinned plan model runs under.
	VendorChatGPT = "chatgpt"

	chatgptDefaultContext = 272_000
)

// chatgptBase is where the plan's requests go; a test points it at its own
// server.
var chatgptBase = chatgpt.APIBase

// chatgptEndpoints is the sign-in's endpoints; a test's are its own.
var chatgptEndpoints = chatgpt.Default

// ChatGPTStore is the credential on this machine.
func ChatGPTStore() *chatgpt.Store { return chatgpt.NewStore(shared.MemdoorHome()) }

// chatgptSource is the token source over the store: the access token,
// refreshed before it expires.
func chatgptSource() *chatgpt.Source {
	return chatgpt.NewSource(chatgptEndpoints(), ChatGPTStore(), &http.Client{Timeout: 30 * time.Second})
}

// chatgptProvider is the built-in entry: connected once a sign-in is kept.
func chatgptProvider() Provider {
	p := Provider{ID: ChatGPTID, Name: "ChatGPT plan (Sign in with ChatGPT)", API: APIChatGPT, Base: chatgptBase, Context: chatgptDefaultContext, BuiltIn: true}
	if c, err := ChatGPTStore().Load(); err == nil && c != nil && c.RefreshToken != "" {
		p.Key = c.AccessToken
		if p.Key == "" {
			p.Key = "signed-in"
		}
		p.KeySource = "Sign in with ChatGPT"
		if c.Email != "" {
			p.KeySource += " (" + c.Email + ")"
		}
	}
	return p
}

// ChatGPTSignedIn is who is signed in, "" when nobody.
func ChatGPTSignedIn() string {
	c, err := ChatGPTStore().Load()
	if err != nil || c == nil || c.RefreshToken == "" {
		return ""
	}
	if c.Email == "" {
		return "signed in"
	}
	return c.Email
}

// newChatGPTClient is the Responses client in plan mode: the bearer token
// from the source on every request, store:false, and none of the fields the
// preview refuses.
func newChatGPTClient(model string) LLMClient {
	src := chatgptSource()
	return &responsesClient{
		url:         responsesURL(chatgptBase),
		model:       model,
		httpClient:  oaiHTTPClient(),
		tokenSource: src.Token,
		plan:        true,
	}
}

// chatgptRefusal is a plan request's refusal in the person's terms
// (developers.openai.com/siwc/token-sharing-open-source/errors-and-recovery).
func chatgptRefusal(status int, code, message string) error {
	switch {
	case code == "subscription_sharing_usage_limit_exceeded":
		return fmt.Errorf("your ChatGPT plan's allowance for Memdoor is used up for now (ChatGPT → Settings → Usage shows it and the weekly cap per app); pin another provider's model with /model meanwhile")
	case code == "subscription_sharing_invalid_user":
		return fmt.Errorf("ChatGPT no longer accepts this sign-in (disconnected from the ChatGPT side): memdoor connect chatgpt signs in again")
	case code == "subscription_sharing_unsupported_capability":
		return fmt.Errorf("the ChatGPT plan does not allow this request (%s); /model picks another of its models", message)
	case code == "chatpass_v2_scope_not_authorized":
		return fmt.Errorf("the ChatGPT sign-in did not allow plan usage: memdoor connect chatgpt signs in again and asks for it")
	case status == http.StatusUnauthorized:
		return fmt.Errorf("the ChatGPT sign-in expired: memdoor connect chatgpt signs in again")
	}
	return fmt.Errorf("HTTP %d from ChatGPT: %s", status, strings.TrimSpace(code+" "+message))
}

// ---- the plan's model list ----------------------------------------------

// chatgptReader reads GET /v1/models with the plan's bearer:
// {"models":[{"slug","display_name","visibility"}]} per OpenAI's docs — the
// ones with visibility "list" are what the person may pick — and the plain
// OpenAI {"data":[{"id"}]} shape, should the endpoint answer that.
type chatgptReader struct{}

func (chatgptReader) ListModels(ctx context.Context, p Provider) ([]Model, error) {
	tok, err := chatgptSource().Token(ctx)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Models []struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Visibility  string `json:"visibility"`
		} `json:"models"`
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	h := map[string]string{"Authorization": "Bearer " + tok}
	for k, v := range AppHeaders(p.Base) {
		h[k] = v
	}
	if err := getJSON(ctx, modelsURL(p.Base), h, &raw); err != nil {
		return nil, err
	}
	var out []Model
	for _, m := range raw.Models {
		if m.Slug == "" || (m.Visibility != "" && m.Visibility != "list") {
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.Slug
		}
		out = append(out, Model{ID: m.Slug, Name: name, Tools: true})
	}
	for _, d := range raw.Data {
		if d.ID != "" {
			out = append(out, Model{ID: d.ID, Name: d.ID, Tools: true})
		}
	}
	return out, nil
}

func (r chatgptReader) GetModel(ctx context.Context, p Provider, id string) (Model, error) {
	return fromList(ctx, r, p, id)
}
