# Building Custom Agents

**Create specialized AI agents for your workflows**

> ⚠ **2026-05-17 local-only pivot**: examples below that show
> `provider: anthropic` / `model: claude-sonnet-4-5` on an agent
> config are historical. Every agent runs on the gateway's engine: a
> provider on the person's key, or the broker for a seat (see `memdoor
> providers`). The agent schema has no per-agent provider/model field. Tools, personality, system prompt,
> sandbox scope are unchanged.

This guide shows you how to build custom agents with unique personalities, skills, and tools.

---

## Table of Contents

1. [Agent Anatomy](#agent-anatomy)
2. [Creating Your First Agent](#creating-your-first-agent)
3. [Agent Configuration](#agent-configuration)
4. [Adding Skills](#adding-skills)
5. [Adding Tools](#adding-tools)
6. [Testing Your Agent](#testing-your-agent)
7. [Advanced Topics](#advanced-topics)

---

## Agent Anatomy

An agent in Memdoor has five key components:

```yaml
agents:
  list:
    - id: my-agent                    # 1. Unique identifier
      name: "My Custom Agent"         # 2. Display name
      personality: |                  # 3. Personality/behavior
        You are a helpful assistant specialized in...
      model:                          # 4. LLM configuration
        provider: anthropic
        name: claude-sonnet-4-5
      tools:                          # 5. Available tools
        - name: file_operations
          enabled: true
```

### 1. ID
- Unique identifier for the agent
- Used in @mentions: `@my-agent`
- Lowercase, alphanumeric, hyphens/underscores only
- Cannot be changed after creation

### 2. Name
- Human-readable display name
- Shown in UI and logs
- Can include spaces and special characters

### 3. Personality
- System prompt that defines behavior
- Sets the agent's role, expertise, tone
- Guides how the agent responds

### 4. Model
- LLM provider (anthropic, openai, etc.)
- Model name (claude-sonnet-4-5, gpt-4, etc.)
- Controls intelligence level and cost

### 5. Tools
- Functions the agent can call
- File operations, web search, data analysis, etc.
- Enable/disable per agent

---

## Creating Your First Agent

Let's create a **Code Reviewer** agent that specializes in code quality and best practices.

### Step 1: Edit Config

Open your Memdoor config:

```bash
# Open config file
code ~/.memdoor/config.yaml

