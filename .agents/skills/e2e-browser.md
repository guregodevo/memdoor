---
name: e2e-browser
description: End-to-end testing workflow for browser automation using Chrome DevTools batch mode
user-invocable: true
disable-model-invocation: false
metadata:
  openclaw:
    emoji: 🌐
    skillKey: e2e-browser
    always: false
    os:
      - darwin
      - linux
---

# E2E Browser Automation

**CRITICAL: ALWAYS use multi-step mode for tests. Single-action commands lose browser state.**

## DSL Syntax (Agent-Only)

The `chrome run` command uses a simple DSL that's much easier for agents to generate:

```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', 'abc123')
reload
wait 4
snap /tmp/screenshot.png
EOF
```

**Why DSL?**
- ✅ **80% less typing** - No JSON quotes, braces, commas to escape
- ✅ **Easier to generate** - Simple template concatenation
- ✅ **More readable** - Clear intent without ceremony
- ✅ **Same power** - All browser automation features available

## DSL Command Reference

```
nav <url>              Navigate to URL (aliases: navigate, go)
wait <duration>        Wait (e.g., 1.5, 2s, 1500ms)
exec <script>          Execute JavaScript (aliases: js, eval)
reload                 Reload page (alias: refresh)
snap <path>            Take screenshot (aliases: screenshot, screen)
snapshot               Take DOM snapshot (alias: dom)
click <selector>       Click element by CSS selector
type <selector> <text> Type text (React-compatible)
press <key>            Press key (Enter, Escape, Tab, etc.)
click-text <text>      Find and click element by text content (alias: clicktext)
hover <selector>       Trigger mouseenter event on element
send                   Focus textarea and press Enter to send message (alias: submit)
focus <selector>       Focus an element
console                Get console messages (alias: logs)
network                Get network requests (alias: requests)
```


## Testing Workflow

### 1. Document Feature
```markdown
**Feature:** [Name]
**Purpose:** [Why it exists]
**DoD:**
- [ ] Navigation works
- [ ] Form fields accept input
- [ ] Submit triggers expected action
- [ ] No console/network errors
- [ ] Screenshots match design
```

### 2. Setup Environment
```bash
make clean stop start
curl http://localhost:18789/health
./memdoor auth login-direct --email alice@localhost --password password123
```

### 3. Execute Test
```bash
#!/bin/bash
set -e
mkdir -p /tmp/test-screenshots

# Get auth token
AUTH_TOKEN=$(./memdoor auth login-direct --email alice@localhost --password password123 2>&1 | grep -oE "token: [a-zA-Z0-9._-]+" | sed 's/token: //')

# Test login flow with DSL
./memdoor chrome run <<EOF
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', '$AUTH_TOKEN')
reload
wait 4
snap /tmp/test-screenshots/01-logged-in.png
snapshot
console
network
EOF

echo "Screenshots saved to /tmp/test-screenshots/"
echo "Check snapshot output for element UIDs"
```

### 4. Verify Results
```bash
# Check screenshots
open /tmp/test-screenshots/*.png

# Check logs
./memdoor logs query --regex "chrome" --limit 20 --since 5m
./memdoor logs errors --since 5m
```

## Complete Example: Send Direct Message

This example demonstrates the full workflow from authentication to sending a DM using DSL:

```bash
#!/bin/bash
set -e
mkdir -p /tmp/dm-screenshots

# Get auth token via API
TOKEN=$(curl -s -X POST 'http://localhost:18789/api/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@localhost","password":"password123"}' | \
  grep -o '"token":"[^"]*"' | cut -d'"' -f4)

echo "Auth token obtained: ${TOKEN:0:20}..."

# Automate browser to send DM
./memdoor chrome run <<EOF
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', '$TOKEN')
reload
wait 4
snap /tmp/dm-screenshots/01-logged-in.png

click-text Direct Messages
wait 1.5
snap /tmp/dm-screenshots/02-dm-list.png

click-text Quentin
wait 2
snap /tmp/dm-screenshots/03-quentin-conversation.png

type textarea Hey Quentin! How are you doing today?
wait 0.8
snap /tmp/dm-screenshots/04-message-typed.png

send
wait 2
snap /tmp/dm-screenshots/05-message-sent.png
snapshot
EOF

echo "Screenshots saved to /tmp/dm-screenshots/"
echo "DM sent successfully!"
```

