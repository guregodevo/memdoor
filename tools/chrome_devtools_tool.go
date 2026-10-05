package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/pkg/shared"
	"path/filepath"
	"sync"
	"time"
)

// Chrome DevTools tool for browser automation using Chrome DevTools MCP
var ChromeDevToolsDefinition = ToolDefinition{
	Name: "chrome_devtools",
	Description: `Advanced browser automation using Chrome DevTools Protocol via MCP.

Supports BOTH single actions AND batched workflows:

SINGLE ACTION MODE (one action per call):
- navigate: Navigate to a URL and wait for page load
- snapshot: Get DOM snapshot with element UIDs (accessibility tree)
- screenshot: Capture page screenshot (saved to file)
- click: Click an element by UID (from snapshot)
- fill: Fill input field by UID
- evaluate: Execute JavaScript and get result
- console: Get console messages (errors, warnings, logs)
- network: Get network requests (API calls, resources)

BATCH MODE (multiple actions in sequence, single browser instance):
Use "actions" array to perform multiple operations in one call.
The browser stays alive across all actions, so you can navigate THEN screenshot.

Single action examples:
- Navigate only: {"action":"navigate","url":"http://localhost:5173"}
- Screenshot only: {"action":"screenshot","path":"./test.png"}

Batch workflow examples:
- Navigate and screenshot: {"actions":[{"action":"navigate","url":"http://localhost:5173"},{"action":"screenshot","path":"./login.png"}]}
- Full login flow: {"actions":[{"action":"navigate","url":"http://localhost:5173"},{"action":"snapshot"},{"action":"fill","uid":"123","value":"user@test.com"},{"action":"fill","uid":"124","value":"password"},{"action":"click","uid":"125"},{"action":"screenshot","path":"./logged-in.png"}]}

IMPORTANT: Use batch mode when you need to navigate to a page and then interact with it.
Single action mode creates a new browser for each call, so screenshots will be blank unless you use batch mode.`,
	InputSchema: ChromeDevToolsInputSchema,
	Function:    ChromeDevTools,
}

type ChromeDevToolsInput struct {
	// Single action mode
	Action string `json:"action,omitempty" jsonschema_description:"Single action: navigate, snapshot, screenshot, click, fill, evaluate, console, network. Mutually exclusive with actions array."`
	URL    string `json:"url,omitempty" jsonschema_description:"URL to navigate to (for navigate)"`
	UID    string `json:"uid,omitempty" jsonschema_description:"Element UID from snapshot (for click, fill)"`
	Value  string `json:"value,omitempty" jsonschema_description:"Value to fill (for fill action)"`
	Script string `json:"script,omitempty" jsonschema_description:"JavaScript function to evaluate (for evaluate)"`
	Path   string `json:"path,omitempty" jsonschema_description:"File path for screenshot (for screenshot)"`

	// Batch mode
	Actions []ActionStep `json:"actions,omitempty" jsonschema_description:"Array of actions to execute in sequence using same browser instance. Mutually exclusive with action field."`
}

type ActionStep struct {
	Action string `json:"action" jsonschema_description:"Action: navigate, snapshot, screenshot, click, fill, evaluate, console, network"`
	URL    string `json:"url,omitempty" jsonschema_description:"URL to navigate to (for navigate)"`
	UID    string `json:"uid,omitempty" jsonschema_description:"Element UID from snapshot (for click, fill)"`
	Value  string `json:"value,omitempty" jsonschema_description:"Value to fill (for fill action)"`
	Script string `json:"script,omitempty" jsonschema_description:"JavaScript function to evaluate (for evaluate)"`
	Path   string `json:"path,omitempty" jsonschema_description:"File path for screenshot (for screenshot)"`
}

var ChromeDevToolsInputSchema = GenerateSchema[ChromeDevToolsInput]()

// WithBrowser starts a fresh BrowserTool, runs fn against it, and
// guarantees Stop() runs even if fn panics or the parent context is
// cancelled. The browser stays alive for the entire duration of fn —
// callers can issue many Navigate/Evaluate calls against the same
// instance without spawning a new chrome-devtools-mcp subprocess each
// time.
//
// This is the single source of truth for browser lifecycle: the agent-facing
// ChromeDevTools tool calls WithBrowser instead of reimplementing
// Start/Stop/cleanup.
//
// parentCtx controls the overall lifetime; if it's cancelled the
// browser is shut down. fn receives a context already derived from
// parentCtx so per-action timeouts compose naturally.
// browserMu: one headless browser at a time. Each WithBrowser starts its
// own chrome-devtools-mcp process, but three sub-sessions searching at
// once broke the pipe to it ("failed to write request: broken pipe",
// 2026-09-14 14:32) — the processes share a Chrome profile lock.
var browserMu sync.Mutex

