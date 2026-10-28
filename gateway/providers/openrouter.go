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

// keylessStallAfter is the same silence for a runtime connected with no key
// (Ollama, llama.cpp, janis): it sends nothing while it reads the prompt, and
// a CPU reads slowly. A GitHub runner's 4 vCPUs read a 9B's 5,655-token
// prompt at 11.8 tokens a second, eight minutes before the first token, and
// the 60 s stall dropped the request twice (2026-10-27).
var keylessStallAfter = 10 * time.Minute

// oaiHTTPClient bounds the wait for response headers; the body is bounded
// between bytes by idleTimeoutReader (oai_native.go).
func oaiHTTPClient() *http.Client { return stallBoundClient(oaiStallAfter, 20*time.Minute) }

// keylessHTTPClient is oaiHTTPClient for a keyless runtime: a slow first
// token, and a whole answer that may take longer than 20 minutes on a CPU
// (the task's or the turn's own clock still bounds it).
func keylessHTTPClient() *http.Client { return stallBoundClient(keylessStallAfter, time.Hour) }

func stallBoundClient(stall, total time.Duration) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = stall
	return &http.Client{Timeout: total, Transport: t}
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
