# Thinking Mode

Control whether agents show their reasoning process before providing answers.

## Overview

Thinking mode allows you to see (or hide) how agents reason through problems. When enabled, agents show their internal thought process before giving final answers.

## Modes

### Enabled
Agent shows detailed reasoning:
```bash
./memdoor agent --message "Debug this issue" --thinking enabled
```

**Output**:
```
<thinking>
Let me analyze this issue:
1. The error occurs at line 42
2. Variable is undefined
3. Likely cause: missing import
4. Solution: Add import statement
</thinking>

The issue is a missing import. Add this line at the top:
import { utils } from './utils';
```

### Disabled
Agent gives direct answers only:
```bash
./memdoor agent --message "Quick question" --thinking disabled
```

**Output**:
```
The issue is a missing import. Add this line at the top:
import { utils } from './utils';
```

### Auto (Default)
Agent decides based on task complexity:
```bash
./memdoor agent --message "Help me" --thinking auto
```

**Behavior**:
- **Simple queries**: No thinking shown
- **Complex problems**: Thinking shown
- **Debugging**: Usually shows thinking
- **Quick facts**: No thinking shown

## When to Use Each Mode

### Enable Thinking When:
- Debugging complex issues
- Learning how agent solves problems
- Verifying agent's reasoning
- Working on critical tasks
- Teaching or demonstrating

### Disable Thinking When:
- You need quick answers
- Output is being piped to another tool
- Terminal space is limited
- You trust the agent's judgment
- Working with simple queries

### Use Auto When:
- General usage (recommended default)
- Unsure of task complexity
- Want optimal experience

## Configuration

### Set Default Mode

In `~/.memdoor/config.yaml`:
```yaml
agent:
  thinking_mode: auto  # or "enabled", "disabled"
```

### Per-Agent Configuration

In workspace `AGENTS.md`:
```yaml
buddies:
  - id: coder
    thinking_mode: enabled  # Always show thinking

  - id: quick-helper
    thinking_mode: disabled  # Never show thinking
```

### Override Per-Command

```bash
# Override default for this message
./memdoor agent --message "Debug" --thinking enabled

# Even if agent default is "enabled"
./memdoor agent --message "Quick" --thinking disabled
```

## Examples

### Debugging Complex Issue

```bash
./memdoor agent --message "Why is authentication failing?" \
  --thinking enabled \
  --channel support
```

Output shows:
1. Analysis of error logs
2. Identification of root cause
3. Step-by-step solution
4. Final recommendation

### Quick Factual Query

```bash
./memdoor agent --message "What's the API rate limit?" \
  --thinking disabled \
  --channel general
```

Output shows:
- Direct answer: "100 requests per minute"

### Code Review

```bash
git diff | ./memdoor agent -m "Review this code" -c general -a coder --thinking enabled
```

Output shows:
- Thought process analyzing each change
- Security considerations
- Performance implications
- Final recommendations

## Thinking Format

When enabled, thinking appears in special tags:

```
<thinking>
Internal reasoning here...
Step-by-step analysis...
Considerations...
</thinking>

Final answer here...
```

Some interfaces may:
- **CLI**: Show in different color
- **TUI**: Show in collapsible section
- **Web**: Show in expandable box
- **API**: Include in separate field

## API Usage

When calling via API:

```json
{
  "message": "Debug this issue",
  "thinking_mode": "enabled"
}
```

Response includes thinking:
```json
{
  "content": "Final answer...",
  "thinking": "Internal reasoning...",
  "thinking_shown": true
}
```

## Best Practices

### 1. Use Auto for General Chat

```bash
# Let agent decide
./memdoor agent -m "Help me with X" -c general -a coder  # Uses auto by default
```

### 2. Enable for Learning

```bash
# See how agent reasons
./memdoor agent --message "How does X work?" --thinking enabled
```

### 3. Disable for Scripting

```bash
# Clean output for parsing
result=$(./memdoor agent -m "Get API key" -c general -a coder --thinking disabled)
```

### 4. Enable for Critical Tasks

```bash
# Verify reasoning for important decisions
./memdoor agent --message "Should we deploy?" \
  --thinking enabled \
  --channel ops
```

## Performance Impact

Thinking mode affects:
- **Response time**: +10-30% when enabled (agent generates more text)
- **Token usage**: +20-50% (thinking consumes tokens)
- **Bandwidth**: Larger responses
- **Screen space**: More text to display

## Troubleshooting

### Thinking Not Showing

Check if:
1. Mode is actually enabled: `--thinking enabled`
2. Agent supports thinking (check agent config)
3. Interface supports thinking display
4. Not being stripped by output formatter

### Too Much Thinking

```bash
# If auto shows too much thinking
./memdoor config set agent.thinking_mode disabled

# Or disable per-agent in AGENTS.md
```

### Inconsistent Behavior

```bash
# Check current setting
./memdoor config get agent.thinking_mode

# Check agent override
cat ~/workspace/AGENTS.md | grep -A 5 "id: AGENT_ID"
```

### Structured Thinking (Planned)
```json
{
  "thinking": {
    "analysis": ["Point 1", "Point 2"],
    "considerations": ["Option A", "Option B"],
    "decision": "Final choice",
    "confidence": 0.95
  }
}
```

### Interactive Thinking (Planned)
```bash
# Ask questions during thinking
./memdoor agent -m "Complex problem" -c general -a coder --thinking auto
```

Agent can request clarification mid-thought:
```
<thinking>
Analyzing... I need more information.
</thinking>

[Question to user]: Which database are you using?
```

## See Also

- [Agent Configuration](../developers/BUILDING_AGENTS.md)
- [CLI Reference](../reference/CLI.md)
- [Channels & Threading](channels-and-threading.md)