func WithBrowser(parentCtx context.Context, fn func(ctx context.Context, b *BrowserTool) error) error {
	browserMu.Lock()
	defer browserMu.Unlock()
	logger := slog.Default()
	browserTool, err := NewBrowserTool(logger)
	if err != nil {
		return fmt.Errorf("create browser: %w", err)
	}

	browserCtx, browserCancel := context.WithCancel(parentCtx)
	defer browserCancel()

	if err := browserTool.Start(browserCtx); err != nil {
		return fmt.Errorf("start browser: %w", err)
	}
	defer func() {
		if stopErr := browserTool.Stop(); stopErr != nil {
			logger.Warn("Browser stop failed", slog.String("error", stopErr.Error()))
		}
	}()

	// Give the MCP server a moment to initialize before fn issues calls.
	// Same fixed delay every other caller used; should eventually become
	// a real readiness probe on the MCP client.
	time.Sleep(2 * time.Second)

	return fn(browserCtx, browserTool)
}

func ChromeDevTools(input json.RawMessage) (string, error) {
	var chromeInput ChromeDevToolsInput
	if err := json.Unmarshal(input, &chromeInput); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate: either action OR actions, not both
	if chromeInput.Action == "" && len(chromeInput.Actions) == 0 {
		return "", fmt.Errorf("either 'action' or 'actions' is required")
	}
	if chromeInput.Action != "" && len(chromeInput.Actions) > 0 {
		return "", fmt.Errorf("cannot specify both 'action' and 'actions' - use one or the other")
	}

	// Batch mode gets longer per-call timeout since it runs multiple actions
	timeout := 30 * time.Second
	if len(chromeInput.Actions) > 0 {
		timeout = time.Duration(30+len(chromeInput.Actions)*10) * time.Second
	}

	var output string
	err := WithBrowser(context.Background(), func(browserCtx context.Context, browserTool *BrowserTool) error {
		ctx, cancel := context.WithTimeout(browserCtx, timeout)
		defer cancel()

		// Batch mode: execute multiple actions in sequence
		if len(chromeInput.Actions) > 0 {
			var results []string
			for i, step := range chromeInput.Actions {
				result, err := executeAction(ctx, browserTool, step.Action, step.URL, step.UID, step.Value, step.Script, step.Path)
				if err != nil {
					return fmt.Errorf("action %d (%s) failed: %w", i+1, step.Action, err)
				}
				results = append(results, fmt.Sprintf("Step %d (%s): %s", i+1, step.Action, result))
			}
			output = fmt.Sprintf("Batch completed successfully:\n%s", joinResults(results))
			return nil
		}

		// Single action mode
		result, err := executeAction(ctx, browserTool, chromeInput.Action, chromeInput.URL, chromeInput.UID, chromeInput.Value, chromeInput.Script, chromeInput.Path)
		if err != nil {
			return err
		}
		output = result
		return nil
	})
	if err != nil {
		return "", err
	}
	return output, nil
}

func executeAction(ctx context.Context, browserTool *BrowserTool, action, url, uid, value, script, path string) (string, error) {
	switch action {
	case "navigate":
		if url == "" {
			return "", fmt.Errorf("url is required for navigate action")
		}
		if err := browserTool.Navigate(ctx, url); err != nil {
			return "", fmt.Errorf("navigate failed: %w", err)
		}
		return fmt.Sprintf("Navigated to %s", url), nil

	case "snapshot":
		snapshot, err := browserTool.Snapshot(ctx)
		if err != nil {
			return "", fmt.Errorf("snapshot failed: %w", err)
		}
		return snapshot, nil

	case "screenshot":
		if path == "" {
			// Default to workspace screenshots directory
			workspace := shared.MemdoorHome("workspace")
			path = filepath.Join(workspace, "screenshots", fmt.Sprintf("screenshot-%d.png", time.Now().Unix()))
		}
		if err := browserTool.Screenshot(ctx, path); err != nil {
			return "", fmt.Errorf("screenshot failed: %w", err)
		}
		return fmt.Sprintf("Screenshot saved to %s", path), nil

	case "click":
		if uid == "" {
			return "", fmt.Errorf("uid is required for click action")
		}
		if err := browserTool.Click(ctx, uid); err != nil {
			return "", fmt.Errorf("click failed: %w", err)
		}
		return fmt.Sprintf("Clicked element %s", uid), nil

	case "fill":
		if uid == "" || value == "" {
			return "", fmt.Errorf("uid and value are required for fill action")
		}
		if err := browserTool.Fill(ctx, uid, value); err != nil {
			return "", fmt.Errorf("fill failed: %w", err)
		}
		return fmt.Sprintf("Filled element %s with value", uid), nil

	case "evaluate":
		if script == "" {
			return "", fmt.Errorf("script is required for evaluate action")
		}
		result, err := browserTool.Evaluate(ctx, script)
		if err != nil {
			return "", fmt.Errorf("evaluate failed: %w", err)
		}
		resultJSON, _ := json.Marshal(result)
		return string(resultJSON), nil

	case "console":
		messages, err := browserTool.ConsoleMessages(ctx)
		if err != nil {
			return "", fmt.Errorf("console failed: %w", err)
		}
		messagesJSON, _ := json.MarshalIndent(messages, "", "  ")
		return string(messagesJSON), nil

	case "network":
		requests, err := browserTool.NetworkRequests(ctx)
		if err != nil {
			return "", fmt.Errorf("network failed: %w", err)
		}
		requestsJSON, _ := json.MarshalIndent(requests, "", "  ")
		return string(requestsJSON), nil

	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func joinResults(results []string) string {
	output := ""
	for _, r := range results {
		output += "  " + r + "\n"
	}
	return output
}
