package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
	"memdoor/gateway/logs"
)

const (
	DefaultWebFetchTimeout = 30 * time.Second
	// DefaultMaxResponseSize caps the raw HTML/text bytes read from the
	// remote server. Reduced from 100KB → 30KB on 2026-05-01 because the
	// improver was blowing past v4-pro's 128k context window on
	// multi-instrument runs (each fetch was returning ~50-80KB extracted
	// text = 15-25k tokens, accumulating to 100k+ across 5+ fetches).
	// That is bounded by DefaultMaxExtractedChars below (the OUTPUT cap) —
	// the right lever. The READ cap only needs to be large enough that the
	// article is actually inside the bytes we read: a real page leads with
	// tens of KB of <head> + nav chrome before any prose (a Wikipedia
	// article is ~370KB, its lead paragraph well past 30KB), so the old
	// 30KB read returned title + "Jump to content" and none of the body.
	// 3MB covers article pages comfortably; extraction still trims to
	// DefaultMaxExtractedChars, so output stays bounded.
	DefaultMaxResponseSize = 3 * 1024 * 1024 // 3MB
	// DefaultMaxExtractedChars caps the extracted-text returned to the
	// model after HTML stripping. 20,000 chars ≈ 5-7k tokens — well
	// under the per-tool-result soft budget. Keeps the model from
	// loading e.g. a Cornell LII full case opinion into context when
	// only the holding + key paragraphs are needed.
	// 20,000 was the first cap; three articles at that size plus three
	// transcripts filled a 64k window in one documentary turn
	// (2026-09-13 23:34). An article's gist fits in 12,000.
	DefaultMaxExtractedChars = 12000
	// A descriptive agent with a contact, as Wikimedia's policy asks: the
	// generic one got 429 from upload.wikimedia.org on every picture
	// (2026-09-14 14:35).
	DefaultUserAgent = "Memdoor/1.0 (https://memdoor.ai; hello@memdoor.ai)"
)

// web_fetch tool - Fetch content from a URL
var WebFetchDefinition = ToolDefinition{
	Name:        "web_fetch",
	Description: "Fetch content from a URL and extract readable text. Use this to read documentation, web pages, API responses, or any web content. Returns the main text content from the page.",
	InputSchema: WebFetchInputSchema,
	Function:    WebFetch,
}

type WebFetchInput struct {
	URL string `json:"url" jsonschema_description:"The HTTP or HTTPS URL to fetch content from."`
}

var WebFetchInputSchema = GenerateSchema[WebFetchInput]()