**Key techniques:**
- Auth token injection via localStorage (bypasses login form)
- `click-text` to find buttons by text content (no UIDs needed)
- `type` command handles React's native value setter automatically
- `send` command simulates Enter key press to send message
- Progressive screenshots at each step for debugging

## Building Blocks

### Login Flow (DSL - Simple!)

Using `chrome run` DSL syntax for maximum simplicity:

```bash
./memdoor chrome run <<'EOF'
# Login with email/password
nav http://localhost:5173
wait 2
type input[type="email"] alice@localhost
type input[type="password"] password123
wait 0.5
snap /tmp/01-filled.png
click button[type="submit"]
wait 5
snap /tmp/02-logged-in.png
EOF
```

The `type` command automatically handles React's native value setter! No need for complex JavaScript.

### Find and Click Elements by Text (DSL)

**NEW: Simple `click-text` command (recommended):**

```bash
./memdoor chrome run <<'EOF'
# Click button by text content - much simpler!
click-text Direct Messages
EOF
```

**Legacy: Using `exec` with arrow function:**

For more complex logic, use `exec` with arrow function:

```bash
./memdoor chrome run <<'EOF'
# Click button by text content
exec () => { const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('Direct Messages')); if(btn) { btn.click(); return 'clicked'; } return 'not found'; }
EOF
```

**Important**: The `exec` command expects JavaScript that will be evaluated by the browser. For multi-statement logic, wrap in an arrow function `() => { ... }`.

### Send Messages with DSL (Simplified!)

**NEW: Using `send` command:**

```bash
./memdoor chrome run <<'EOF'
# Type message and send automatically
type textarea "Hello! This is a test message."
send
EOF
```

The `send` command automatically focuses the textarea and presses Enter - no need for complex JavaScript!

**Legacy: Manual focus and Enter key:**

```bash
./memdoor chrome run <<'EOF'
type textarea "Hello!"
focus textarea
press Enter
EOF
```

### Hover to Reveal Elements (DSL)

**NEW: Using `hover` command:**

```bash
./memdoor chrome run <<'EOF'
# Hover over element to reveal hidden content
hover button.dm-button
wait 0.8
click svg.lucide-plus
EOF
```

The `hover` command dispatches `mouseenter` event - perfect for hover-only UI elements like icon buttons.

### Helper Functions (Advanced Pattern)

For complex workflows with repeated operations, define reusable helper functions in the browser's `window` object:

**Pattern:**
```bash
./memdoor chrome run <<'EOF'
# Define helper once at the start
exec () => {
  const clickBtn = (text) => {
    const btn = Array.from(document.querySelectorAll('button'))
      .find(b => b.textContent.includes(text));
    if(!btn) return null;
    const rect = btn.getBoundingClientRect();
    const x = rect.x + rect.width/2;
    const y = rect.y + rect.height/2;
    btn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: x, clientY: y }));
    btn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: x, clientY: y }));
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: x, clientY: y }));
    return btn;
  };
  window.clickBtn = clickBtn;
  return 'helper loaded';
}

# Then use it multiple times - much cleaner!
exec () => { return window.clickBtn('Members') ? 'clicked' : 'not found'; }
exec () => { return window.clickBtn('Add Buddy') ? 'clicked' : 'not found'; }
exec () => { return window.clickBtn('Submit') ? 'clicked' : 'not found'; }
EOF
```

