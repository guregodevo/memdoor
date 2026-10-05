# Multi-Agent Collaboration

Memdoor supports sophisticated agent-to-agent (A2A) communication, enabling agents to work together on complex tasks.

## Features

### Agent Mentions
Agents can mention other agents using `@agent-name` syntax, just like in Slack or Discord.

```bash
./memdoor agent --message "@coder Build login page" --channel dev
```

### Depth Control
Built-in depth limiting prevents infinite agent loops. The system tracks:
- Current mention depth
- Maximum allowed depth (configurable)
- Automatic loop prevention

### Fan-out Collaboration
One agent can coordinate multiple other agents simultaneously:

```bash
./memdoor agent --message "@coder Build login page with @designer and @analyst" --channel dev
```

The coder agent will collaborate with both designer and analyst agents automatically.

### Threaded Conversations
Agents can reply in threads for organized discussions:
- Parent-child message relationships
- Thread-aware context
- Organized conversation flow

## How It Works

1. **User sends message** mentioning an agent (e.g., "@coder help")
2. **System detects mention** and routes to the mentioned agent
3. **Agent processes** and can mention other agents in response
4. **Depth tracking** prevents infinite loops
5. **Responses threaded** under original message

## Configuration

Agent collaboration is configured in your workspace's `AGENTS.md`:

```yaml
buddies:
  - id: coder
    name: "Code Assistant"
    # Can mention other agents

  - id: designer
    name: "Design Expert"
    # Can be mentioned by other agents
```

## Best Practices

1. **Clear delegation**: Be specific when mentioning agents
2. **Limit depth**: Keep collaboration chains short (2-3 levels max)
3. **Use threads**: Keep related collaboration in threads
4. **Monitor loops**: Check logs if agents seem stuck

## Example Workflows

### Code Review Workflow
```bash
./memdoor agent --message "@reviewer Check this PR: @coder please implement feedback" --channel code-review
```

### Research + Writing
```bash
./memdoor agent --message "@researcher Find data on X, then @writer create a report" --channel projects
```

### Multi-Phase Development
```bash
./memdoor agent --message "@analyst Spec the feature, @coder implement, @tester verify" --channel dev
```

## Debugging A2A

Use logs to debug agent collaboration:

```bash
# Check agent mentions and depth
./memdoor logs query --regex "A2A|mention_depth" --limit 20

# View specific agent's activity
./memdoor logs query --regex "agent_id.*coder" --limit 10
```

## Limitations

- **Max depth**: a chain runs at most two agents deep — user → agent →
  agent. The second agent's own @mentions are dropped (`maxMentionDepth = 2`
  in `pkg/shared/execution_context.go`; not configurable). A human message
  always bypasses the limit, so a person can restart a chain at any point.
- **No return leg**: the second agent cannot @mention the first one back, so a
  delegation is one-way — the delegating agent does not see the result unless
  the user relays it or a later human turn re-enters the thread.
- **No cycles**: Agents can't create circular mention chains
- **Rate limiting**: 10 agent executions per minute per workspace
- **Context limits**: Very long collaboration chains may hit context limits

## See Also

- [Agent Configuration](../developers/BUILDING_AGENTS.md)
- [Threading](channels-and-threading.md)
- Testing A2A: the quick test in [`AGENTS.md`](../../AGENTS.md) ("Agent-to-Agent (A2A) Communication Testing")
