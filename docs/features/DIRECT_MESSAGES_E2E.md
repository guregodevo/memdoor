# Direct Messages (DM) - E2E Testing Documentation

## Feature Overview

### Purpose
Enable private one-on-one conversations between users (humans or agents) in Memdoor, providing a focused communication channel separate from public/private group channels.

### User Stories

**As a user**, I want to start a direct message with another user, so that I can have private conversations.

**As a user**, I want to start a direct message with an agent, so that I can get personalized assistance without involving others.

**As a user**, I want to see my DM conversations in the sidebar, so that I can quickly access ongoing private conversations.

### Scope

**Included:**
- Create DM with human users
- Create DM with agent users
- Send messages in DM
- Receive messages in DM
- Navigate to existing DMs
- Visual indicators for unread messages in DMs
- Real-time message delivery via WebSocket
- Presence indicators (online/offline/busy)

**Excluded:**
- Group DMs (more than 2 participants)
- DM-specific settings (mute, archive)
- DM search/filtering beyond standard search
- File sharing in DMs (uses standard message functionality)

## Definition of Done (DoD)

### Acceptance Criteria

#### Functional Requirements
- [ ] User can click "+" button in DMs section of sidebar
- [ ] User sees dropdown list of available users (humans and agents)
- [ ] User can select a user from the dropdown to create DM
- [ ] New DM appears in sidebar DMs section with user's name and avatar
- [ ] User can click on DM in sidebar to switch to that conversation
- [ ] User can send messages in DM using message input
- [ ] Messages appear in DM conversation in real-time
- [ ] Other participant receives messages via WebSocket
- [ ] User can see typing indicators (if implemented)
- [ ] User can see presence status (online/offline/busy)
- [ ] Unread message count appears in sidebar for DM
- [ ] DM marked as read when user views messages

#### Technical Requirements
- [ ] DM channel created with format `dm-{userA}-{userB}`
- [ ] DM channel type set to `"dm"` in database
- [ ] Only 2 participants allowed in DM channel
- [ ] Messages sent to correct DM channel ID
- [ ] WebSocket broadcasts messages to both participants
- [ ] API endpoint `/api/channels` returns DM channels
- [ ] API endpoint `/api/channels/{id}/messages` works for DMs
- [ ] No console errors during DM operations
- [ ] No network errors (4xx/5xx) during DM operations

#### UI/UX Requirements
- [ ] DM section expandable/collapsible in sidebar
- [ ] DMs sorted by most recent activity
- [ ] DM shows other user's name (not full channel name)
- [ ] DM shows other user's avatar (emoji or icon)
- [ ] Current DM highlighted in sidebar
- [ ] Presence indicator appears next to user's avatar
- [ ] Unread count badge appears when messages unread
- [ ] DM conversation header shows other user's name

### Edge Cases
- [ ] Cannot create DM with self
- [ ] Creating DM with existing DM partner reuses existing DM
- [ ] Deleted users show as "Unknown User" in DMs
- [ ] Offline users still receive messages when they reconnect
- [ ] Long messages wrap correctly in DM view
- [ ] Emoji and mentions work in DMs
- [ ] Thread replies work in DMs

## Happy Path Scenarios

### Scenario 1: Create DM with Human User

**Preconditions:**
- Memdoor web app running at http://localhost:5173
- User logged in as `test@example.com`
- Another user exists: `alice@localhost`
- No existing DM between test user and Alice

**Steps:**
1. Navigate to http://localhost:5173
2. Verify logged in (see user avatar in header)
3. Locate "Direct Messages" section in sidebar
4. Click "+" button next to "Direct Messages"
5. Dropdown appears with list of available users
6. Click on "Alice" in dropdown
7. DM created and appears in sidebar
8. Conversation view switches to DM with Alice
9. Header shows "Alice" (not channel ID)
10. Message input is active and ready

**Expected Outcome:**
- **Final State**: DM with Alice open and active
- **Sidebar**: DM appears under "Direct Messages" section
- **Header**: Shows "Alice" as conversation name
- **Console**: No errors
- **Network**: POST to `/api/channels` successful (201 or existing DM returned)
- **Screenshot**: Shows DM created with Alice's name and avatar

