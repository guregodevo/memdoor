package tools

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parser parses browser DSL scripts
type Parser struct {
	lineNum int
}

// NewParser creates a new DSL parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a DSL script from a reader
func (p *Parser) Parse(r io.Reader) ([]Command, error) {
	scanner := bufio.NewScanner(r)
	commands := []Command{}
	p.lineNum = 0

	for scanner.Scan() {
		p.lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		cmd, err := p.parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", p.lineNum, err)
		}

		commands = append(commands, cmd)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return commands, nil
}

// parseLine parses a single line into a command
func (p *Parser) parseLine(line string) (Command, error) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmdType := parts[0]
	args := parts[1:]

	switch cmdType {
	case "nav", "navigate", "go":
		if len(args) == 0 {
			return nil, fmt.Errorf("nav requires URL argument")
		}
		return NavigateCommand{URL: args[0]}, nil

	case "wait", "sleep":
		if len(args) == 0 {
			return nil, fmt.Errorf("wait requires duration argument")
		}
		seconds, err := parseWaitDuration(args[0])
		if err != nil {
			return nil, err
		}
		return WaitCommand{Seconds: seconds}, nil

	case "exec", "js", "eval":
		if len(args) == 0 {
			return nil, fmt.Errorf("exec requires JavaScript code")
		}
		// Join remaining args back into script
		script := strings.Join(args, " ")
		return ExecuteCommand{Script: script}, nil

	case "reload", "refresh":
		return ReloadCommand{}, nil

	case "snap", "screenshot", "screen":
		if len(args) == 0 {
			return nil, fmt.Errorf("snap requires output path")
		}
		return ScreenshotCommand{Path: args[0]}, nil

	case "snapshot", "dom":
		return SnapshotCommand{}, nil

	case "click":
		if len(args) == 0 {
			return nil, fmt.Errorf("click requires selector")
		}
		// Join remaining args for selector (may contain spaces in quotes)
		selector := strings.Join(args, " ")
		selector = strings.Trim(selector, `"'`)
		return ClickCommand{Selector: selector}, nil

	case "type", "fill":
		if len(args) < 2 {
			return nil, fmt.Errorf("type requires selector and text")
		}
		// First arg is selector, rest is text
		selector := args[0]
		text := strings.Join(args[1:], " ")
		text = strings.Trim(text, `"'`)
		return TypeCommand{Selector: selector, Text: text}, nil

	case "press", "key":
		if len(args) == 0 {
			return nil, fmt.Errorf("press requires key name")
		}
		return PressCommand{Key: args[0]}, nil

	case "console", "logs":
		return ConsoleCommand{}, nil

	case "network", "requests":
		return NetworkCommand{}, nil

	case "click-text", "clicktext":
		if len(args) == 0 {
			return nil, fmt.Errorf("click-text requires text to search for")
		}
		// Join remaining args for text (may contain spaces)
		text := strings.Join(args, " ")
		text = strings.Trim(text, `"'`)
		return ClickTextCommand{Text: text}, nil

	case "hover":
		if len(args) == 0 {
			return nil, fmt.Errorf("hover requires selector")
		}
		selector := strings.Join(args, " ")
		selector = strings.Trim(selector, `"'`)
		return HoverCommand{Selector: selector}, nil

	case "send", "submit":
		return SendCommand{}, nil

	case "focus":
		if len(args) == 0 {
			return nil, fmt.Errorf("focus requires selector")
		}
		selector := strings.Join(args, " ")
		selector = strings.Trim(selector, `"'`)
		return FocusCommand{Selector: selector}, nil

	default:
		return nil, fmt.Errorf("unknown command: %s", cmdType)
	}
}

// parseWaitDuration parses wait duration (e.g., "1.5", "1.5s", "1500ms")
func parseWaitDuration(s string) (float64, error) {
	s = strings.TrimSpace(s)

	// Handle milliseconds
	if strings.HasSuffix(s, "ms") {
		ms, err := strconv.ParseFloat(strings.TrimSuffix(s, "ms"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return ms / 1000.0, nil
	}

	// Handle seconds (with or without 's' suffix)
	s = strings.TrimSuffix(s, "s")
	seconds, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	if seconds < 0 {
		return 0, fmt.Errorf("duration must be positive: %s", s)
	}

	return seconds, nil
}