**Benefits:**
- Reduces code duplication by ~80%
- Improves readability and maintainability
- Functions persist throughout the browser session
- Can define multiple helpers: `window.clickBtn()`, `window.fillInput()`, `window.submitForm()`, etc.

**Common Helper Functions:**

```javascript
// Button clicker with full mouse sequence
window.clickBtn = (text) => {
  const btn = Array.from(document.querySelectorAll('button'))
    .find(b => b.textContent.includes(text));
  if(!btn) return null;
  const rect = btn.getBoundingClientRect();
  const x = rect.x + rect.width/2;
  const y = rect.y + rect.height/2;
  btn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: x, clientY: y }));
  btn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: x, clientY: y }));
  btn.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: x, clientY: y }));
  return btn;
};

// React-compatible input filler
window.fillInput = (selector, value) => {
  const input = document.querySelector(selector);
  if(!input) return null;
  input.focus();
  const nativeSetter = Object.getOwnPropertyDescriptor(
    input.tagName === 'TEXTAREA'
      ? window.HTMLTextAreaElement.prototype
      : window.HTMLInputElement.prototype,
    'value'
  ).set;
  nativeSetter.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
  return input;
};

// Form submitter using closest()
window.submitForm = (buttonText) => {
  const btn = Array.from(document.querySelectorAll('button'))
    .find(b => b.textContent.includes(buttonText));
  if(!btn) return null;
  const form = btn.closest('form');
  if(!form) return null;
  form.requestSubmit();
  return form;
};
```


### Adding a Member to a Channel

This example demonstrates adding a buddy to a channel using helper functions for cleaner code:

```bash
#!/bin/bash
set -e
mkdir -p /tmp/add-member-test

# Get auth token via API
TOKEN=$(curl -s -X POST 'http://localhost:18789/api/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@localhost","password":"password123"}' | \
  grep -o '"token":"[^"]*"' | cut -d'"' -f4)

./memdoor chrome run <<EOF
# Login
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', '$TOKEN')
reload
wait 4
snap /tmp/add-member-test/01-logged-in.png

# Define helper function for consistent clicking
exec () => {
  const clickBtn = (text) => {
    const btn = Array.from(document.querySelectorAll('button'))
      .find(b => b.textContent.includes(text));
    if(!btn) return null;
    const rect = btn.getBoundingClientRect();
    const x = rect.x + rect.width/2;
    const y = rect.y + rect.height/2;
    btn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: x, clientY: y }));
    btn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: x, clientY: y }));
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: x, clientY: y }));
    return btn;
  };
  window.clickBtn = clickBtn;
  return 'helper loaded';
}
wait 0.5

# Open Members panel
exec () => { return window.clickBtn('Members') ? 'clicked' : 'not found'; }
wait 2
snap /tmp/add-member-test/02-panel-opened.png

# Click Add Buddy button
exec () => { return window.clickBtn('Add Buddy') ? 'clicked' : 'not found'; }
wait 2
snap /tmp/add-member-test/03-dialog-open.png

# Type buddy mention (React-compatible)
exec () => {
  const input = document.querySelector('input[placeholder*="writer"]');
  if(!input) return 'input not found';
  input.focus();
  const nativeSetter = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    'value'
  ).set;
  nativeSetter.call(input, '@writer');
  input.dispatchEvent(new Event('input', { bubbles: true }));
  return 'typed @writer';
}
wait 0.5
snap /tmp/add-member-test/04-typed.png

# Submit form using closest()
exec () => {
  const btn = Array.from(document.querySelectorAll('button'))
    .find(b => b.textContent.includes('Add Member'));
  if(!btn) return 'button not found';
  const form = btn.closest('form');
  if(!form) return 'form not found';
  form.requestSubmit();
  return 'submitted';
}
wait 3
snap /tmp/add-member-test/05-submitted.png
snapshot
EOF

echo "Member addition workflow complete!"
echo "Screenshots saved to /tmp/add-member-test/"
```