---

### Scenario 2: Send Message in DM

**Preconditions:**
- DM with Alice already exists and is open
- User logged in as `test@example.com`
- WebSocket connected (green indicator)

**Steps:**
1. DM with Alice is open (from Scenario 1)
2. Click in message input field
3. Type message: "Hey Alice, can we discuss the project?"
4. Press Enter or click Send button
5. Message appears in conversation
6. Message shows timestamp and user's avatar
7. WebSocket broadcasts message to Alice
8. No console errors

**Expected Outcome:**
- **Message Visible**: Message appears in DM conversation immediately
- **Message Content**: Text matches what was typed
- **Message Metadata**: Shows correct timestamp, author avatar
- **Console**: No errors
- **Network**: POST to `/api/messages` successful (200)
- **WebSocket**: Message broadcast event received (visible in network tab)
- **Screenshot**: Shows message sent successfully

---

### Scenario 3: Create DM with Agent

**Preconditions:**
- Memdoor web app running at http://localhost:5173
- User logged in as `test@example.com`
- Agent exists: `writer` (agent:writer)
- No existing DM between test user and writer agent

**Steps:**
1. Navigate to http://localhost:5173 (or already logged in)
2. Locate "Direct Messages" section in sidebar
3. Click "+" button next to "Direct Messages"
4. Dropdown appears with list of available users (includes agents)
5. Click on "writer" agent in dropdown
6. DM created and appears in sidebar
7. Conversation view switches to DM with writer
8. Header shows "writer" (agent name)
9. Message input is active
10. Type message: "@writer Please help me write a README"
11. Press Enter to send
12. Message appears and agent starts processing (typing indicator)
13. Agent responds in DM

**Expected Outcome:**
- **DM Created**: DM with writer agent appears in sidebar
- **Agent Execution**: Agent receives mention and starts processing
- **Agent Response**: Agent replies in DM (not public channel)
- **Console**: No errors
- **Network**: All requests successful
- **Screenshot**: Shows DM with agent and agent's response

---

### Scenario 4: Navigate to Existing DM

**Preconditions:**
- Multiple DMs exist (with Alice, with writer agent)
- User currently viewing different channel or DM
- Unread messages exist in DM with Alice

**Steps:**
1. User viewing "#general" channel or different DM
2. Locate DM with Alice in sidebar
3. Notice unread count badge (e.g., "2")
4. Click on DM with Alice in sidebar
5. Conversation switches to DM with Alice
6. Unread messages visible and scrolled into view
7. Unread badge clears after messages viewed
8. Header shows "Alice"

**Expected Outcome:**
- **Channel Switch**: View changes to DM with Alice
- **Unread Messages**: Previous unread messages visible
- **Read Status**: Messages marked as read after viewing
- **Badge Cleared**: Unread count badge disappears
- **Console**: No errors
- **Screenshot**: Shows DM with unread messages

## Test Data Setup

### Required Users
```bash
# Create test users via CLI (if not exists)
./memdoor setup  # Creates default users including:
# - admin@localhost (password: password123)
# - alice@localhost (password: password123)
# - quentin@localhost (password: password123)

# Or create custom test user
./memdoor user create --email test@example.com --password password123 --name "Test User"
```

### Required Agents
```bash
# List existing agents
./memdoor agents list

# Should show:
# - writer (agent:writer)
# - coder (agent:coder)

# If needed, create agent via web UI:
# 1. Click "+" button in Agents section
# 2. Fill in agent details
# 3. Click Create
```

### Clean State (Optional)
```bash
# To start with clean DM state, remove existing DMs via database
# WARNING: This deletes all DMs for the user
sqlite3 ~/.memdoor/data/memdoor.db "DELETE FROM channel_members WHERE channel_id IN (SELECT id FROM channels WHERE type='dm' AND name LIKE '%test@example.com%');"
sqlite3 ~/.memdoor/data/memdoor.db "DELETE FROM channels WHERE type='dm' AND name LIKE '%test@example.com%';"
```

## Browser Automation Test Scenarios

### Test 1: Create DM with Human (Automated)

**Using Chrome DevTools DSL Mode:**

