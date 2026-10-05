package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"memdoor/pkg/mcp"
)

// BrowserTool provides browser automation capabilities via Chrome DevTools MCP
type BrowserTool struct {
	client mcp.Client
	logger *slog.Logger
	// pageID is the page every page-scoped call names. chrome-devtools-mcp
	// (2026-09) requires a pageId on evaluate_script, take_snapshot,
	// take_screenshot, click and fill — "Required at pageId" — and its
	// first page is 1. Refreshed from list_pages after each navigation.
	pageID int
}

// NewBrowserTool creates a new browser tool with a unique profile per execution
// This prevents browser profile conflicts when multiple instances run concurrently
func NewBrowserTool(logger *slog.Logger) (*BrowserTool, error) {
	// Generate unique profile path for this execution
	// Pattern: Unique profile per execution prevents profile lock conflicts
	// Use nanosecond timestamp to ensure uniqueness even within same process/goroutine
	profileDir := fmt.Sprintf("/tmp/chrome-devtools-mcp-profile-%d", time.Now().UnixNano())

	// Create MCP client for Chrome DevTools MCP server with unique profile
	// Note: chrome-devtools-mcp uses --user-data-dir (Chrome flag) not --profile-dir
	args := []string{
		"-y",
		"chrome-devtools-mcp@latest",
		"--headless",
		"--viewport", "1280x800",
		fmt.Sprintf("--user-data-dir=%s", profileDir),
	}
	client := mcp.NewClient("npx", args, logger)

	return &BrowserTool{
		client: client,
		logger: logger,
	}, nil
}

// Start starts the browser MCP server
func (b *BrowserTool) Start(ctx context.Context) error {
	return b.client.Start(ctx)
}

// Stop stops the browser MCP server
func (b *BrowserTool) Stop() error {
	return b.client.Stop()
}

// Navigate navigates to a URL
func (b *BrowserTool) Navigate(ctx context.Context, url string) error {
	b.logger.Info("Navigating to URL", slog.String("url", url))

	res, err := b.client.CallTool(ctx, "navigate_page", b.pageArgs(map[string]interface{}{
		"type": "url",
		"url":  url,
	}))
	if err == nil {
		err = mcpResultError(res)
	}
	if err != nil {
		return fmt.Errorf("failed to navigate: %w", err)
	}

	b.logger.Info("Navigation successful", slog.String("url", url))
	b.refreshPageID(ctx)
	return nil
}

// refreshPageID asks the server which page is selected; 1 when it cannot
// tell (the first page, which is the only one a fresh profile has).
func (b *BrowserTool) refreshPageID(ctx context.Context) {
	b.pageID = 1
	res, err := b.client.CallTool(ctx, "list_pages", map[string]interface{}{})
	if err != nil {
		return
	}
	var text string
	for _, c := range res.Content {
		text += c.Text + "\n"
	}
	// Lines look like "1: https://… [selected]"; take the selected one, else the first.
	first := 0
	for _, line := range strings.Split(text, "\n") {
		m := pageLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, _ := strconv.Atoi(m[1])
		if first == 0 {
			first = id
		}
		if strings.Contains(strings.ToLower(line), "selected") {
			b.pageID = id
			return
		}
	}
	if first > 0 {
		b.pageID = first
	}
}

// mcpResultError turns a tool result that IS an error into one. The
// server reports a bad call as result text ("MCP error -32602: … Required
// at pageId") with a nil transport error; "Navigation successful" was
// logged over exactly that for a page that never moved (2026-09-13).
func mcpResultError(res *mcp.ToolResult) error {
	if res == nil {
		return nil
	}
	for _, c := range res.Content {
		if t := strings.TrimSpace(c.Text); res.IsError || strings.HasPrefix(t, "MCP error") {
			return fmt.Errorf("%s", t)
		}
	}
	return nil
}

var pageLineRe = regexp.MustCompile(`^\s*(\d+)\s*[:.)]`)

// pageArgs is the page-scoped argument set: whatever the caller passes,
// plus the page.
func (b *BrowserTool) pageArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		args = map[string]interface{}{}
	}
	if b.pageID == 0 {
		b.pageID = 1
	}
	args["pageId"] = b.pageID
	return args
}

