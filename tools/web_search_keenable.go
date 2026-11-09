package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// web_search on Keenable's search API (#168): an opt-in engine, no model in
// between. The results come back as the same numbered Sources list the
// OpenRouter answer ends with, so the TUI renders both alike.
//
// Without a key the request goes to the public endpoint, which requires the
// app's name in X-Keenable-Title (10 req/s, 1000/h per IP). A key goes to
// /v1/search in X-API-Key and lifts the limit.

const (
	// WebSearchKeenable is the engine name MEMDOOR_WEB_SEARCH selects.
	WebSearchKeenable = "keenable"

	keenablePathPublic = "/v1/search/public"
	keenablePathKeyed  = "/v1/search"
	keenableKeyHeader  = "X-API-Key"
	keenableSnippetMin = 180 // the API refuses a shorter snippet_max_length
)

type keenableResponse struct {
	Results []struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		Description string `json:"description"`
		Snippet     string `json:"snippet"`
	} `json:"results"`
	Warning string `json:"warning"`
}

func keenableSearch(be WebSearchBackend, query string, n int) (string, error) {
	path := keenablePathPublic
	if be.Key != "" {
		path = keenablePathKeyed
	}
	body, _ := json.Marshal(map[string]any{
		"query":              query,
		"max_results":        n,
		"snippet_max_length": max(webSearchSnippetMax, keenableSnippetMin),
	})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(be.Endpoint, "/")+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	for k, v := range be.Headers {
		req.Header.Set(k, v)
	}
	if be.Key != "" {
		req.Header.Set(keenableKeyHeader, be.Key)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: webSearchTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("the search did not answer: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	switch {
	case resp.StatusCode == http.StatusTooManyRequests && be.Key == "":
		return "", fmt.Errorf("Keenable's keyless limit is reached (1000 searches an hour per IP); KEENABLE_API_KEY lifts it: %s", truncateForMessage(string(raw), 300))
	case resp.StatusCode >= 300:
		return "", fmt.Errorf("the search answered HTTP %d: %s", resp.StatusCode, truncateForMessage(string(raw), 300))
	}
	var out keenableResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("the search answer could not be read: %s", truncateForMessage(string(raw), 300))
	}

	var b strings.Builder
	head := fmt.Sprintf("Search results for %q (Keenable).", query)
	if w := strings.TrimSpace(out.Warning); w != "" {
		head += " " + w
	}
	b.WriteString(head)
	b.WriteString("\n\nSources:")
	seen := map[string]bool{}
	for _, r := range out.Results {
		if r.URL == "" || seen[r.URL] {
			continue
		}
		seen[r.URL] = true
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = r.URL
		}
		fmt.Fprintf(&b, "\n[%d] %s — %s", len(seen), title, r.URL)
		excerpt := r.Snippet
		if strings.TrimSpace(excerpt) == "" {
			excerpt = r.Description
		}
		if s := strings.Join(strings.Fields(excerpt), " "); s != "" {
			b.WriteString("\n    " + truncateForMessage(s, webSearchSnippetMax))
		}
	}
	if len(seen) == 0 {
		return "", fmt.Errorf("the search found nothing for %q; rephrase the query", query)
	}
	return b.String(), nil
}
