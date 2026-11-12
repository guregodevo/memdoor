// Package gemini is the decision provider for Google's Decisions API
// (gemini-decision-1): one request carries the input and every question, and
// the answers come back under the caller's ids, scored in one pass with no
// generated text. It runs on the person's own GEMINI_API_KEY.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"memdoor/pkg/decision"
)

const (
	ProviderID   = "gemini"
	DefaultURL   = "https://generativelanguage.googleapis.com"
	DefaultModel = "gemini-decision-1"
	path         = "/v1beta/decisions"
	maxBody      = 4 << 20

	// RefusalHold is how long a client whose key was refused stays not-ready,
	// so a key with no access is not asked again on every turn.
	RefusalHold = 10 * time.Minute
)

// Config is what a Client needs. BaseURL is an origin; /v1beta/decisions is
// appended.
type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	Logger     Logger
}

// Logger is the gateway's logger, duck-typed so this package imports none.
type Logger interface {
	Warn(msg string, args ...any)
}

// Client implements decision.Provider over the Gemini Decisions API.
type Client struct {
	endpoint string
	key      string
	model    string
	httpc    *http.Client
	log      Logger

	mu        sync.Mutex
	refusedAt time.Time
}

// New fails fast on a missing key: this provider has no keyless mode.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("gemini decisions: no API key")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = DefaultURL
	}
	if !strings.HasPrefix(base, "https://") && !strings.HasPrefix(base, "http://") {
		return nil, fmt.Errorf("gemini decisions: invalid base URL %q", cfg.BaseURL)
	}
	httpc := cfg.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: decision.DefaultTimeout}
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	return &Client{endpoint: base + path, key: cfg.APIKey, model: model, httpc: httpc, log: cfg.Logger}, nil
}

func (c *Client) ID() string { return ProviderID }

// Endpoint is the full URL requests are posted to.
func (c *Client) Endpoint() string { return c.endpoint }

// Ready is network-free: a key, and not refused recently.
func (c *Client) Ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refusedAt.IsZero() || time.Since(c.refusedAt) > RefusalHold
}

func (c *Client) refused(why string) {
	c.mu.Lock()
	wasUp := c.refusedAt.IsZero() || time.Since(c.refusedAt) > RefusalHold
	c.refusedAt = time.Now()
	c.mu.Unlock()
	if wasUp && c.log != nil {
		c.log.Warn("decision model stood down: judged reads off until it is asked again",
			"endpoint", c.endpoint, "reason", why, "hold", RefusalHold.String())
	}
}

type wireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type wireRequest struct {
	Model     string                  `json:"model"`
	Input     string                  `json:"input"`
	Questions map[string]wireQuestion `json:"questions"`
}

type wireAnswer struct {
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
}

type wireResponse struct {
	Answers map[string]wireAnswer `json:"answers"`
}

func toWire(model string, req decision.Request) wireRequest {
	w := wireRequest{Model: model, Input: req.State, Questions: map[string]wireQuestion{}}
	for id, q := range req.Questions {
		wq := wireQuestion{Instructions: q.Instructions}
		switch q.Kind {
		case decision.KindBoolean:
			wq.Type = "noul"
			if q.TrueText != "" || q.FalseText != "" {
				wq.Criteria = map[string]string{"true_description": q.TrueText, "false_description": q.FalseText}
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
	out := make(map[string]decision.Answer, len(req.Questions))
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
			if wa.Choice == "" {
				return nil, fmt.Errorf("answer %q has no choice", id)
			}
			a.Choice = wa.Choice
			a.Probabilities = wa.Probabilities
		}
		out[id] = a
	}
	return out, nil
}

// Evaluate posts one request and classifies the HTTP outcome the way the
// decision service expects: 400/413/422 are the request's fault (a safety
// block is a 400), 429/503/529 are capacity, 401/402/403/404 stand the client
// down, a timeout is the deadline, anything else is a provider error.
func (c *Client) Evaluate(ctx context.Context, model string, req decision.Request) (decision.Result, error) {
	if model == "" {
		model = c.model
	}
	body, err := json.Marshal(toWire(model, req))
	if err != nil {
		return decision.Result{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return decision.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.key)
	resp, err := c.httpc.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return decision.Unavailable(decision.ReasonDeadline, err.Error()), nil
		}
		return decision.Result{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	switch code := resp.StatusCode; {
	case code == http.StatusBadRequest || code == http.StatusRequestEntityTooLarge || code == http.StatusUnprocessableEntity:
		return decision.Unavailable(decision.ReasonUnsupportedInput, httpDetail(code, raw)), nil
	case code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable || code == 529:
		return decision.Unavailable(decision.ReasonOverloaded, httpDetail(code, raw)), nil
	case code == http.StatusUnauthorized || code == http.StatusPaymentRequired || code == http.StatusForbidden || code == http.StatusNotFound:
		c.refused(fmt.Sprintf("HTTP %d", code))
		return decision.Unavailable(decision.ReasonNotConfigured, httpDetail(code, raw)), nil
	case code/100 != 2:
		return decision.Unavailable(decision.ReasonProviderError, httpDetail(code, raw)), nil
	}
	var wr wireResponse
	if err := json.Unmarshal(raw, &wr); err != nil {
		return decision.Result{}, fmt.Errorf("gemini decisions: bad response body: %w", err)
	}
	answers, err := fromWire(req, wr)
	if err != nil {
		return decision.Result{}, fmt.Errorf("gemini decisions: %w", err)
	}
	return decision.Result{Status: decision.StatusOK, Answers: answers, Provider: ProviderID, Model: model}, nil
}

func httpDetail(code int, raw []byte) string {
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return fmt.Sprintf("HTTP %d %s", code, msg)
}
