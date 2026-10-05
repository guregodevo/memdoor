# Browser Automation with Chrome DevTools MCP

Memdoor provides powerful browser automation capabilities using the official Chrome DevTools MCP (Model Context Protocol) server. This enables agents to interact with web applications for testing, verification, and automated workflows.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [CLI Usage](#cli-usage)
- [Agent Usage](#agent-usage)
- [DSL Mode](#dsl-mode)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)

## Overview

The browser automation tool provides:

- **Navigate**: Load web pages and wait for completion
- **Snapshot**: Get DOM structure with element UIDs (accessibility tree)
- **Screenshot**: Capture visual state of pages
- **Click**: Interact with elements by UID
- **Fill**: Enter text into form fields
- **Evaluate**: Execute JavaScript and get results
- **Console**: Retrieve console messages (errors, warnings, logs)
- **Network**: Monitor network requests (API calls, resources)

### Key Features

-  **Per-execution isolation**: Each browser instance uses unique profile (no conflicts)
-  **Process cleanup**: Automatic cleanup prevents orphaned Chrome processes
-  **DSL mode**: Simple command syntax for multi-step workflows
-  **Headless**: Runs without visible browser window
-  **Official Chrome DevTools**: Uses Chrome's native automation protocol

## Architecture

```
┌─────────────────┐
│   CLI/Agent     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ chrome_devtools │  Tool (tools/chrome_devtools_tool.go)
│      Tool       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  BrowserTool    │  Wrapper (tools/browser.go)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   MCP Client    │  Client (pkg/mcp/client.go)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ chrome-devtools │  NPM package (subprocess)
│   -mcp server   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Chrome Browser │  Headless Chrome
│   (headless)    │
└─────────────────┘
```

### Component Responsibilities

- **chrome_devtools Tool**: Agent-facing API, validates input, manages lifecycle
- **BrowserTool**: Wraps MCP client, provides high-level methods
- **MCP Client**: Manages subprocess, handles JSON-RPC communication
- **chrome-devtools-mcp**: Official Chrome DevTools MCP server (via npx)
- **Chrome Browser**: Actual browser instance (headless mode)

## CLI Usage

All browser automation uses the DSL (Domain-Specific Language) mode via `chrome run`:

```bash
./memdoor chrome run <<'EOF'
# Your DSL commands here
nav http://localhost:5173
wait 2
snap /tmp/page.png
EOF
```

See the [DSL Mode](#dsl-mode-recommended) section below for complete command reference and examples.

## DSL Mode (Recommended)

Simple command syntax for browser automation. Keeps the browser alive across all actions in a single session.

### Why DSL Mode?

**The Problem:**
Using individual CLI commands creates a new browser for each action, losing state between commands:

```bash
#  WRONG - Each command creates new browser
./memdoor chrome navigate http://localhost:5173
./memdoor chrome screenshot /tmp/test.png
# Result: Screenshot is blank (new browser at about:blank)
```

**The Solution:**
DSL mode runs multiple commands in one browser session:
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snap /tmp/test.png
EOF
# Result: Screenshot shows actual page content! 
```

### DSL Command Reference

`exec` runs a function in the page, the way chrome's `evaluate_script` wants
it. A bare expression (`exec document.body.innerText`) is wrapped as
`() => (…)`; a script the browser rejects as a syntax error (several
statements) is run again as `() => { … }`.

The browser is not a search engine: search engines answer a headless browser
with a bot challenge. Agents search with `web_search`
([WEB_SEARCH.md](WEB_SEARCH.md)).

```
nav <url>              Navigate to URL (aliases: navigate, go)
wait <duration>        Wait (e.g., 1.5, 2s, 1500ms)
exec <script>          Execute JavaScript: an expression or a function (aliases: js, eval)
reload                 Reload page (alias: refresh)
snap <path>            Take screenshot (aliases: screenshot, screen)
snapshot               Take DOM snapshot (alias: dom)
click <selector>       Click element by CSS selector
type <selector> <text> Type text (React-compatible)
press <key>            Press key (Enter, Escape, Tab, etc.)
click-text <text>      Find and click element by text content
hover <selector>       Trigger mouseenter event on element
send                   Focus textarea and press Enter to send message
focus <selector>       Focus an element
console                Get console messages (alias: logs)
network                Get network requests (alias: requests)
```

### DSL Examples

#### Navigate and Screenshot
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snap /tmp/login-page.png
EOF
```

#### Complete Login Flow
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
type input[type="email"] user@test.com
type input[type="password"] password123
click button[type="submit"]
wait 3
snap /tmp/logged-in.png
exec () => window.location.href
EOF
```

#### Multi-Step Form Submission
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173/registration
wait 2
snapshot
type input[name="name"] John Doe
type input[name="email"] john@example.com
type input[name="password"] password123
type input[name="confirm"] password123
click-text Register
wait 3
console
snap /tmp/registration-result.png
EOF
```

#### API Testing with Network Monitor
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
click-text Submit
wait 1
network
console
EOF
```

### DSL Best Practices

1. **Always navigate first** in a workflow
2. **Add wait commands** after navigation and state changes (1-3 seconds)
3. **Use snapshot** to inspect DOM structure when debugging
4. **Take screenshot at the end** for visual verification
5. **Check console/network** for errors or API calls
6. **Use click-text** for buttons when possible (simpler than selectors)

## Common Workflows

### Testing Login Flow

**Using DSL (recommended):**
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
type input[type="email"] test@example.com
type input[type="password"] password123
click button[type="submit"]
wait 3
snap /tmp/logged-in.png
exec () => window.location.href
console
EOF
```

**Using Agent:**
```bash
./memdoor agent --message "@writer Please test the login flow at http://localhost:5173 using DSL mode. Credentials: test@example.com / password123. Navigate, fill form, click login, and screenshot the result." --channel test --agent-id writer
```

### Automated UI Testing

```bash
./memdoor agent --message "@writer Use chrome DSL mode to test the registration form at http://localhost:5173/register. Fill all fields, submit, and verify success." --channel test --agent-id writer
```

### Debugging Page Issues

```bash
./memdoor agent --message "@writer Navigate to http://localhost:5173/dashboard using DSL mode, then check console messages and network requests for any errors." --channel test --agent-id writer
```

### Screenshot Comparison

```bash
# Take baseline screenshot
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snap baseline.png
EOF

# Make changes to code...

# Take new screenshot
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snap current.png
EOF

# Compare manually or with diff tool
```

## Troubleshooting

### Common Issues

#### 1. Blank Screenshots

**Symptom**: Screenshots are blank or show `about:blank`

**Cause**: Using single action mode - each action creates new browser

**Solution**: Use DSL mode to keep browser alive:
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snap /tmp/page.png
EOF
```

#### 2. Element Not Found

**Symptom**: `click` or `type` fails with "element not found"

**Cause**: Element doesn't exist, or page hasn't loaded yet

**Solution**: Add wait commands and verify selectors:
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snapshot
type input[type="email"] test@example.com
EOF
```

#### 3. Timeout Errors

**Symptom**: Actions fail with timeout

**Cause**: Default timeout is 30s per command

**Solution**:
- Check if page is actually loading (slow network?)
- Add explicit wait commands between actions
- Check Chrome DevTools logs for errors

#### 4. Process Leaks

**Symptom**: Chrome processes keep running after tool exits

**Cause**: This was fixed in the current version

**Verification**:
```bash
# Before test
ps aux | grep chrome | wc -l

# Run test
./memdoor chrome navigate http://example.com

# After test
ps aux | grep chrome | wc -l
# Should be same or +1 (no accumulation over multiple runs)
```

#### 5. Profile Lock Errors

**Symptom**: "The browser is already running" error

**Cause**: This was fixed in the current version (unique profiles per execution)

**Verification**: Each browser instance gets unique profile path like:
```
/tmp/chrome-devtools-mcp-profile-1234567890123456789
```

### Debug Logging

Check gateway logs:
```bash
./memdoor logs query --regex "chrome_devtools|browser|MCP" --limit 50 --since 5m
```

Check for errors:
```bash
./memdoor logs errors --limit 20 --since 5m
```

### Manual Testing

Test the browser tool directly:
```bash
# Should complete in ~5 seconds and produce valid PNG
./memdoor chrome run <<'EOF'
nav http://example.com
wait 2
snap /tmp/test.png
EOF
file /tmp/test.png  # Should say "PNG image data"
```

## Technical Details

### Browser Lifecycle

```
1. Tool call received
   ↓
2. Create BrowserTool instance (unique profile)
   ↓
3. Start MCP server subprocess
   ↓
4. Wait 2 seconds for initialization
   ↓
5. Execute action(s)
   ↓
6. Stop MCP server (graceful with timeout)
   ↓
7. Kill process group if timeout
   ↓
8. Clean up resources
```

### Timeouts

- **Single action**: 30 seconds
- **DSL mode**: 30 seconds per command
- **MCP server shutdown**: 2 seconds before force-kill

### Profile Management

Each browser instance gets a unique profile:
```
/tmp/chrome-devtools-mcp-profile-<nanosecond-timestamp>
```

This prevents:
- Profile lock conflicts
- State bleeding between executions
- Concurrent access issues

### Process Cleanup

The tool uses process groups to ensure complete cleanup:

```go
// Create process group
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true,
}

// Kill entire process group (parent + all children)
syscall.Kill(-pid, syscall.SIGKILL)
```

This kills:
- The `npx` process
- The `chrome-devtools-mcp` server
- All Chrome browser processes

## References

- [Chrome DevTools MCP Server](https://github.com/ChromeDevTools/chrome-devtools-mcp)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/)

## Support

For issues or questions:

1. Check [Troubleshooting](#troubleshooting) section
2. Review gateway logs: `./memdoor logs query --regex "chrome|browser" --limit 50`
3. Test with CLI first before using agents
4. File issue in the repository with logs and reproduction steps