**Key techniques demonstrated:**
- Helper function pattern with `window.clickBtn()` reduces code duplication
- All clicks use full mouse event sequence for reliability
- React-compatible input filling with native value setter
- Form submission using `closest()` to find correct form
- Progressive screenshots verify each step

**Screenshot size verification:**
- 01: Baseline (~152KB)
- 02: Panel opens (size decreases ~148KB)
- 03: Dialog opens (size increases ~149KB)
- 04: Input filled (size increases ~150KB)
- 05: Submitted (size increases ~158KB - error message visible, permission check working)

### Creating a DM with a User

This example shows how to open the DM dropdown and select a user, dealing with hover-only elements using DSL:

```bash
#!/bin/bash
set -e
mkdir -p /tmp/dm-test

# Get auth token via API
TOKEN=$(curl -s -X POST 'http://localhost:18789/api/auth/login' \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@localhost","password":"password123"}' | \
  grep -o '"token":"[^"]*"' | cut -d'"' -f4)

./memdoor chrome run <<EOF
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', '$TOKEN')
reload
wait 4
snap /tmp/dm-test/01-logged-in.png

hover button.dm-button
wait 0.8
snap /tmp/dm-test/02-plus-visible.png

exec () => { const dmBtn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('Direct Messages')); if(dmBtn) { const svg = dmBtn.querySelector('svg.lucide-plus'); if(svg) { const rect = svg.getBoundingClientRect(); const x = rect.x + rect.width/2; const y = rect.y + rect.height/2; svg.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: x, clientY: y })); svg.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: x, clientY: y })); svg.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: x, clientY: y })); return 'clicked'; } return 'not found'; } return 'not found'; }
wait 2
snap /tmp/dm-test/03-dropdown-opened.png

click-text greg
wait 3
snap /tmp/dm-test/04-dm-opened.png

type textarea Hey! How are things?
wait 0.8
send
wait 2
snap /tmp/dm-test/05-message-sent.png
EOF

echo "DM created and message sent!"
```

**Key techniques:**
- `hover` command to show hover-only Plus icon
- `exec` with full mouse sequence for complex SVG clicking
- `click-text` to find user in dropdown
- `type` and `send` for messaging

## Common DSL Patterns

**Fill form inputs (React-compatible):**
```bash
type input[name="email"] test@example.com
type input[type="password"] mypassword
```

**Click buttons:**
```bash
click button[type="submit"]
click-text Login
```

**Hover then click (for hover-only elements):**
```bash
hover button.parent-element
wait 0.8
click svg.lucide-plus
```

**Send message:**
```bash
type textarea Your message here
send
```

**Set localStorage token:**
```bash
exec localStorage.setItem('auth_token', 'YOUR_TOKEN')
```

**Wait for state changes:**
```bash
wait 2
wait 1.5s
wait 500ms
```

**Check URL/state:**
```bash
exec () => ({ url: window.location.href, path: window.location.pathname })
```

**Complex JavaScript logic:**
```bash
exec () => { const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('Text')); if(btn) { btn.click(); return 'clicked'; } return 'not found'; }
```

## Common Issues

**Forms don't submit:** Use `exec` with custom JavaScript or `click-text` with form submission

**Timing issues:** Add explicit `wait` commands between actions (0.5s for state, 2s for navigation)

**Blank screenshots:** Always use `chrome run` DSL mode with multiple commands, never single-action mode

## Key Learnings from Debugging

### 1. Screenshot Size Analysis is Your Best Friend
When debugging browser automation, **always check screenshot file sizes** to verify state changes:
```bash
ls -lh /tmp/screenshots/
# 01-initial.png: 150KB (baseline)
# 02-after-action.png: 150KB (SAME SIZE = action failed!)
# 03-after-success.png: 162KB (+12KB = state changed!)
```

**What size changes mean:**
- Same size (±100 bytes) = UI didn't change, action likely failed
- Increase of 10-20KB = Dialog/modal opened, content added
- Decrease = Dialog closed, content removed
- Large jumps (>100KB) = Page navigation or major UI change

