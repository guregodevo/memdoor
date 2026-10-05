package tools

// Command represents a single browser automation command
type Command interface {
	// Type returns the command type (nav, wait, exec, etc.)
	Type() string
}

// NavigateCommand navigates to a URL
type NavigateCommand struct {
	URL string
}

func (c NavigateCommand) Type() string { return "nav" }

// WaitCommand waits for a duration
type WaitCommand struct {
	Seconds float64
}

func (c WaitCommand) Type() string { return "wait" }

// ExecuteCommand executes JavaScript
type ExecuteCommand struct {
	Script string
}

func (c ExecuteCommand) Type() string { return "exec" }

// ReloadCommand reloads the page
type ReloadCommand struct{}

func (c ReloadCommand) Type() string { return "reload" }

// ScreenshotCommand takes a screenshot
type ScreenshotCommand struct {
	Path string
}

func (c ScreenshotCommand) Type() string { return "snap" }

// SnapshotCommand takes a DOM snapshot
type SnapshotCommand struct{}

func (c SnapshotCommand) Type() string { return "snapshot" }

// ClickCommand clicks an element
type ClickCommand struct {
	Selector string
}

func (c ClickCommand) Type() string { return "click" }

// TypeCommand types text into an element
type TypeCommand struct {
	Selector string
	Text     string
}

func (c TypeCommand) Type() string { return "type" }

// PressCommand presses a key
type PressCommand struct {
	Key string
}

func (c PressCommand) Type() string { return "press" }

// ConsoleCommand retrieves console messages
type ConsoleCommand struct{}

func (c ConsoleCommand) Type() string { return "console" }

// NetworkCommand retrieves network requests
type NetworkCommand struct{}

func (c NetworkCommand) Type() string { return "network" }

// ClickTextCommand clicks an element by text content
type ClickTextCommand struct {
	Text string
}

func (c ClickTextCommand) Type() string { return "click-text" }

// HoverCommand triggers mouseenter on an element
type HoverCommand struct {
	Selector string
}

func (c HoverCommand) Type() string { return "hover" }

// SendCommand focuses textarea and sends message (press Enter)
type SendCommand struct{}

func (c SendCommand) Type() string { return "send" }

// FocusCommand focuses an element
type FocusCommand struct {
	Selector string
}

func (c FocusCommand) Type() string { return "focus" }