// Screenshot takes a screenshot and saves it to a file
func (b *BrowserTool) Screenshot(ctx context.Context, outputPath string) error {
	b.logger.Info("Taking screenshot", slog.String("output", outputPath))

	// Do NOT pass filePath: the MCP server restricts file writes to its
	// workspace roots (the OS temp dir by default), silently denies anything
	// else and reports the denial as error text in the result — which used to
	// be reported as "Screenshot saved" with no file on disk (2026-09-30).
	// Ask for the image data and write the file ourselves instead.
	result, err := b.client.CallTool(ctx, "take_screenshot", b.pageArgs(map[string]interface{}{
		"fullPage": true,
	}))
	if err != nil {
		return fmt.Errorf("failed to take screenshot: %w", err)
	}
	if err := mcpResultError(result); err != nil {
		return fmt.Errorf("screenshot failed: %s", err)
	}

	// The image is not always first — the server prepends a text line
	// ("Took a screenshot of the full current page.").
	for _, content := range result.Content {
		if content.Type != "image" || content.Data == "" {
			continue
		}
		if err := b.SaveScreenshotFromBase64(content.Data, outputPath); err != nil {
			return fmt.Errorf("failed to save screenshot: %w", err)
		}
		return nil
	}

	return fmt.Errorf("screenshot did not return image data")
}

// Snapshot takes a DOM snapshot (text-based accessibility tree)
func (b *BrowserTool) Snapshot(ctx context.Context) (string, error) {
	b.logger.Info("Taking DOM snapshot")

	result, err := b.client.CallTool(ctx, "take_snapshot", b.pageArgs(map[string]interface{}{
		"verbose": false,
	}))

	if err != nil {
		return "", fmt.Errorf("failed to take snapshot: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty snapshot result")
	}

	snapshot := result.Content[0].Text
	b.logger.Info("DOM snapshot retrieved", slog.Int("length", len(snapshot)))

	return snapshot, nil
}

// Click clicks an element by UID (from snapshot)
func (b *BrowserTool) Click(ctx context.Context, uid string) error {
	b.logger.Info("Clicking element", slog.String("uid", uid))

	_, err := b.client.CallTool(ctx, "click", b.pageArgs(map[string]interface{}{
		"uid": uid,
	}))

	if err != nil {
		return fmt.Errorf("failed to click: %w", err)
	}

	b.logger.Info("Click successful", slog.String("uid", uid))
	return nil
}

// Fill fills an input element with text
func (b *BrowserTool) Fill(ctx context.Context, uid string, value string) error {
	b.logger.Info("Filling input", slog.String("uid", uid), slog.String("value", value))

	_, err := b.client.CallTool(ctx, "fill", b.pageArgs(map[string]interface{}{
		"uid":   uid,
		"value": value,
	}))

	if err != nil {
		return fmt.Errorf("failed to fill input: %w", err)
	}

	b.logger.Info("Fill successful", slog.String("uid", uid))
	return nil
}

// Evaluate evaluates JavaScript in the browser and returns the value the
// script produced.
//
// chrome-devtools-mcp wraps the script result in a markdown-fenced text
// block that looks like:
//
//	Script ran on page and returned:
//	```json
//	<actual value>
//	```
//
// This helper unwraps the fence and json.Unmarshals the inner payload so
// callers get a typed Go value (map / slice / string / number / nil)
// instead of the raw display string. If unwrapping fails for any reason
// the raw text is returned as a string — preserving the previous fallback
// behavior for callers that just want a debug message ("clicked",
// "not found", etc).
func (b *BrowserTool) Evaluate(ctx context.Context, script string) (interface{}, error) {
	b.logger.Info("Evaluating JavaScript", slog.String("script", script))

	result, err := b.client.CallTool(ctx, "evaluate_script", b.pageArgs(map[string]interface{}{
		"function": script,
	}))

	if err != nil {
		return nil, fmt.Errorf("failed to evaluate script: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty evaluation result")
	}
	if err := mcpResultError(result); err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}

	raw := result.Content[0].Text
	payload := unwrapEvaluatePayload(raw)

	// Try to parse the unwrapped payload as JSON.
	var evalResult interface{}
	if err := json.Unmarshal([]byte(payload), &evalResult); err == nil {
		b.logger.Info("JavaScript evaluation successful")
		return evalResult, nil
	}

	// Fallback: return the raw text as a string. Some scripts return
	// human-readable strings ("clicked", "not found") that aren't JSON.
	return raw, nil
}