```bash
# Get auth token first
TOKEN=$(./memdoor auth login-direct --email alice@localhost --password password123 2>&1 | grep -oE "token: [a-zA-Z0-9._-]+" | sed 's/token: //')

# Run automated DM creation
./memdoor chrome run <<EOF
nav http://localhost:5173
wait 1.5
exec localStorage.setItem('auth_token', '$TOKEN')
reload
wait 4
snap /tmp/dm-test/01-homepage.png

hover button.dm-button
wait 0.8
exec () => { const btn = document.querySelector('button.dm-button svg.lucide-plus'); if(btn) { btn.click(); return 'clicked'; } return 'not found'; }
wait 2
snap /tmp/dm-test/02-dm-dropdown.png

click-text Alice
wait 3
snap /tmp/dm-test/03-dm-created.png
console
network
EOF
```

**Note**: See `.agents/skills/e2e-browser.md` for complete DSL patterns and examples.

### Test 2: Send Message in DM (Automated)

```bash
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
click-text Alice
wait 2
snap /tmp/dm-test/04-dm-open.png

type textarea Test message from automation
wait 0.8
send
wait 2
snap /tmp/dm-test/05-message-sent.png
console
network
EOF
```

### Test 3: Agent-Driven Testing (Recommended)

**Let the writer agent execute the test:**

```bash
./memdoor agent --message "@writer Please execute an E2E browser test for Direct Messages:

1. Navigate to http://localhost:5173 and login
2. Get DOM snapshot to find element UIDs
3. Click the '+' button in Direct Messages section
4. Select 'alice' from the dropdown to create DM
5. Verify DM appears in sidebar with name 'Alice'
6. Click on the DM to open it
7. Send a test message: 'Hello from automated test'
8. Verify message appears in conversation
9. Check console for any errors
10. Check network requests - all should be successful

Save screenshots at each step to /tmp/dm-e2e-test/
Report any errors found with screenshots and console output." --channel test --agent-id writer
```

**Verify agent's work:**
```bash
# Check agent's test results
./memdoor messages --channel test --limit 10

# Review screenshots
ls -la /tmp/dm-e2e-test/
open /tmp/dm-e2e-test/*.png  # macOS
# or: xdg-open /tmp/dm-e2e-test/*.png  # Linux

# Check for any errors
./memdoor logs query --regex "chrome|browser|dm|direct.?message" --limit 50 --since 10m
./memdoor logs errors --since 10m
```

## Verification Checklist

After running E2E test, verify:

###  Functional Verification
- [ ] DM created successfully (appears in sidebar)
- [ ] DM shows correct user name (not channel ID)
- [ ] DM shows correct user avatar
- [ ] Message sent successfully (appears in conversation)
- [ ] Message shows correct content
- [ ] Message shows correct timestamp
- [ ] WebSocket connected throughout test
- [ ] No console errors
- [ ] No network errors (all requests 2xx)

###  Visual Verification
```bash
# Compare screenshots
open /tmp/dm-test/01-homepage.png        # Initial state
open /tmp/dm-test/02-dm-dropdown.png     # Dropdown open
open /tmp/dm-test/03-dm-created.png      # DM created
open /tmp/dm-test/04-dm-open.png         # DM conversation open
open /tmp/dm-test/05-message-sent.png    # Message sent

# Verify:
# - Dropdown shows available users
# - DM appears in sidebar after creation
# - DM name matches selected user
# - Message appears in conversation
# - No error dialogs/toasts visible
```

###  Technical Verification
```bash
# Check logs for DM creation
./memdoor logs query --regex "dm-|direct.?message|channel.*type.*dm" --limit 30 --since 10m

# Verify no errors
./memdoor logs errors --since 10m

# Check database (optional)
sqlite3 ~/.memdoor/data/memdoor.db "SELECT id, name, type FROM channels WHERE type='dm' ORDER BY created_at DESC LIMIT 5;"
```

## Common Issues & Troubleshooting

### Issue: DM Dropdown Doesn't Open

**Symptom**: Clicking "+" button doesn't show dropdown

**Debugging:**
```bash
# Check DOM structure
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snapshot
EOF

# Search snapshot for "Direct Messages" section
# Look for button with "Plus" icon or hover states
# Update selectors in test script
```

