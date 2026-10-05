package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// web_search: OpenRouter's web search server tool, on the person's own key.
//
// One request to OpenRouter with {"type": "openrouter:web_search"} among its
// tools: OpenRouter runs the search (any model — models without their own
// search get the engine below), the model answers from the results, and the
// sources come back as url_citation annotations. An answer without a single
// source is refused: that is the model answering from memory, not a search.

const (
	webSearchEngine     = "parallel" // the cheapest engine OpenRouter offers ($0.001-0.005 a search)
	webSearchDefaultMax = 5
	webSearchMaxResults = 10
	webSearchTimeout    = 60 * time.Second
	webSearchSnippetMax = 400
)

// WebSearchBackend is where a search goes: the chat-completions endpoint,
// the key, and the model that answers from the results. An empty key means
// there is none.
type WebSearchBackend struct {
	Endpoint, Key, Model string
}

var webSearchBackend = func() WebSearchBackend { return WebSearchBackend{} }

// SetWebSearchBackend wires the backend (the gateway's OpenRouter key).
func SetWebSearchBackend(fn func() WebSearchBackend) { webSearchBackend = fn }

var WebSearchDefinition = ToolDefinition{
	Name: "web_search",
	Description: "Search the web and get an answer with its numbered sources (title, URL, excerpt). " +
		"Use it to find pages, current facts or documentation you do not have; read a source in full with web_fetch.",
	InputSchema: WebSearchInputSchema,
	Function:    WebSearch,
}

type WebSearchInput struct {
	Query      string `json:"query" jsonschema_description:"What to search for, as you would type it into a search engine."`
	MaxResults int    `json:"max_results,omitempty" jsonschema_description:"How many results to search (default 5, max 10)."`
}

var WebSearchInputSchema = GenerateSchema[WebSearchInput]()

func WebSearch(input json.RawMessage) (string, error) {
	var in WebSearchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}
	max := in.MaxResults
	if max <= 0 {
		max = webSearchDefaultMax
	}
	max = min(max, webSearchMaxResults)
	be := webSearchBackend()
	if be.Key == "" {
		return "", fmt.Errorf("web search runs on an OpenRouter key and this gateway has none (OPEN_ROUTER_API_KEY)")
	}

	body, _ := json.Marshal(map[string]any{
		"model": be.Model,
		"messages": []map[string]string{{
			"role":    "user",
			"content": "Search the web for: " + query + "\nAnswer briefly from the results only. Do not list the sources: they are listed after your answer.",
		}},
		"tools": []map[string]any{{
			"type": "openrouter:web_search",
			// One search per call: each is billed, and the agent calls again
			// for another query (live: without max_uses one call ran two).
			"parameters": map[string]any{"engine": webSearchEngine, "max_results": max, "max_uses": 1},
		}},
	})
	req, err := http.NewRequest(http.MethodPost, be.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+be.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: webSearchTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("the search did not answer: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("the search answered HTTP %d: %s", resp.StatusCode, truncateForMessage(string(raw), 300))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content     string `json:"content"`
				Annotations []struct {
					Type        string `json:"type"`
					URLCitation struct {
						URL     string `json:"url"`
						Title   string `json:"title"`
						Content string `json:"content"`
					} `json:"url_citation"`
				} `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("the search answer could not be read: %s", truncateForMessage(string(raw), 300))
	}
	msg := out.Choices[0].Message

	var b strings.Builder
	b.WriteString(strings.TrimSpace(msg.Content))
	b.WriteString("\n\nSources:")
	seen := map[string]bool{}
	for _, a := range msg.Annotations {
		c := a.URLCitation
		if a.Type != "url_citation" || c.URL == "" || seen[c.URL] {
			continue
		}
		seen[c.URL] = true
		fmt.Fprintf(&b, "\n[%d] %s — %s", len(seen), strings.TrimSpace(c.Title), c.URL)
		if s := strings.Join(strings.Fields(c.Content), " "); s != "" {
			b.WriteString("\n    " + truncateForMessage(s, webSearchSnippetMax))
		}
	}
	if len(seen) == 0 {
		return "", fmt.Errorf("the answer came back without a single source, so it was not a search result (the model answered from memory); search again or rephrase the query")
	}
	return b.String(), nil
}