# Or use the CLI
./memdoor config edit
```

### Step 2: Add Agent Definition

Add this to the `agents.list` section:

```yaml
agents:
  list:
    # ... existing agents ...

    - id: code-reviewer
      name: "Code Reviewer"
      personality: |
        You are a senior software engineer specialized in code reviews.

        Your expertise:
        - Code quality and best practices
        - Security vulnerabilities (SQL injection, XSS, etc.)
        - Performance optimization
        - Design patterns and architecture
        - Testing and test coverage

        When reviewing code:
        1. Start with positive feedback (what's done well)
        2. Identify critical issues (security, bugs)
        3. Suggest improvements (performance, readability)
        4. Recommend best practices
        5. Be constructive and educational

        Always explain WHY a change is needed, not just WHAT to change.
        Provide specific code examples when suggesting improvements.

        Respond in this format:
        ## Summary
        [Brief overview]

        ## Strengths
        - [Positive points]

        ## Issues
        ### Critical
        - [Security/bugs]

        ### Improvements
        - [Code quality]

        ## Recommendations
        - [Best practices]

      model:
        provider: anthropic
        name: claude-sonnet-4-5  # High intelligence for complex analysis

      tools:
        - name: file_operations
          enabled: true          # Can read code files
        - name: web_search
          enabled: true          # Can look up best practices
        - name: memory
          enabled: true          # Remembers past reviews
```

### Step 3: Reload Configuration

```bash
# Restart the gateway to load new agent
make stop
make start
```

### Step 4: Verify Agent

```bash
# List all agents
./memdoor agent list
```

You should see:
```
NAME           SCOPE   MODEL                STATUS
───────────────────────────────────────────────────
code-reviewer  user    claude-sonnet-4-5    online
coder          user    claude-sonnet-4-5    online
writer         user    claude-sonnet-4-5    online
analyst        user    claude-sonnet-4-5    online
```

### Step 5: Test Your Agent

```bash
# Send a code review request
./memdoor agent --message "Please review this code:

function login(user, pass) {
  const query = 'SELECT * FROM users WHERE email=' + user + ' AND password=' + pass;
  return db.execute(query);
}
" --channel engineering --agent-id code-reviewer
```

The agent should identify the **SQL injection vulnerability** and suggest using parameterized queries!

---

## Agent Configuration

### Model Selection

Choose the right model for your agent's task:

```yaml
# High intelligence (expensive, slow)
model:
  provider: anthropic
  name: claude-opus-4  # Best reasoning, highest cost

# Balanced (recommended for most tasks)
model:
  provider: anthropic
  name: claude-sonnet-4-5  # Good reasoning, moderate cost

# Fast (cheap, quick)
model:
  provider: anthropic
  name: claude-haiku-4  # Basic tasks, lowest cost
```

### Thinking Mode

Control whether agents show their thinking process:

```yaml
agents:
  list:
    - id: my-agent
      thinking: auto  # Options: enabled, disabled, auto
```

- **enabled** - Always show thinking (verbose, educational)
- **disabled** - Never show thinking (concise, fast)
- **auto** - Show thinking for complex tasks only (recommended)

### Temperature

Control randomness/creativity:

```yaml
model:
  provider: anthropic
  name: claude-sonnet-4-5
  temperature: 0.7  # Range: 0.0 (deterministic) to 1.0 (creative)
```

- **0.0** - Consistent, factual (code, data analysis)
- **0.5** - Balanced (default)
- **1.0** - Creative, varied (writing, brainstorming)

### Max Tokens

Limit response length:

```yaml
model:
  max_tokens: 4096  # Max response length
```

- **1024** - Short responses (quick answers)
- **4096** - Medium responses (explanations)
- **8192** - Long responses (detailed analysis)

---

## Adding Skills

Skills are specialized capabilities that extend agent behavior.

### Creating a Skill

Skills are markdown files in the `skills/` directory:

```bash
# Create a skill file
mkdir -p ~/.memdoor/skills
cat > ~/.memdoor/skills/code-review-checklist.md << 'EOF'
# Code Review Checklist

When reviewing code, always check:

## Security
- [ ] No SQL injection vulnerabilities
- [ ] No XSS vulnerabilities
- [ ] No hardcoded secrets/API keys
- [ ] Input validation present
- [ ] Authentication/authorization checks

## Performance
- [ ] No N+1 queries
- [ ] Efficient algorithms (avoid O(n²) if possible)
- [ ] Proper caching strategy
- [ ] Lazy loading where appropriate

## Code Quality
- [ ] DRY (Don't Repeat Yourself)
- [ ] Clear variable/function names
- [ ] Proper error handling
- [ ] Comprehensive comments
- [ ] Consistent formatting

## Testing
- [ ] Unit tests present
- [ ] Edge cases covered
- [ ] Mocks/stubs used correctly
- [ ] Test coverage > 80%

## Best Practices
- [ ] Follows project conventions
- [ ] Uses design patterns appropriately
- [ ] Proper dependency injection
- [ ] SOLID principles followed
EOF
```

### Attach Skill to Agent

```yaml
agents:
  list:
    - id: code-reviewer
      skills:
        - path: ~/.memdoor/skills/code-review-checklist.md
          enabled: true
```

The agent will now use this checklist when reviewing code!

---

## Adding Tools

Tools are functions agents can call to interact with the world.

### Available Built-in Tools

Memdoor includes these tools by default:

| Tool | Description | Use Cases |
|------|-------------|-----------|
| **file_operations** | Read/write files | Code analysis, documentation |
| **web_search** | Search the web | Research, fact-checking |
| **memory** | Query past conversations | Context recall, learning |
| **bash** | Execute shell commands | System tasks, automation |
| **browser** | Control web browser | Web scraping, testing |

### Enable/Disable Tools Per Agent

```yaml
agents:
  list:
    - id: researcher
      tools:
        - name: web_search
          enabled: true      # Researcher can search the web
        - name: bash
          enabled: false     # Researcher cannot run shell commands
```

### Tool Configuration

Some tools accept configuration:

```yaml
tools:
  - name: web_search
    enabled: true
    config:
      max_results: 10          # Return up to 10 search results
      timeout: 30000           # 30 second timeout
      allowed_domains:         # Only search these domains
        - docs.python.org
        - stackoverflow.com
```

### Creating Custom Tools

There is no separate custom-tools guide yet; the tool palette and the `tool_guards` workspace setting are documented in the [CLI Reference](../reference/CLI.md), and skills (the supported way to extend an agent without Go code) in the [Skills Reference](../reference/SKILLS.md).

---

## Testing Your Agent

### 1. Unit Testing

Test agent behavior in isolation:

```bash
# Send a test message
./memdoor agent --message "Test: What is 2+2?" --channel test --agent-id my-agent

# Verify response
./memdoor messages --channel test --limit 1
```

### 2. Integration Testing

Test agent collaboration:

```bash
# Test A2A communication
./memdoor agent --message "@my-agent Please work with @coder on this task..." --channel test --agent-id my-agent

# Verify both agents responded
./memdoor messages --channel test --limit 5
```

### 3. Debugging

Use logs to debug agent behavior:

```bash
# View agent thinking process
./memdoor logs query --regex "my-agent" --limit 20 --verbose

# Check for errors
./memdoor logs errors --limit 10

# View tool calls
./memdoor logs query --regex "tool_call" --limit 20
```

---

## Advanced Topics

### Context Window Management

Control how much context the agent sees:

```yaml
agents:
  list:
    - id: my-agent
      context:
        max_messages: 20       # Last 20 messages
        max_tokens: 100000     # Max total tokens
        compaction:
          enabled: true        # Enable memory compaction
          strategy: semantic   # Semantic summarization
```

### Agent Permissions

Restrict what agents can do:

```yaml
agents:
  list:
    - id: junior-agent
      permissions:
        can_mention_agents: false    # Cannot @mention other agents
        can_execute_tools: true      # Can use tools
        can_access_memory: false     # No access to long-term memory
        max_tool_calls: 5            # Max 5 tool calls per response
```

### Custom System Prompts

Use templates for reusable personalities:

```yaml
# templates/senior-engineer.yaml
personality_template: |
  You are a senior {{ specialty }} engineer with {{ years }} years of experience.

  Your expertise includes:
  {{ expertise }}

  When helping users:
  - Explain complex concepts simply
  - Provide code examples
  - Reference best practices
  - Cite sources when possible

# Agent using template
agents:
  list:
    - id: backend-engineer
      personality_template: senior-engineer
      template_vars:
        specialty: backend
        years: 10
        expertise: |
          - Distributed systems
          - Database optimization
          - API design
```

### Remote Agents

Use remote LLM endpoints:

```yaml
agents:
  list:
    - id: custom-llm-agent
      execution_type: remote
      remote_config:
        endpoint: https://my-llm-api.com/v1/chat/completions
        api_key: ${MY_CUSTOM_API_KEY}
        model: my-custom-model
        timeout: 60000
```

The remote agent developer guide this section used to link no longer exists; the protocol lives in `pkg/remote` (`executor.go`, `translator.go`).

---

## Example Agents

### Data Analyst

```yaml
- id: data-analyst
  name: "Data Analyst"
  personality: |
    You are a data analyst specialized in Python, pandas, and visualization.

    When analyzing data:
    1. Ask clarifying questions about the data
    2. Perform exploratory data analysis (EDA)
    3. Create visualizations (matplotlib, seaborn)
    4. Identify trends, patterns, anomalies
    5. Provide actionable insights

    Always show your work: include code, charts, and statistics.

  model:
    provider: anthropic
    name: claude-sonnet-4-5

  tools:
    - name: bash
      enabled: true  # Can run Python scripts
    - name: file_operations
      enabled: true  # Can read CSV/data files
```

### Customer Support

```yaml
- id: support-agent
  name: "Customer Support"
  personality: |
    You are a friendly customer support agent.

    Your goals:
    - Understand customer issues empathetically
    - Provide clear, step-by-step solutions
    - Escalate complex issues to humans
    - Maintain professional, friendly tone

    Always:
    - Acknowledge the customer's frustration
    - Ask clarifying questions
    - Provide specific, actionable steps
    - Follow up to ensure resolution

  model:
    provider: anthropic
    name: claude-haiku-4  # Fast responses for customers
    temperature: 0.7      # Warm, friendly tone

  tools:
    - name: memory
      enabled: true  # Remember customer history
    - name: web_search
      enabled: true  # Look up documentation
```

### Security Auditor

```yaml
- id: security-auditor
  name: "Security Auditor"
  personality: |
    You are a security expert specialized in vulnerability detection.

    When auditing code/systems:
    1. Check OWASP Top 10 vulnerabilities
    2. Review authentication/authorization
    3. Verify input validation
    4. Check for hardcoded secrets
    5. Review dependency security

    Always:
    - Classify severity (Critical/High/Medium/Low)
    - Provide CVE references when applicable
    - Suggest specific fixes with code examples
    - Recommend security tools (SAST, DAST)

  model:
    provider: anthropic
    name: claude-opus-4  # High intelligence for security analysis

  tools:
    - name: file_operations
      enabled: true
    - name: web_search
      enabled: true  # Look up CVEs, security advisories
```

---

## Next Steps

- **[CLI Reference](../reference/CLI.md)** - `agent add` / `agent update` config options and workspace settings
- **[Skills Development](../reference/SKILLS.md)** - Create reusable skills
- **[Client API](CLIENT_API.md)** - Drive agents over REST / WebSocket for testing

---

**Happy building! **