**Common Causes:**
- Incorrect UID (page structure changed)
- User not logged in (auth token missing)
- JavaScript error preventing dropdown
- DMs section collapsed (need to expand first)

---

### Issue: User Not in Dropdown

**Symptom**: Selected user doesn't appear in dropdown list

**Debugging:**
```bash
# Check if user exists
./memdoor users list

# Check if user is already in DM
sqlite3 ~/.memdoor/data/memdoor.db "SELECT * FROM channels WHERE type='dm' AND (name LIKE '%alice%' OR name LIKE '%test%');"

# Check API response
curl http://localhost:18789/api/users -H "Authorization: Bearer YOUR_TOKEN"
```

**Common Causes:**
- User doesn't exist in system
- User is current user (can't DM yourself)
- DM already exists (dropdown filters out existing DMs)
- User account inactive/deleted

---

### Issue: Message Not Sent

**Symptom**: Message typed but doesn't appear in conversation

**Debugging:**
```bash
# Check console and network errors
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
type textarea Test message
send
wait 2
console
network
EOF

# Check for failed POST to /api/messages
# Check WebSocket connection status
# Should see "Connected" indicator in UI
```

**Common Causes:**
- WebSocket disconnected (message queued)
- Auth token expired (401 response)
- Invalid channel ID (404 response)
- Message input UID incorrect
- Send button UID incorrect

---

### Issue: Blank Screenshots

**Symptom**: Screenshots are blank or show wrong page

**Fix**: Always use DSL mode to keep browser alive:
```bash
#  WRONG - New browser for each action
./memdoor chrome navigate http://localhost:5173
./memdoor chrome screenshot  # Blank!

#  CORRECT - Same browser session with DSL
./memdoor chrome run <<'EOF'
nav http://localhost:5173
wait 2
snap /tmp/dm-test/page.png
EOF
```

## Performance Benchmarks

**Expected Timing:**
- DM creation: < 500ms
- Message send: < 300ms
- Message receive (WebSocket): < 100ms
- Sidebar DM list update: < 200ms
- Total E2E test (automated): ~10-15 seconds

**If slower:**
- Check network latency
- Check WebSocket connection stability
- Check browser rendering performance
- Check database query performance (add indexes if needed)

## Related Documentation

- **Browser Automation**: `docs/features/BROWSER_AUTOMATION.md`
- **E2E Testing Skill**: `.agents/skills/e2e-browser.md`
- **E2E Test Scripts**: `tests/e2e/README.md`
- **DM Architecture**: Check `web/src/App.tsx` (lines 78-163)
- **Sidebar DM Code**: `web/src/components/Sidebar.tsx` (lines 64-150)

## Test Report Template

```markdown
## E2E Test Report: Direct Messages

**Date**: 2024-XX-XX
**Tester**: [Name/Agent]
**Environment**: http://localhost:5173
**Test Duration**: XX seconds

### Test Execution

**Scenario 1: Create DM with Human**:  PASS /  FAIL
**Scenario 2: Send Message in DM**:  PASS /  FAIL
**Scenario 3: Create DM with Agent**:  PASS /  FAIL

### Acceptance Criteria Results

- [x] DM creation works
- [x] DM appears in sidebar with correct name
- [x] Message sending works
- [x] Message appears in conversation
- [ ] Presence indicator shows correct status (FAILED - see issue #1)
- [x] No console errors
- [x] No network errors

### Issues Found

**Issue #1: Presence indicator not updating**
- Steps: Create DM, check presence status
- Expected: Green dot for online users
- Actual: Gray dot (offline) even when user is online
- Screenshot: /tmp/dm-test/presence-issue.png
- Severity: Low (cosmetic)

### Screenshots

- Homepage: /tmp/dm-test/01-homepage.png
- DM Dropdown: /tmp/dm-test/02-dm-dropdown.png
- DM Created: /tmp/dm-test/03-dm-created.png
- Message Sent: /tmp/dm-test/05-message-sent.png

### Recommendations

1. Fix presence indicator in DM sidebar
2. Add visual test for presence states
3. Retest after fix

---
**Test Status**:  PASSED (with minor issues)
```