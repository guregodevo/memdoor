# Channels & Threading

Organize conversations in Slack-like channels with threading support for structured discussions.

## Overview

Memdoor provides two organizational features:
- **Channels**: Topic-based conversation spaces
- **Threading**: Reply to specific messages to keep discussions organized

## Channels

### What Are Channels?

Channels are isolated conversation spaces, similar to Slack channels. Each channel has:
- Unique name (e.g., `general`, `dev`, `support`)
- Independent message history
- Own set of participants
- Private or public visibility

### Create Channels

Channels are created automatically when first used:

```bash
# Sending to a new channel creates it
./memdoor agent --message "Hello" --channel new-channel --agent-id coder
```

Or explicitly:

```bash
./memdoor channels create engineering
./memdoor channels create support
./memdoor channels create random
```

### List Channels

```bash
./memdoor channels list
```

Output:
```
NAME         DESCRIPTION            TYPE
-------------------------------------------------------------------
general                             public
engineering                         public
support                             public
```

### Channel Membership

```bash
# Add agent to channel
./memdoor channels add-member --channel engineering --actor agent:coder

# List channel members
./memdoor channels members --channel engineering
```

There is no `channel remove-member` subcommand: membership is the actor
list the gateway keeps, and an agent that should not answer in a channel
simply is not added.

### Send Messages to Channels

```bash
# Send to specific channel
./memdoor agent --message "Start new feature" --channel dev --agent-id coder

# Default channel (if not specified)
./memdoor agent --message "Hello" --channel general --agent-id coder
```

### View Channel Messages

```bash
# View recent messages
./memdoor messages --channel dev --limit 20

# View older messages (pagination)
./memdoor messages --channel dev --before MESSAGE_ID --limit 20

# Include thread replies
./memdoor messages --channel dev --include-threads

# There is no --search on messages: pipe the channel through grep
./memdoor messages --channel dev --limit 200 | grep -i authentication
```

## Threading

### What Is Threading?

Threading allows you to reply to specific messages, creating organized sub-conversations:

```
Main message
├── Reply 1
│   └── Reply to Reply 1
├── Reply 2
└── Reply 3
```

Similar to Slack or Discord threads.

### Create Threaded Replies

**Via CLI**:
```bash
# A thread is a gateway-side relation: a subagent's turn, or a reply the
# TUI/web posts, carries parent_message_id. No CLI flag sets it.
./memdoor agent --message "Good point!" --channel dev --agent-id coder
```

**Via TUI/Web**: the reply control on a message posts it with
`parent_message_id` set, which is what puts it in the thread.

### View Threads

```bash
# View messages with their thread replies (no single-thread flag exists)
./memdoor messages --channel dev --include-threads
```

### Thread Features

#### Parent-Child Relationships
- Each thread reply has a `parent_message_id`
- Forms a tree structure
- Can have multiple levels (reply to reply)

#### Thread Context
- Agents see thread context when replying
- Previous thread messages included in context
- Maintains conversation flow

#### Thread Notifications
- Get notified when someone replies in a thread
- Follow/unfollow specific threads
- Mute noisy threads

## Use Cases

### Project Channels

Organize by project:
```bash
./memdoor channels create project-alpha
./memdoor channels create project-beta
```

Messages stay organized by project:
```bash
./memdoor agent --message "Deploy v1.0" --channel project-alpha
./memdoor agent --message "Fix bug #123" --channel project-beta
```

### Team Channels

Organize by team:
```bash
./memdoor channels create team-eng
./memdoor channels create team-product
./memdoor channels create team-design
```

### Topic Channels

Organize by topic:
```bash
./memdoor channels create architecture
./memdoor channels create deployment
./memdoor channels create incidents
```

### Threaded Discussions

Keep related messages together:

```bash
# Main question
./memdoor agent --message "How do we implement authentication?" --channel dev

# Then read the thread back (--include-threads brings the replies)
./memdoor messages --channel dev --include-threads
```

### Code Reviews

Use threads for review comments:

```bash
# Post diff for review
./memdoor agent --message "Review this PR #123" --channel code-review

# Each review turn on that conversation is a threaded reply
./memdoor messages --channel code-review --include-threads
```

### Support Tickets

Use threads for ticket conversations:

```bash
# New support ticket
./memdoor agent --message "User reports login issue" --channel support

# The troubleshooting turns thread under it
./memdoor messages --channel support --include-threads
```

## Agent Collaboration in Channels

### Multi-Agent Channels

Multiple agents can participate:

```yaml
# AGENTS.md
buddies:
  - id: coder
    channels: [dev, engineering]

  - id: reviewer
    channels: [dev, code-review]

  - id: designer
    channels: [design, engineering]
```

### Agent Mentions in Channels

```bash
# One agent mentions another in channel
./memdoor agent --message "@reviewer Please review this code" --channel dev --agent-id coder
```

Flow:
1. `coder` sends message to `dev` channel
2. `@reviewer` mention triggers reviewer agent
3. `reviewer` responds in same channel (optionally in thread)

### Threaded Agent Collaboration

```bash
# User starts thread
./memdoor agent --message "Build login page" --channel dev --agent-id coder

# Coder spawns designer in thread
# (Designer's response automatically threaded)
```

## Channel Configuration

### In `config.yaml`

```yaml
channels:
  # Default channel
  default: "general"

  # Auto-create on first use
  auto_create: true

  # Channel retention
  retention:
    default: 90  # days
    archive_after: 180  # days

  # Message limits
  max_messages: 10000
```

### In `AGENTS.md`

```yaml
buddies:
  - id: coder
    # Channels this agent monitors
    channels:
      - dev
      - engineering
      - code-review

    # Default channel for agent's cron jobs
    default_channel: dev
```

## Advanced Features

### Channel Search

```bash
# There is no cross-channel search command; grep the channel you mean
./memdoor messages --channel dev --limit 200 | grep -i "bug fix"

# Across channels, iterate (the CLI lists them first)
./memdoor channels list
```

### Channel Export

The gateway pages a channel's messages through `GET /api/messages`; the
CLI's `memdoor messages` is the reader for one channel
(`--limit`, `--before`, `--include-threads`). There is no
`channel export`/`--format` command.

### Channel Analytics

There is none: `channel stats`, `activity` and `contributors` were never
subcommands of `memdoor channels`, whose whole surface is `create`, `list`,
`add-member` and `members`.

### Thread Summarization

Ask the agent that owns the conversation: `memdoor agent --message
"summarize this thread" --channel dev --agent-id coder`. There is no
`thread` command.

## Best Practices

### 1. Use Channels for Topics

```bash
# Good: Separate channels by topic
./memdoor agent --message "Deploy v2" --channel deployments
./memdoor agent --message "Fix auth" --channel security

# Bad: Everything in one channel
./memdoor agent --message "Deploy v2" --channel general
./memdoor agent --message "Fix auth" --channel general
```

### 2. Use Threads for Discussions

```bash
# Good: Keep related messages in thread
./memdoor agent --message "Main topic" --channel dev
./memdoor agent --message "Related point" --parent-message MAIN_MSG

# Bad: Flat messages
./memdoor agent --message "Main topic" --channel dev
./memdoor agent --message "Related point" --channel dev  # Disconnected
```

### 3. Name Channels Clearly

```bash
# Good: Clear purpose
./memdoor channels create incident-2024-03-18

# Bad: Vague name
./memdoor channels create temp123
```

### 4. Archive Old Channels

There is no archive step: a channel that is done is simply not posted
into, and `memdoor channels list` shows every one.

## Limitations

### Channel Limits
- **Max channels per workspace**: 1000 (configurable)
- **Max messages per channel**: Unlimited (subject to retention)
- **Max channel name length**: 80 characters

### Thread Limits
- **Max thread depth**: 10 levels (replies to replies...)
- **Max messages per thread**: Unlimited
- **Thread context**: Last 50 messages (for agents)

## Troubleshooting

### Messages Not Appearing

```bash
# Check channel exists
./memdoor channels list | grep CHANNEL_NAME

# Check permissions
./memdoor channels members --channel CHANNEL_NAME

# View recent errors
./memdoor logs errors --since 1h
```

### Threads Not Linking

```bash
# Verify parent message exists
./memdoor messages --channel CHANNEL_NAME --limit 200 | grep PARENT_MSG_ID

# Read the thread back
./memdoor messages --channel CHANNEL_NAME --include-threads
```

### Channel Performance

```bash
# Message count: the CLI prints what it fetched
./memdoor messages --channel CHANNEL_NAME --limit 1000 | wc -l

# There is no channel cleanup command; messages go with their session
# (`memdoor sessions clear --channel CHANNEL_NAME`).
```

## See Also

- [Multi-Agent Collaboration](multi-agent-collaboration.md)
- [CLI Reference](../reference/CLI.md)
- [Message Format (Client API)](../developers/CLIENT_API.md)
