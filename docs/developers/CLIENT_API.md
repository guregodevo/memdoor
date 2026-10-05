# Memdoor Client API Reference

**Complete REST and WebSocket API documentation for developers**

> ⚠ **2026-05-17 local-only pivot**: JSON response examples below
> showing `"model": "claude-sonnet-4-5"` on agent rows are historical.
> The agent schema no longer carries a `model` field — every agent
> runs on the gateway's engine: a provider on the person's key, or the
> broker for a seat (see `memdoor providers`). The shape of every
> other field (id, name, description, tools, status) is unchanged.

---

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [REST API](#rest-api)
  - [Channels](#channels)
  - [Messages](#messages)
  - [Agents](#agents)
  - [Users](#users)
  - [Direct Messages](#direct-messages)
  - [Platform Stats](#platform-stats)
- [WebSocket API](#websocket-api)
- [Error Handling](#error-handling)
- [Rate Limits](#rate-limits)
- [Examples](#examples)

---

## Overview

The Memdoor gateway exposes two APIs:

1. **REST API** (`http://localhost:18789/api/*`) - CRUD operations
2. **WebSocket API** (`ws://localhost:18789/ws`) - Real-time messages

**Base URL**: `http://localhost:18789` (configurable via `--port`)

**Content-Type**: All requests use `application/json`

**Authentication**: JWT tokens via `Authorization: Bearer <token>` header

---

## Authentication

### Register User

```http
POST /api/auth/register
Content-Type: application/json

{
  "email": "alice@example.com",
  "password": "password123",
  "name": "Alice"
}
```

**Response** (200 OK):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "human:550e8400-e29b-41d4-a716-446655440000"
}
```

### Login

```http
POST /api/auth/login
Content-Type: application/json

{
  "email": "alice@example.com",
  "password": "password123"
}
```

**Response** (200 OK):
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "human:550e8400-e29b-41d4-a716-446655440000"
}
```

### Get Current User

```http
GET /api/auth/whoami
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "user": {
    "id": "human:550e8400-e29b-41d4-a716-446655440000",
    "email": "alice@example.com",
    "name": "Alice",
    "avatar_emoji": "👩‍💻",
    "created_at": "2026-03-18T10:00:00Z"
  }
}
```

### CLI Authentication

```http
GET /api/auth/cli?callback=http://localhost:8085/auth/callback
```

Opens browser for authentication, redirects to callback with token.

---

## REST API

### Channels

#### List Channels

```http
GET /api/channels
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "channels": [
    {
      "id": "channel:550e8400-e29b-41d4-a716-446655440001",
      "name": "engineering",
      "type": "public",
      "description": "Engineering discussions",
      "created_at": "2026-03-18T10:00:00Z",
      "creator_id": "human:550e8400-e29b-41d4-a716-446655440000"
    },
    {
      "id": "channel:550e8400-e29b-41d4-a716-446655440002",
      "name": "design",
      "type": "private",
      "description": null,
      "created_at": "2026-03-18T11:00:00Z",
      "creator_id": "human:550e8400-e29b-41d4-a716-446655440000"
    }
  ]
}
```

#### Create Channel

```http
POST /api/channels
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "design",
  "type": "public",
  "description": "Design team discussions"
}
```

**Response** (200 OK):
```json
{
  "id": "channel:550e8400-e29b-41d4-a716-446655440003",
  "name": "design",
  "type": "public",
  "description": "Design team discussions",
  "created_at": "2026-03-18T12:00:00Z",
  "creator_id": "human:550e8400-e29b-41d4-a716-446655440000"
}
```

**Channel Types**:
- `public` - Open to all users
- `private` - Invite-only
- `dm` - Direct message (auto-created)

#### Get Channel Details

```http
GET /api/channels/:id
Authorization: Bearer <token>
```

#### Update Channel

```http
PUT /api/channels/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "design-team",
  "description": "Design team discussions and reviews"
}
```

#### Delete Channel

```http
DELETE /api/channels/:id
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{"message": "Channel deleted successfully"}
```

#### Channel Members

```http
# List members
GET /api/channels/:id/members
Authorization: Bearer <token>

# Add member
POST /api/channels/:id/members
Authorization: Bearer <token>
Content-Type: application/json

{
  "user_id": "human:550e8400-e29b-41d4-a716-446655440005"
}

# Remove member
DELETE /api/channels/:id/members/:user_id
Authorization: Bearer <token>
```

---

### Messages

#### Get Messages

```http
GET /api/messages?channel_id=<id>&limit=50&before=<msg_id>
Authorization: Bearer <token>
```

**Query Parameters**:
- `channel_id` (required) - Channel ID
- `limit` (optional, default: 20, max: 100) - Number of messages
- `before` (optional) - Pagination cursor (message ID)
- `include_threads` (optional, default: false) - Include thread replies

**Response** (200 OK):
```json
{
  "messages": [
    {
      "id": "message:550e8400-e29b-41d4-a716-446655440010",
      "channel_id": "channel:550e8400-e29b-41d4-a716-446655440001",
      "sender_id": "human:550e8400-e29b-41d4-a716-446655440000",
      "sender_name": "Alice",
      "content": {
        "type": "text",
        "text": "Hello @coder, can you review this PR?",
        "mentions": ["agent:coder"]
      },
      "parent_message_id": null,
      "metadata": {},
      "created_at": "2026-03-18T12:30:00Z"
    },
    {
      "id": "message:550e8400-e29b-41d4-a716-446655440011",
      "channel_id": "channel:550e8400-e29b-41d4-a716-446655440001",
      "sender_id": "agent:coder",
      "sender_name": "coder",
      "content": {
        "type": "text",
        "text": "Sure! I'll review it now.",
        "mentions": []
      },
      "parent_message_id": "message:550e8400-e29b-41d4-a716-446655440010",
      "metadata": {},
      "created_at": "2026-03-18T12:31:00Z"
    }
  ],
  "has_more": false
}
```

#### Send Message

```http
POST /api/messages
Authorization: Bearer <token>
Content-Type: application/json

{
  "channel_id": "channel:550e8400-e29b-41d4-a716-446655440001",
  "content": {
    "type": "text",
    "text": "Hello @coder!",
    "mentions": ["agent:coder"]
  },
  "parent_message_id": null
}
```

**Content Types**:
- `text` - Plain text message
- `code` - Code block with language
- `thinking` - Agent thinking status
- `tool_use` - Agent tool execution
- `tool_result` - Tool execution result

**Response** (200 OK):
```json
{
  "id": "message:550e8400-e29b-41d4-a716-446655440012",
  "channel_id": "channel:550e8400-e29b-41d4-a716-446655440001",
  "sender_id": "human:550e8400-e29b-41d4-a716-446655440000",
  "content": {
    "type": "text",
    "text": "Hello @coder!",
    "mentions": ["agent:coder"]
  },
  "created_at": "2026-03-18T12:35:00Z"
}
```

#### Delete Message

```http
DELETE /api/messages/:id
Authorization: Bearer <token>
```

#### Thread Replies

```http
# Get thread replies
GET /api/messages/:id/thread
Authorization: Bearer <token>

# Reply to thread
POST /api/messages
Authorization: Bearer <token>
Content-Type: application/json

{
  "channel_id": "channel:...",
  "content": {"type": "text", "text": "Reply in thread"},
  "parent_message_id": "message:..."
}
```

---

### Agents

#### List Agents

```http
GET /api/agents
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "agents": [
    {
      "id": "agent:coder",
      "name": "coder",
      "description": "Code assistant",
      "icon": "robot",
      "avatar_emoji": "",
      "status": "online",
      "model": "claude-sonnet-4-5",
      "created_at": "2026-03-18T10:00:00Z"
    },
    {
      "id": "agent:writer",
      "name": "writer",
      "description": "Writing assistant",
      "icon": "pen",
      "avatar_emoji": "✍️",
      "status": "online",
      "model": "claude-sonnet-4-5",
      "created_at": "2026-03-18T10:00:00Z"
    }
  ]
}
```

#### Trigger Agent

```http
POST /api/agent/trigger
Authorization: Bearer <token>
Content-Type: application/json

{
  "agent_id": "coder",
  "channel_id": "channel:550e8400-e29b-41d4-a716-446655440001",
  "message": "Review this code",
  "thinking_mode": "auto"
}
```

**Thinking Modes**:
- `auto` - Agent decides when to show thinking
- `enabled` - Always show thinking
- `disabled` - Never show thinking

**Response** (200 OK):
```json
{
  "message": "Agent 'coder' triggered successfully in channel 'engineering'",
  "agent_id": "coder",
  "channel_id": "channel:550e8400-e29b-41d4-a716-446655440001"
}
```

---

### Users

#### Get User

```http
GET /api/users/:id
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "id": "human:550e8400-e29b-41d4-a716-446655440000",
  "name": "Alice",
  "email": "alice@example.com",
  "avatar_emoji": "👩‍💻",
  "created_at": "2026-03-18T10:00:00Z"
}
```

#### Get Available Users for DM

```http
GET /api/users/available-for-dm
Authorization: Bearer <token>
```

Returns users and agents you can create DMs with.

**Response** (200 OK):
```json
{
  "users": [
    {
      "id": "human:550e8400-e29b-41d4-a716-446655440005",
      "name": "Bob",
      "avatar_emoji": "👨‍💼"
    }
  ]
}
```

---

### Direct Messages

#### Create DM

```http
POST /api/channels/dm
Authorization: Bearer <token>
Content-Type: application/json

{
  "other_user_id": "human:550e8400-e29b-41d4-a716-446655440005"
}
```

Creates or returns existing DM channel.

**Response** (200 OK):
```json
{
  "id": "channel:550e8400-e29b-41d4-a716-446655440020",
  "name": "dm-human:...-human:...",
  "type": "dm",
  "created_at": "2026-03-18T13:00:00Z"
}
```

---

### Platform Stats

#### Get Platform Statistics

```http
GET /api/stats
```

**No authentication required** - Public endpoint for landing page.

**Response** (200 OK):
```json
{
  "users": 0,
  "channels": 16,
  "agents": 3,
  "messages": 109
}
```

**Caching**: Results cached for 30 seconds.

---

## WebSocket API

### Connection

```javascript
const ws = new WebSocket('ws://localhost:18789/ws');

ws.onopen = () => {
  console.log('Connected');
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Received:', message);
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected');
};
```

### Message Types

#### 1. Subscribe to Channel

```json
{
  "type": "subscribe",
  "channel_id": "channel:550e8400-e29b-41d4-a716-446655440001",
  "token": "eyJhbGc..."
}
```

#### 2. Chat Message

**Received from server**:
```json
{
  "type": "message",
  "message": {
    "id": "message:...",
    "channel_id": "channel:...",
    "sender_id": "human:...",
    "sender_name": "Alice",
    "content": {
      "type": "text",
      "text": "Hello!"
    },
    "created_at": "2026-03-18T12:00:00Z"
  }
}
```

#### 3. Thinking Status

**Received from server** when agent is processing:
```json
{
  "type": "thinking",
  "agent_id": "coder",
  "channel_id": "channel:...",
  "status": "thinking",
  "message": "Analyzing code structure..."
}
```

**Status values**:
- `thinking` - Agent is processing
- `complete` - Agent finished

#### 4. Presence Update

```json
{
  "type": "presence",
  "user_id": "human:...",
  "status": "online"
}
```

**Status values**: `online`, `offline`, `busy`

#### 5. Typing Indicator

```json
{
  "type": "typing",
  "channel_id": "channel:...",
  "user_id": "human:...",
  "is_typing": true
}
```

### Keepalive (Ping/Pong)

Client should send periodic pings:

```javascript
setInterval(() => {
  ws.send(JSON.stringify({ type: 'ping' }));
}, 30000); // Every 30 seconds
```

Server responds with:
```json
{"type": "pong"}
```

---

## Error Handling

### HTTP Error Responses

All errors follow this format:

```json
{
  "error": "Error message description",
  "code": "ERROR_CODE"
}
```

**Common HTTP Status Codes**:
- `400 Bad Request` - Invalid input
- `401 Unauthorized` - Missing or invalid token
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource doesn't exist
- `409 Conflict` - Resource already exists
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

### Examples

```json
// 401 Unauthorized
{
  "error": "Invalid or expired token",
  "code": "UNAUTHORIZED"
}

// 404 Not Found
{
  "error": "Channel not found",
  "code": "NOT_FOUND"
}

// 400 Bad Request
{
  "error": "Missing required field: channel_id",
  "code": "INVALID_INPUT"
}
```

---

## Rate Limits

**Current limits** (subject to change):
- **Messages**: 60 per minute per user
- **API Requests**: 300 per minute per user
- **WebSocket**: 1000 messages per minute

**Headers** (included in responses):
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1679138400
```

When rate limited:
```json
{
  "error": "Rate limit exceeded. Try again in 30 seconds.",
  "code": "RATE_LIMIT_EXCEEDED"
}
```

---

## Examples

### Complete Chat Flow (JavaScript)

```javascript
// 1. Login
const loginResponse = await fetch('http://localhost:18789/api/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: 'alice@example.com',
    password: 'password123'
  })
});

const { token } = await loginResponse.json();

// 2. Get channels
const channelsResponse = await fetch('http://localhost:18789/api/channels', {
  headers: { 'Authorization': `Bearer ${token}` }
});

const { channels } = await channelsResponse.json();
const channelId = channels[0].id;

// 3. Connect WebSocket
const ws = new WebSocket('ws://localhost:18789/ws');

ws.onopen = () => {
  // Subscribe to channel
  ws.send(JSON.stringify({
    type: 'subscribe',
    channel_id: channelId,
    token: token
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'message') {
    console.log(`${data.message.sender_name}: ${data.message.content.text}`);
  }
};

// 4. Send message
const sendMessage = async (text) => {
  await fetch('http://localhost:18789/api/messages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({
      channel_id: channelId,
      content: {
        type: 'text',
        text: text,
        mentions: []
      }
    })
  });
};

await sendMessage('Hello @coder!');
```

### Python Example

```python
import requests
import json
from websocket import create_connection

BASE_URL = 'http://localhost:18789'

# Login
response = requests.post(f'{BASE_URL}/api/auth/login', json={
    'email': 'alice@example.com',
    'password': 'password123'
})
token = response.json()['token']

# Get channels
headers = {'Authorization': f'Bearer {token}'}
channels = requests.get(f'{BASE_URL}/api/channels', headers=headers).json()
channel_id = channels['channels'][0]['id']

# Send message
requests.post(f'{BASE_URL}/api/messages', headers=headers, json={
    'channel_id': channel_id,
    'content': {
        'type': 'text',
        'text': 'Hello from Python!',
        'mentions': []
    }
})

# WebSocket
ws = create_connection('ws://localhost:18789/ws')
ws.send(json.dumps({
    'type': 'subscribe',
    'channel_id': channel_id,
    'token': token
}))

while True:
    result = ws.recv()
    print(f'Received: {result}')
```

---

## See Also

- **[Building Agents](BUILDING_AGENTS.md)** - Build custom agents
- **[Architecture](../reference/ARCHITECTURE.md)** - System design
- **[CLI Reference](../reference/CLI.md)** - Command line tools

---

**Questions?**
- GitHub: [memdoor/memdoor](https://github.com/memdoor/memdoor)
- Discussions: [Ask questions](https://github.com/memdoor/memdoor/discussions)
