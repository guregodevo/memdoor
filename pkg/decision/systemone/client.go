// Package systemone is the decision provider for the System One HTTP API:
// TypeSafe's hosted Jev, or any compatible local server such as Kev. One
// request carries the state and every question; the answer map comes back
// under the caller's ids.
package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"memdoor/pkg/decision"

	"memdoor/pkg/attribution"
)

const (
	ProviderID   = "systemone"
	DefaultURL   = "https://api.typesafe.ai"
	DefaultModel = "jev-latest"
	path         = "/v1/systemone"
	maxBody      = 4 << 20

	// RefusalHold is how long a client whose endpoint refused it (no access
	// to the model, no credits, no such route) stays not-ready: if there is
	// a Jev model, use it, otherwise don't (Greg, 2026-10-03). A refused key
	// is not asked again on every turn.
	RefusalHold = 10 * time.Minute
)

// Config is what a Client needs. APIKey may be empty only for a local server
// (localhost / 127.0.0.1 / ::1), the way OpenClaw's plugin allows a Kev
// instance without credentials.
//
// BaseURL is an origin ("https://api.typesafe.ai", "http://127.0.0.1:8009"),
// to which /v1/systemone is appended — or a full endpoint when it carries a
// path, for gateways that serve the same body elsewhere: OpenRouter's is
// "https://openrouter.ai/api/alpha/decisions" (model "~typesafe/jev-latest").
type Config struct {
	BaseURL    string
	APIKey     string
	Model      string // default model when the reference names none
	HTTPClient *http.Client
}

// Client implements decision.Provider over the System One API.
type Client struct {
	base   string
	key    string
	model  string
	httpc  *http.Client
	local  bool
	origin string

	mu        sync.Mutex
	refusedAt time.Time
}

// New validates the config and returns the provider. Fail-fast on a base URL
// that is not http(s) or a remote URL without a key.
func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = DefaultURL
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("systemone: invalid base URL %q", cfg.BaseURL)
	}
	endpoint := base + path
	if u.Path != "" && u.Path != "/" {
		endpoint = base // a full endpoint was given; use it as is
	}
	host := u.Hostname()
	local := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if cfg.APIKey == "" && !local {
		return nil, fmt.Errorf("systemone: %s needs an API key", u.Host)
	}
	httpc := cfg.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: decision.DefaultTimeout}
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	return &Client{base: endpoint, key: cfg.APIKey, model: model, httpc: httpc, local: local, origin: u.Host}, nil
}

func (c *Client) ID() string { return ProviderID }

// Ready is network-free: configured, and not refused recently.
func (c *Client) Ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return (c.key != "" || c.local) && (c.refusedAt.IsZero() || time.Since(c.refusedAt) > RefusalHold)
}

func (c *Client) refused() {
	c.mu.Lock()
	c.refusedAt = time.Now()
	c.mu.Unlock()
}

// Origin is the host the client talks to, for status displays.
func (c *Client) Origin() string { return c.origin }

// Endpoint is the full URL requests are posted to.
func (c *Client) Endpoint() string { return c.base }

// Wire shapes, per docs.typesafe.ai/api.
type wireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type wireRequest struct {
	Model     string                  `json:"model"`
	State     string                  `json:"state"`
	Questions map[string]wireQuestion `json:"questions"`
}

type wireAnswer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
}

