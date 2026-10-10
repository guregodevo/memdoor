package providers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// newDirectOpenRouterClient is the client for a gateway using the PERSON'S own
// OpenRouter key (byok.go): the base is OpenRouter itself and the routing
// policy travels with the request.
func newDirectOpenRouterClient(ctx context.Context, re *RemoteEngine, model string) LLMClient {
	return &oaiClient{
		apiKey:     re.APIKey,
		baseURL:    strings.TrimSuffix(strings.TrimRight(re.Endpoint, "/"), "/chat/completions"),
		httpClient: oaiHTTPClient(),
		model:      model,
		provider:   byokProviderPolicy(ctx, model),
		attribute:  true,
	}
}

// oaiStallAfter is how long an OpenRouter request may go without a byte —
// no headers, then no token and no keep-alive comment (OpenRouter sends one
// while a model thinks) — before the attempt is abandoned and asked again.
// The 20-minute client limit alone held a turn four minutes with nothing
// on screen but a clock, until the person gave up (live 2026-09-29).
var oaiStallAfter = 60 * time.Second

// oaiHTTPClient bounds the wait for response headers; the body is bounded
// between bytes by idleTimeoutReader (oai_native.go).
func oaiHTTPClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = oaiStallAfter
	return &http.Client{Timeout: 20 * time.Minute, Transport: t}
}

// retryAfter is how long a refusal asks the client to wait (the Retry-After
// header, seconds or an HTTP date), capped at a minute; fallback when the
// provider names none. Groq's free tier refuses with "try again in 12s" and
// a header to match; two retries at 2 s and 5 s went nowhere (2026-10-02).
func retryAfter(h http.Header, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return fallback
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil && secs >= 0 {
		return minDuration(time.Duration(secs*float64(time.Second)), time.Minute)
	}
	if at, err := http.ParseTime(v); err == nil {
		if d := time.Until(at); d > 0 {
			return minDuration(d, time.Minute)
		}
	}
	return fallback
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