**Use this to debug without opening screenshots:**
- If typing should fill a field but screenshot size is unchanged → value was cleared (blur issue)
- If clicking should open dialog but size unchanged → click didn't work
- If submitting should close dialog but size unchanged → submit failed

### 2. DOM Snapshots Reveal Hidden State
When actions return success but nothing happens, **take a snapshot** to see the actual DOM state:
```bash
snapshot
```

**Critical things snapshots reveal:**
- Button disabled state: `button "Create" disableable disabled`
- Input values: `textbox "e.g., marketing-team" required` (no value = empty!)
- Form validation errors in the DOM
- Whether elements actually exist vs just not visible

**Example from our debugging:**
```
uid=1_121 button "Create" disableable disabled
```
This revealed the Create button was disabled, leading us to discover the blur() bug.

### 3. React Forms Need Special Handling

**The `.blur()` Bug We Fixed:**
React controlled components lose their values when you call `.blur()` after setting them. This is because:
1. React tracks internal state separately from DOM
2. Blur can trigger validation that resets the field
3. Some frameworks clear uncontrolled inputs on blur

**Solution:** Never call `.blur()` after setting input values in React forms.

**The Submit Button Challenge:**
Programmatic clicks (`.click()`, `dispatchEvent`) often don't trigger React's form submission because:
- React uses synthetic events that may not fire from programmatic clicks
- Form validation happens in React's event system
- Submit handlers expect the full event chain

**Workarounds to try:**
1. Find the form element and call `form.requestSubmit()` or `form.submit()`
2. Focus the input and press Enter (simulates real user action)
3. Use Chrome DevTools Protocol's Input.dispatchMouseEvent for true mouse simulation

### 4. Progressive Debugging Strategy

When automation fails, follow this systematic approach:

**Step 1: Verify Authentication**
```bash
exec () => { return !!localStorage.getItem('auth_token'); }
```
If false → auth token not set or page didn't reload

**Step 2: Check Element Exists**
```bash
exec () => { return !!document.querySelector('button'); }
```
If false → selector is wrong or page structure changed

**Step 3: Check Element State**
```bash
exec () => { const btn = document.querySelector('button'); return { disabled: btn.disabled, visible: btn.offsetParent !== null }; }
```
Reveals why clicking might not work

**Step 4: Verify Value Persistence**
After typing, check if the value stuck:
```bash
type input[type="text"] myvalue
wait 0.5
exec () => { return document.querySelector('input[type="text"]').value; }
```
If returns empty string → value was cleared (blur issue, validation, etc.)

**Step 5: Compare Screenshot Sizes**
As described above - this catches 90% of issues instantly

### 5. Working vs Broken Patterns

**✅ WORKS: Auth via localStorage**
```bash
exec () => { localStorage.setItem('auth_token', 'TOKEN'); return 'token set'; }
reload
```

**✅ WORKS: Hover to reveal elements**
```bash
hover button
wait 0.8
# Now hidden elements with group-hover are visible
```

**✅ WORKS: Type with native value setter (NO BLUR!)**
```bash
type input[type="text"] myvalue
# Value persists because we removed .blur() call
```

**✅ WORKS: Mouse events on SVG elements**
```javascript
const rect = svg.getBoundingClientRect();
const x = rect.x + rect.width/2;
const y = rect.y + rect.height/2;
svg.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: x, clientY: y }));
svg.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: x, clientY: y }));
svg.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: x, clientY: y }));
```

**❌ BROKEN: Calling .blur() after typing**
```javascript
input.value = 'text';
input.dispatchEvent(new Event('input', { bubbles: true }));
input.blur();  // ❌ This clears the value in React forms!
```

**❌ BROKEN: Simple .click() on React submit buttons**
```javascript
button.click();  // ❌ May not trigger React's form submission
```