type wireResponse struct {
	Model   string                `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	Error   any                   `json:"error,omitempty"`
}

func toWire(model string, req decision.Request) wireRequest {
	w := wireRequest{Model: model, State: req.State, Questions: map[string]wireQuestion{}}
	for id, q := range req.Questions {
		wq := wireQuestion{Instructions: q.Instructions}
		switch q.Kind {
		case decision.KindBoolean:
			wq.Type = "noul"
			if q.TrueText != "" || q.FalseText != "" {
				wq.Criteria = map[string]string{"true": q.TrueText, "false": q.FalseText}
			}
		case decision.KindScore:
			wq.Type = "score"
			levels := make([]string, len(q.Options))
			for i, o := range q.Options {
				levels[i] = o.Description
			}
			wq.Criteria = levels
		default:
			wq.Type = "choice"
			crit := make(map[string]string, len(q.Options))
			for _, o := range q.Options {
				d := o.Description
				if d == "" {
					d = o.Label
				}
				crit[o.Label] = d
			}
			wq.Criteria = crit
		}
		w.Questions[id] = wq
	}
	return w
}

func fromWire(req decision.Request, resp wireResponse) (map[string]decision.Answer, error) {
	out := make(map[string]decision.Answer, len(resp.Answers))
	for id, q := range req.Questions {
		wa, ok := resp.Answers[id]
		if !ok {
			return nil, fmt.Errorf("no answer for %q", id)
		}
		a := decision.Answer{Kind: q.Kind, Confidence: wa.Confidence}
		switch q.Kind {
		case decision.KindBoolean:
			if wa.Noul == nil {
				return nil, fmt.Errorf("answer %q has no noul value", id)
			}
			a.ProbabilityTrue = *wa.Noul
			if a.Confidence == 0 {
				a.Confidence = max(a.ProbabilityTrue, 1-a.ProbabilityTrue)
			}
		case decision.KindScore:
			if wa.Score == nil {
				return nil, fmt.Errorf("answer %q has no score", id)
			}
			a.Score = *wa.Score
			a.ScoreProbabilities = make([]float64, len(q.Options))
			for i := range q.Options {
				a.ScoreProbabilities[i] = wa.Probabilities[strconv.Itoa(i)]
			}
		default:
			a.Choice = wa.Choice
			a.Probabilities = wa.Probabilities
			if a.Choice == "" {
				return nil, fmt.Errorf("answer %q has no choice", id)
			}
		}
		out[id] = a
	}
	return out, nil
}

// Evaluate posts one request and classifies the HTTP outcome the way the
// decision service expects: 413/422 are the request's fault, 429/503/529 are
// capacity, a timeout is the deadline, anything else is a provider error.
func (c *Client) Evaluate(ctx context.Context, model string, req decision.Request) (decision.Result, error) {
	if model == "" {
		model = c.model
	}
	body, err := json.Marshal(toWire(model, req))
	if err != nil {
		return decision.Result{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base, bytes.NewReader(body))
	if err != nil {
		return decision.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// The free tier's decisions run on the person's own OpenRouter key, and
	// until 2026-09-27 they counted toward no app at all — invisible in the
	// ranking that is our channel (pkg/attribution).
	attribution.SetIfOpenRouter(httpReq.Header, c.Endpoint())
	if c.key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.httpc.Do(httpReq)
	if err != nil {
		var ne net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			return decision.Unavailable(decision.ReasonDeadline, err.Error()), nil
		}
		return decision.Result{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	switch {
	case resp.StatusCode == http.StatusRequestEntityTooLarge || resp.StatusCode == http.StatusUnprocessableEntity:
		return decision.Unavailable(decision.ReasonUnsupportedInput, httpDetail(resp.StatusCode, raw)), nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == 529:
		return decision.Unavailable(decision.ReasonOverloaded, httpDetail(resp.StatusCode, raw)), nil
	case resp.StatusCode == http.StatusPaymentRequired || resp.StatusCode == http.StatusNotImplemented ||
		resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound:
		// No credits (402), a key with no access to the model (401/403), no
		// such route (404), or a broker with no decision key (501): nothing to
		// retry until someone sets it up, so the client stands down a while.
		c.refused()
		return decision.Unavailable(decision.ReasonNotConfigured, httpDetail(resp.StatusCode, raw)), nil
	case resp.StatusCode/100 != 2:
		return decision.Unavailable(decision.ReasonProviderError, httpDetail(resp.StatusCode, raw)), nil
	}
	var wr wireResponse
	if err := json.Unmarshal(raw, &wr); err != nil {
		return decision.Result{}, fmt.Errorf("systemone: bad response body: %w", err)
	}
	answers, err := fromWire(req, wr)
	if err != nil {
		return decision.Result{}, fmt.Errorf("systemone: %w", err)
	}
	return decision.Result{Status: decision.StatusOK, Answers: answers, Provider: ProviderID, Model: wr.Model}, nil
}

func httpDetail(code int, raw []byte) string {
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return fmt.Sprintf("HTTP %d %s", code, msg)
}

// Elapsed is exposed for tests of timeout classification.
var _ = time.Second