// unwrapEvaluatePayload strips the chrome-devtools-mcp display wrapper
// around an evaluate_script result. Returns the original input if the
// expected wrapper isn't present.
func unwrapEvaluatePayload(raw string) string {
	// The wrapper looks like:
	//   Script ran on page and returned:
	//   ```json
	//   <payload>
	//   ```
	// Some MCP server versions skip the language tag (```\n…\n```).
	// Find the first ``` and the last ``` and take what's between.
	start := strings.Index(raw, "```")
	if start < 0 {
		return raw
	}
	// Skip past the opening fence and an optional language hint line.
	rest := raw[start+3:]
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		rest = rest[nl+1:]
	}
	end := strings.LastIndex(rest, "```")
	if end < 0 {
		return raw
	}
	return strings.TrimSpace(rest[:end])
}

// ConsoleMessages retrieves console messages
func (b *BrowserTool) ConsoleMessages(ctx context.Context) ([]ConsoleMessage, error) {
	b.logger.Info("Retrieving console messages")

	result, err := b.client.CallTool(ctx, "list_console_messages", map[string]interface{}{})

	if err != nil {
		return nil, fmt.Errorf("failed to list console messages: %w", err)
	}

	if len(result.Content) == 0 {
		return []ConsoleMessage{}, nil
	}

	// Parse console messages
	var messages struct {
		Messages []ConsoleMessage `json:"messages"`
	}

	rawText := result.Content[0].Text
	if err := json.Unmarshal([]byte(rawText), &messages); err != nil {
		b.logger.Warn("Failed to parse console messages", slog.String("error", err.Error()), slog.String("raw_response", rawText))
		// Return empty list instead of error - console messages are informational
		return []ConsoleMessage{}, nil
	}

	b.logger.Info("Console messages retrieved", slog.Int("count", len(messages.Messages)))
	return messages.Messages, nil
}

// NetworkRequests retrieves network requests
func (b *BrowserTool) NetworkRequests(ctx context.Context) ([]NetworkRequest, error) {
	b.logger.Info("Retrieving network requests")

	result, err := b.client.CallTool(ctx, "list_network_requests", map[string]interface{}{})

	if err != nil {
		return nil, fmt.Errorf("failed to list network requests: %w", err)
	}

	if len(result.Content) == 0 {
		return []NetworkRequest{}, nil
	}

	// Parse network requests
	var requests struct {
		Requests []NetworkRequest `json:"requests"`
	}

	rawText := result.Content[0].Text
	if err := json.Unmarshal([]byte(rawText), &requests); err != nil {
		b.logger.Warn("Failed to parse network requests", slog.String("error", err.Error()), slog.String("raw_response", rawText))
		// Return empty list instead of error - network requests are informational
		return []NetworkRequest{}, nil
	}

	b.logger.Info("Network requests retrieved", slog.Int("count", len(requests.Requests)))
	return requests.Requests, nil
}

// SaveScreenshotFromBase64 saves a base64-encoded screenshot to a file
func (b *BrowserTool) SaveScreenshotFromBase64(base64Data string, outputPath string) error {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode screenshot: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write screenshot: %w", err)
	}

	return nil
}

// ConsoleMessage represents a console message
type ConsoleMessage struct {
	Type   string `json:"type"`   // log, warn, error, etc.
	Text   string `json:"text"`   // Message text
	Source string `json:"source"` // Source file
	Line   int    `json:"line"`   // Line number
	Column int    `json:"column"` // Column number
	MsgID  int    `json:"msgid"`  // Message ID
}

// NetworkRequest represents a network request
type NetworkRequest struct {
	ReqID        int    `json:"reqid"`
	URL          string `json:"url"`
	Method       string `json:"method"`
	Status       int    `json:"status"`
	ResourceType string `json:"resourceType"`
	MimeType     string `json:"mimeType"`
}
