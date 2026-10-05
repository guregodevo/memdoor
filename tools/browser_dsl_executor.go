package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Executor executes DSL commands using a BrowserTool
type Executor struct {
	browser *BrowserTool
}

// NewExecutor creates a new command executor
func NewExecutor(browser *BrowserTool) *Executor {
	return &Executor{browser: browser}
}

// Execute executes a single command
func (e *Executor) Execute(ctx context.Context, cmd Command) (interface{}, error) {
	switch c := cmd.(type) {
	case NavigateCommand:
		return nil, e.browser.Navigate(ctx, c.URL)

	case WaitCommand:
		duration := time.Duration(c.Seconds * float64(time.Second))
		time.Sleep(duration)
		return nil, nil

	case ExecuteCommand:
		return evaluateScript(ctx, e.browser.Evaluate, c.Script)

	case ReloadCommand:
		// Reload by executing JavaScript
		_, err := e.browser.Evaluate(ctx, "() => window.location.reload()")
		return nil, err

	case ScreenshotCommand:
		return nil, e.browser.Screenshot(ctx, c.Path)

	case SnapshotCommand:
		return e.browser.Snapshot(ctx)

	case ClickCommand:
		// Convert selector to JavaScript click with full mouse event sequence
		script := fmt.Sprintf(`() => {
			const el = document.querySelector(%q);
			if(el) {
				const rect = el.getBoundingClientRect();
				const x = rect.x + rect.width/2;
				const y = rect.y + rect.height/2;
				el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, clientX: x, clientY: y }));
				el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, clientX: x, clientY: y }));
				el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: x, clientY: y }));
				return 'clicked';
			}
			return 'not found';
		}`, c.Selector)
		return e.browser.Evaluate(ctx, script)

	case TypeCommand:
		// Convert to JavaScript for React-compatible input
		script := fmt.Sprintf(`() => {
			const el = document.querySelector(%q);
			if(!el) return 'not found';
			el.focus();
			if(el.tagName === 'TEXTAREA') {
				const nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
				nativeSetter.call(el, %q);
			} else {
				const nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
				nativeSetter.call(el, %q);
			}
			el.dispatchEvent(new Event('input', { bubbles: true }));
			return 'typed';
		}`, c.Selector, c.Text, c.Text)
		return e.browser.Evaluate(ctx, script)

	case PressCommand:
		// Convert key name to JavaScript key event
		keyCode := getKeyCode(c.Key)
		script := fmt.Sprintf(`() => {
			const el = document.activeElement;
			if(!el) return 'no active element';
			const event = new KeyboardEvent('keydown', {
				key: %q,
				code: %q,
				keyCode: %d,
				which: %d,
				bubbles: true,
				cancelable: true
			});
			el.dispatchEvent(event);
			return 'pressed';
		}`, c.Key, getKeyCodeName(c.Key), keyCode, keyCode)
		return e.browser.Evaluate(ctx, script)

	case ConsoleCommand:
		return e.browser.ConsoleMessages(ctx)

	case NetworkCommand:
		return e.browser.NetworkRequests(ctx)

	case ClickTextCommand:
		// Find and click element by text content with full mouse event sequence
		// Use a smarter strategy: find elements with matching text where the text is "exact" or "close enough"
		script := fmt.Sprintf(`() => {
			const searchText = %q;
			// First try buttons, links, and explicit clickable elements
			let elements = Array.from(document.querySelectorAll('button, a, [role="button"], [onclick]'));
			let el = elements.find(e => e.textContent.trim() === searchText || e.textContent.includes(searchText));

			// If not found, search in divs with cursor:pointer or specific roles
			if(!el) {
				const allDivs = Array.from(document.querySelectorAll('div'));
				// Filter to only divs that might be clickable (small text content, has cursor pointer, etc.)
				const clickableDivs = allDivs.filter(div => {
					const style = window.getComputedStyle(div);
					const textLen = div.textContent.trim().length;
					// Only consider divs with small text (likely buttons/links) or pointer cursor
					return (textLen < 100 && textLen > 0) || style.cursor === 'pointer';
				});
				el = clickableDivs.find(e => e.textContent.trim() === searchText || e.textContent.includes(searchText));
			}

			if(el) {
				const rect = el.getBoundingClientRect();
				const x = rect.x + rect.width/2;
				const y = rect.y + rect.height/2;
				el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, clientX: x, clientY: y }));
				el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, clientX: x, clientY: y }));
				el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: x, clientY: y }));
				return 'clicked';
			}
			return 'not found';
		}`, c.Text)
		return e.browser.Evaluate(ctx, script)

	case HoverCommand:
		// Trigger mouseenter event on selector
		script := fmt.Sprintf(`() => {
			const el = document.querySelector(%q);
			if(el) {
				el.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }));
				return 'hovered';
			}
			return 'not found';
		}`, c.Selector)
		return e.browser.Evaluate(ctx, script)

	case SendCommand:
		// Focus textarea and send message with Enter key
		script := `() => {
			const textarea = document.querySelector('textarea');
			if(textarea) {
				textarea.focus();
				const enterEvent = new KeyboardEvent('keydown', {
					key: 'Enter',
					code: 'Enter',
					keyCode: 13,
					which: 13,
					bubbles: true,
					cancelable: true
				});
				textarea.dispatchEvent(enterEvent);
				return 'sent';
			}
			return 'textarea not found';
		}`
		return e.browser.Evaluate(ctx, script)

	case FocusCommand:
		// Focus an element
		script := fmt.Sprintf(`() => {
			const el = document.querySelector(%q);
			if(el) {
				el.focus();
				return 'focused';
			}
			return 'not found';
		}`, c.Selector)
		return e.browser.Evaluate(ctx, script)

	default:
		return nil, fmt.Errorf("unknown command type: %T", cmd)
	}
}

// getKeyCode returns the key code for common keys
func getKeyCode(key string) int {
	switch key {
	case "Enter":
		return 13
	case "Escape":
		return 27
	case "Tab":
		return 9
	case "Backspace":
		return 8
	case "Delete":
		return 46
	default:
		return 0
	}
}

// getKeyCodeName returns the code name for common keys
func getKeyCodeName(key string) string {
	switch key {
	case "Enter":
		return "Enter"
	case "Escape":
		return "Escape"
	case "Tab":
		return "Tab"
	case "Backspace":
		return "Backspace"
	case "Delete":
		return "Delete"
	default:
		return key
	}
}

// jsFunctionHead is the start of a function: chrome's evaluate_script runs
// a function, not a bare expression.
var jsFunctionHead = regexp.MustCompile(`^(async\s+)?(function\b|\([^)]*\)\s*=>|[A-Za-z_$][\w$]*\s*=>)`)

// evaluateScript runs an exec script as the function evaluate_script wants.
// A bare expression ("document.body.innerText", which the chrome skill
// shows) failed with "fn is not a function" (live 2026-09-30); it is
// wrapped as () => (script), and when the browser rejects that as a syntax
// error (several statements) as () => { script }.
func evaluateScript(ctx context.Context, eval func(context.Context, string) (interface{}, error), script string) (interface{}, error) {
	script = strings.TrimSpace(script)
	if jsFunctionHead.MatchString(script) {
		return eval(ctx, script)
	}
	out, err := eval(ctx, "() => ("+script+")")
	if err != nil && strings.Contains(err.Error(), "SyntaxError") {
		return eval(ctx, "() => { "+script+" }")
	}
	return out, err
}
