package tools

import (
	"strings"
	"testing"
)

func TestParser_ParseNavigate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantURL string
		wantErr bool
	}{
		{
			name:    "nav with URL",
			input:   "nav http://localhost:5173",
			wantURL: "http://localhost:5173",
		},
		{
			name:    "navigate with URL",
			input:   "navigate http://example.com",
			wantURL: "http://example.com",
		},
		{
			name:    "go with URL",
			input:   "go https://google.com",
			wantURL: "https://google.com",
		},
		{
			name:    "nav without URL",
			input:   "nav",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			commands, err := p.Parse(strings.NewReader(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(commands))
			}

			cmd, ok := commands[0].(NavigateCommand)
			if !ok {
				t.Fatalf("expected NavigateCommand, got %T", commands[0])
			}

			if cmd.URL != tt.wantURL {
				t.Errorf("expected URL %s, got %s", tt.wantURL, cmd.URL)
			}
		})
	}
}

func TestParser_ParseWait(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantSeconds float64
		wantErr     bool
	}{
		{
			name:        "wait with seconds",
			input:       "wait 1.5",
			wantSeconds: 1.5,
		},
		{
			name:        "wait with seconds suffix",
			input:       "wait 2s",
			wantSeconds: 2.0,
		},
		{
			name:        "wait with milliseconds",
			input:       "wait 1500ms",
			wantSeconds: 1.5,
		},
		{
			name:        "sleep alias",
			input:       "sleep 0.5",
			wantSeconds: 0.5,
		},
		{
			name:    "wait without duration",
			input:   "wait",
			wantErr: true,
		},
		{
			name:    "wait with invalid duration",
			input:   "wait abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			commands, err := p.Parse(strings.NewReader(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(commands))
			}

			cmd, ok := commands[0].(WaitCommand)
			if !ok {
				t.Fatalf("expected WaitCommand, got %T", commands[0])
			}

			if cmd.Seconds != tt.wantSeconds {
				t.Errorf("expected %f seconds, got %f", tt.wantSeconds, cmd.Seconds)
			}
		})
	}
}

func TestParser_ParseExecute(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantScript string
		wantErr    bool
	}{
		{
			name:       "exec with script",
			input:      "exec localStorage.setItem('token', '123')",
			wantScript: "localStorage.setItem('token', '123')",
		},
		{
			name:       "js alias",
			input:      "js () => document.title",
			wantScript: "() => document.title",
		},
		{
			name:       "eval alias",
			input:      "eval window.location.href",
			wantScript: "window.location.href",
		},
		{
			name:    "exec without script",
			input:   "exec",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			commands, err := p.Parse(strings.NewReader(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(commands))
			}

			cmd, ok := commands[0].(ExecuteCommand)
			if !ok {
				t.Fatalf("expected ExecuteCommand, got %T", commands[0])
			}

			if cmd.Script != tt.wantScript {
				t.Errorf("expected script %s, got %s", tt.wantScript, cmd.Script)
			}
		})
	}
}

func TestParser_ParseMultipleCommands(t *testing.T) {
	input := `
# Login test script
nav http://localhost:5173
wait 1.5s
exec localStorage.setItem('auth_token', 'abc123')
reload
wait 4
snap /tmp/screenshot.png
`

	p := NewParser()
	commands, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCount := 6
	if len(commands) != expectedCount {
		t.Fatalf("expected %d commands, got %d", expectedCount, len(commands))
	}

	// Verify command types
	if _, ok := commands[0].(NavigateCommand); !ok {
		t.Errorf("command 0 should be NavigateCommand, got %T", commands[0])
	}
	if _, ok := commands[1].(WaitCommand); !ok {
		t.Errorf("command 1 should be WaitCommand, got %T", commands[1])
	}
	if _, ok := commands[2].(ExecuteCommand); !ok {
		t.Errorf("command 2 should be ExecuteCommand, got %T", commands[2])
	}
	if _, ok := commands[3].(ReloadCommand); !ok {
		t.Errorf("command 3 should be ReloadCommand, got %T", commands[3])
	}
	if _, ok := commands[4].(WaitCommand); !ok {
		t.Errorf("command 4 should be WaitCommand, got %T", commands[4])
	}
	if _, ok := commands[5].(ScreenshotCommand); !ok {
		t.Errorf("command 5 should be ScreenshotCommand, got %T", commands[5])
	}
}

func TestParser_ParseClick(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantSelector string
		wantErr      bool
	}{
		{
			name:         "click with selector",
			input:        "click button.submit",
			wantSelector: "button.submit",
		},
		{
			name:         "click with quoted selector",
			input:        `click "button[type=submit]"`,
			wantSelector: "button[type=submit]",
		},
		{
			name:    "click without selector",
			input:   "click",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			commands, err := p.Parse(strings.NewReader(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(commands))
			}

			cmd, ok := commands[0].(ClickCommand)
			if !ok {
				t.Fatalf("expected ClickCommand, got %T", commands[0])
			}

			if cmd.Selector != tt.wantSelector {
				t.Errorf("expected selector %s, got %s", tt.wantSelector, cmd.Selector)
			}
		})
	}
}

func TestParser_ParseType(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantSelector string
		wantText     string
		wantErr      bool
	}{
		{
			name:         "type with selector and text",
			input:        "type textarea Hello world",
			wantSelector: "textarea",
			wantText:     "Hello world",
		},
		{
			name:         "type with quoted text",
			input:        `type input "test@example.com"`,
			wantSelector: "input",
			wantText:     "test@example.com",
		},
		{
			name:    "type without text",
			input:   "type textarea",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			commands, err := p.Parse(strings.NewReader(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(commands) != 1 {
				t.Fatalf("expected 1 command, got %d", len(commands))
			}

			cmd, ok := commands[0].(TypeCommand)
			if !ok {
				t.Fatalf("expected TypeCommand, got %T", commands[0])
			}

			if cmd.Selector != tt.wantSelector {
				t.Errorf("expected selector %s, got %s", tt.wantSelector, cmd.Selector)
			}

			if cmd.Text != tt.wantText {
				t.Errorf("expected text %s, got %s", tt.wantText, cmd.Text)
			}
		})
	}
}