**❌ BROKEN: Setting value without native setter**
```javascript
input.value = 'text';  // ❌ React won't detect this change
```

### 6. The Multiple Forms Problem - CRITICAL

**Problem**: `document.querySelector('form')` grabs the FIRST form, which might be the wrong one!

**Example**: Page with search bar form + dialog form:
- Form 0: Search bar (placeholder: "Search in #...")
- Form 1: Channel creation dialog (placeholder: "e.g., marketing-team")

Calling `querySelector('form')` returns the search bar, NOT your dialog!

**Solution**: Find the form via the submit button:
```javascript
exec () => {
  const submitBtn = Array.from(document.querySelectorAll('button'))
    .find(b => b.textContent.includes('Create'));
  if(submitBtn) {
    const form = submitBtn.closest('form');  // ✅ Gets the CORRECT form!
    if(form) {
      form.requestSubmit();
      return 'submitted';
    }
  }
  return 'form not found';
}
```

**Why `closest()` works:**
- Walks up the DOM tree from the button
- Finds the first parent `<form>` element
- Guarantees you get the form that contains that specific button

**Complete Working Example - Create Channel:**
```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', 'YOUR_TOKEN')
reload
wait 4

# Open dialog
click-text Channels
wait 1.5
exec () => { const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('Channels')); if(btn) { btn.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true })); return 'hovered'; } return 'not found'; }
wait 0.8
exec () => { const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('Channels')); if(btn) { const svg = btn.querySelector('svg.lucide-plus'); if(svg) { const rect = svg.getBoundingClientRect(); const x = rect.x + rect.width/2; const y = rect.y + rect.height/2; svg.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, clientX: x, clientY: y })); svg.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: x, clientY: y })); svg.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: x, clientY: y })); return 'clicked'; } return 'not found'; } return 'not found'; }
wait 2

# Type channel name (React-compatible!)
exec () => { const input = document.querySelector('input[placeholder*="marketing"]'); if(input) { input.focus(); const nativeSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set; nativeSetter.call(input, 'my-channel'); input.dispatchEvent(new Event('input', { bubbles: true })); return 'typed'; } return 'not found'; }
wait 0.5

# Submit via CORRECT form (using closest!)
exec () => { const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent.includes('Create')); if(btn) { const form = btn.closest('form'); if(form) { form.requestSubmit(); return 'submitted'; } return 'no form'; } return 'no button'; }
wait 3
EOF
```

### 7. The Scientific Method for Automation

**Our successful debugging process:**
1. **Observe:** "Create button returns 'clicked' but channel wasn't created"
2. **Hypothesize:** "Maybe the click didn't actually trigger submission"
3. **Test:** Take screenshot before/after, check file sizes (same size = no change)
4. **Analyze:** Take DOM snapshot, discover button is disabled
5. **New hypothesis:** "Input value must be missing"
6. **Test:** Check input in snapshot (no value shown)
7. **New hypothesis:** "blur() might be clearing it"
8. **Fix:** Remove blur() call
9. **Verify:** File size increases after typing (value persisted!)
10. **New problem:** Form submission still fails
11. **Investigate:** Check all forms on page → Found 2 forms!
12. **Root cause:** `querySelector('form')` grabbed search bar, not dialog
13. **Solution:** Use `submitBtn.closest('form')` to find correct form
14. **Success:** Channels created! 🎉

**This process works because:**
- Each test gives concrete data (file sizes, DOM state)
- We don't guess - we measure
- Failed tests narrow down the problem
- Success is verifiable (screenshot sizes, channel created)

## Best Practices

✅ **DO:**
- Use batch mode for multi-step workflows
- Take screenshots at each critical step
- Check console/network at end
- Document expected outcomes first
- Let agents execute complex tests

❌ **DON'T:**
- Use single-action mode for tests
- Hardcode UIDs (they change)
- Skip error checks
- Test without DoD
- Ignore flaky tests