func WebFetch(input json.RawMessage) (string, error) {
	var params WebFetchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate URL
	url := strings.TrimSpace(params.URL)
	if url == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("URL must start with http:// or https://")
	}

	log := logs.New("Agent")
	log.Info("Fetching content from URL", slog.String("url", url))

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: DefaultWebFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects (Go's default)
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// Create request with user agent
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.WithError(err).Error("Failed to create request")
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("Failed to fetch URL")
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code. A site that refuses a plain client (403, 429,
	// 503 behind a bot wall) usually serves a real browser: read it
	// through the headless one instead of failing the turn (an HN best
	// story's article, 2026-09-13 23:27).
	if resp.StatusCode != http.StatusOK {
		log.Warn("HTTP request refused",
			slog.Int("status_code", resp.StatusCode),
			slog.String("status", resp.Status))
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			if text, err := browserPageText(url); err == nil && strings.TrimSpace(text) != "" {
				log.Info("WebFetch read the page through the browser", slog.Int("chars", len(text)))
				return capExtractedFor(text, url, injectedTurnTask(input)), nil
			} else if err != nil {
				log.Warn("browser fallback failed: " + err.Error())
			}
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, DefaultMaxResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		log.WithError(err).Error("Failed to read response body")
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Detect content type
	contentType := resp.Header.Get("Content-Type")
	log.Debug("Fetched content",
		slog.String("content_type", contentType),
		slog.Int("bytes", len(body)))

	// Extract text based on content type
	var text string
	if strings.Contains(contentType, "text/html") {
		// HTML content - extract text
		text, err = ExtractTextFromHTML(string(body))
		if err != nil {
			log.WithError(err).Error("Failed to extract HTML text")
			return "", fmt.Errorf("failed to extract text from HTML: %w", err)
		}
	} else {
		// Plain text or other - return as-is
		text = string(body)
	}

	// Trim and clean up
	text = strings.TrimSpace(text)
	if text == "" {
		log.Warn("Extracted text is empty")
		return "", fmt.Errorf("no readable text content found at URL")
	}

	return capExtractedFor(text, url, injectedTurnTask(input)), nil
}

// browserPageText reads a page the way a person would — the headless
// browser, the body's text — behind a seam for tests.
var browserPageText = func(url string) (string, error) {
	var text string
	err := WithBrowser(context.Background(), func(ctx context.Context, b *BrowserTool) error {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		if err := b.Navigate(ctx, url); err != nil {
			return err
		}
		res, err := b.Evaluate(ctx, `() => (document.body && document.body.innerText) || ''`)
		if err != nil {
			return err
		}
		text, _ = res.(string)
		return nil
	})
	return text, err
}

// capExtractedFor keeps a fetch's text under the per-call budget. A page
// longer than the budget, fetched in a turn whose task is known with a
// decision model answering, returns the sections of the WHOLE page that
// matter for the task instead of its first 12 000 characters — the part a
// task needs is often past the head (navigation, cookie text, intros).
func capExtractedFor(text, url, task string) string {
	if len(text) > DefaultMaxExtractedChars && strings.TrimSpace(task) != "" && decisionService() != nil {
		if out, err := judgedSections("web_fetch", url, text, task, 10); err == nil && !strings.HasPrefix(out, "UNJUDGED") {
			out = fmt.Sprintf("page text is %d chars; the sections that matter for this turn's task:\n", len(text)) + out
			if len(out) > DefaultMaxExtractedChars {
				out = out[:DefaultMaxExtractedChars] + "\n[…cut at the per-call budget]"
			}
			logs.New("Agent").Info("WebFetch judged extracted text", slog.Int("original_chars", len(text)), slog.Int("returned_chars", len(out)), slog.String("url", url))
			return out
		}
	}
	return capExtracted(text, url)
}

// capExtracted keeps a fetch's text under the per-call budget.
func capExtracted(text, url string) string {
	log := logs.New("Agent")
	// Cap returned text to keep tool output under the per-call token
	// budget. Without this cap a single Cornell LII case page can
	// return 50KB+ of text and the improver loop accumulates 100k+
	// tokens of fetch output across a multi-instrument question — see
	// the constant doc above for the failure history.
	originalLen := len(text)
	if originalLen > DefaultMaxExtractedChars {
		text = text[:DefaultMaxExtractedChars] + fmt.Sprintf("\n\n[truncated — extracted text was %d chars, returned first %d. Re-fetch a more specific URL if you need more.]", originalLen, DefaultMaxExtractedChars)
		log.Info("WebFetch truncated extracted text",
			slog.Int("original_chars", originalLen),
			slog.Int("returned_chars", DefaultMaxExtractedChars),
			slog.String("url", url))
	} else {
		log.Info("WebFetch succeeded", slog.Int("chars", len(text)))
	}
	return text
}

// ExtractTextFromHTML extracts readable text from HTML content
// This is a simple implementation - just extracts text from all nodes
func ExtractTextFromHTML(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var text strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			// Add text content
			t := strings.TrimSpace(n.Data)
			if t != "" {
				text.WriteString(t)
				text.WriteString(" ")
			}
		}

		// Skip non-content elements: script/style, plus the structural chrome
		// (site nav, header, footer, sidebars) that otherwise dominates an
		// extracted article — e.g. a Wikipedia fetch led with "Jump to content
		// Main menu … Random article" before any real prose. Dropping these
		// whole subtrees is a cheap readability win short of a full extractor.
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "header", "footer", "aside", "noscript", "form", "button":
				return
			}
		}

		// Recurse to children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}

		// Add newline after block elements
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "br":
				text.WriteString("\n")
			}
		}
	}

	extract(doc)
	return strings.TrimSpace(text.String()), nil
}
