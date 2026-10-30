# Memdoor Client API Reference

The HTTP and WebSocket surface of a Memdoor gateway, as the handlers in
`gateway/` serve it. Routes are registered in `gateway/server_lifecycle.go`;
each entry below names the handler that answers it. The CLI
(`cmd/cli/cmd/`) and the TUI are clients of this same API, so a command's
source is a worked example for any route here.

Base URL: `http://localhost:18789` (`memdoor gateway --port` changes it; a
gateway binds the network by default, `MEMDOOR_HOST=localhost` keeps it to
the machine). Requests and responses are JSON. Every route under `/api/`
except the ones marked public takes the gateway token as
`Authorization: Bearer <token>`.

## Two accounts

A gateway account and a memdoor.ai account are different things, and the
routes below take only the first.

**The gateway account** is a user row on your own gateway. The first one is
made from the gateway's machine itself: `memdoor setup` posts to
`POST /api/setup` (`gateway/setup_handlers.go handleSetup`), which refuses
any request that did not come from loopback (`fromThisMachine`), creates the
workspace, the admin user and the default agents, and answers
`{"success":true,"user":…,"token":…,"workspace_slug":…}`. After that:

```bash
# sign in with email and password; the token is kept in ~/.memdoor/credentials.json for 30 days
memdoor auth login-direct --email you@example.com --password '…'
TOKEN=$(memdoor auth token)          # the same token, for curl
curl -s http://localhost:18789/api/auth/whoami -H "Authorization: Bearer $TOKEN"
```

| Route | Handler | Notes |
|---|---|---|
| `GET /api/setup/status` (public) | `handleSetupStatus` | `{"initialized":bool,"workspace_name","workspace_slug","sign_in_with":[…]}` |
| `POST /api/setup` (public, loopback only, once) | `handleSetup` | `{"workspace_name","email","password","username"?,"language"?}` |
| `POST /api/auth/login` | `AuthGatewayAdapter.HandleLogin` | `{"email","password"}` → `{"user":{…},"token":"…"}`; rate limited per IP (429) |
| `POST /api/auth/register` | `HandleRegister` | `{"email","password","username"?,"name"?,"workspace_name"?,"invite_token"?}`; a new account gets a workspace of its own, an invite token joins the inviter's; answers like login |
| `GET /api/auth/whoami` | `HandleWhoami` | `{"user":{…}}` for the bearer token |
| `POST /api/auth/logout` | `HandleLogout` | invalidates the token |
| `POST /api/auth/change-password` | `HandleChangePassword` | authenticated |
| `GET /api/auth/cli?callback=http://127.0.0.1:…` | `handleCLIAuth` (`gateway/user_handlers.go`) | browser sign-in that redirects to a loopback callback with `?token=`; any other callback host is refused |
| `GET /api/auth/google`, `/api/auth/github` (+ `/callback`) | `HandleSocialStart`, `HandleSocialCallback` | only when the gateway is configured for them (`SocialProviders`) |

**The memdoor.ai account** is `memdoor login you@example.com`: a six-digit
code is emailed and typed back (`cmd/cli/cmd/account.go`). Its token is
used for two things only: the relay for a gateway kept behind NAT
(`/api/relay`, where `gateway/remote_relay_auth.go relayAuth` accepts a
gateway token first and a memdoor.ai account token second) and the hosted
workflow state. No route in the rest of this page accepts it.

## Status

```bash
curl -s http://localhost:18789/api/status     # {"status":"ok","version":"…","pid":12345}
curl -s http://localhost:18789/health          # the health snapshot; ?probe=true forces a fresh one
```

`GET /api/status` (`gateway/status_handler.go handleStatus`) is public and
is what `make up`, the installer and the TUI poll for readiness.

## Channels

`gateway/channel_handlers.go`. A channel is a conversation; its id is a
UUID string. `ChannelDTO` (`gateway/dto/channel.go`): `id`, `name`, `type`
(`public`, `private`, `dm`), `description`, `members` (actor ids),
`created_at`, `updated_at`.

```bash
curl -s http://localhost:18789/api/channels -H "Authorization: Bearer $TOKEN"
# {"channels":[{"id":"…","name":"general","type":"public","members":["human:…","agent:coder"],…}]}

curl -s -X POST http://localhost:18789/api/channels -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"test","type":"public","description":"scratch"}'
# 201, the ChannelDTO; admin only

curl -s "http://localhost:18789/api/channels/<id>/messages?limit=20&before=<message id>" \
  -H "Authorization: Bearer $TOKEN"
# {"messages":[MessageDTO…],"has_more":false}; what `memdoor messages` reads
```

| Route | Handler |
|---|---|
| `GET /api/channels` | `handleRooms`: the channels of the caller's workspace the caller may read |
| `POST /api/channels` | `handleRooms`: `{"name","type","description"}`, admin only, 201 |
| `DELETE /api/channels/<id>` | `handleDeleteRoom`, admin only |
| `GET /api/channels/<id>/messages?limit=&before=` | `handleGetRoomMessages`, cursor paging on the message id |
| `GET /api/channels/<id>/members` | `handleGetRoomMembers` → `{"members":[ChannelMemberDTO]}` (`actor_id`, `name`, `avatar_emoji`, `icon`, `status`, `role`, `joined_at`) |
| `POST /api/channels/<id>/join`, `/leave`, `/read` | join a public channel, leave, mark every message read |
| `GET /api/channels/<id>/membership` | is the caller a member |
| `POST /api/channels/members` | `handleRoomMembers`: `{"channel_id","actor_id","role":"member"\|"admin"}`, 201 |

There is no `GET` or `PUT` of a single channel: `GET /api/channels/<id>`
answers `{"status":"ok"}` and nothing else.

## Messages

`gateway/message_handlers.go`. Posting a message whose text mentions an
agent (`@coder …`) starts that agent's turn in the channel; this is what
`memdoor agent --message` does (`cmd/cli/cmd/agent.go`).

```bash
curl -s -X POST http://localhost:18789/api/messages -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"channel_id":"<id>","text":"@coder what does main.go do?","workspace":"<slug>","workdir":"/abs/path/on/the/gateway"}'
# 201, the MessageDTO of the posted message; the reply arrives in the channel
```

`POST /api/messages` body: `channel_id` and `workspace` (slug) are required;
`text` is the message; `permission_mode` is `plan` (read-only turn),
`acceptEdits` or empty; `workdir` is an absolute directory on the gateway's
machine the coder works in (a missing or relative one is a 400);
`attachment_ids` are ids from `POST /api/files`. The caller must be a member
of the channel (403 otherwise).

`MessageDTO` (`gateway/dto/message.go`): `id`, `channel_id`, `author_id`
(`human:…` or `agent:<name>`), `author_name`, `content` (`text`, `mentions`
as actor ids, `link_previews`, `attachments` with `id`, `filename`,
`mime_type`, `size_bytes`, `url`), `parent_id` (a thread reply's parent),
`is_read`, `created_at`, `updated_at`.

| Route | Handler |
|---|---|
| `GET /api/messages/<id>` | `handleGetSingleMessage` |
| `PUT /api/messages/<id>` | `handleEditMessage`: `{"text"}`, the author only |
| `GET /api/messages/<id>/replies` | the thread under a message |
| `POST /api/messages/<id>/replies` | reply in that thread |
| `POST /api/messages/<id>/read` | `handleMarkMessageRead` |
| `GET /api/messages/unread-mentions` | `handleUnreadMentionCounts`: per-channel counts for the caller |
| `GET /api/messages/threads?limit=` | `handleListUnreadMentions`: the unread mentions themselves |
| `POST /api/files` (multipart), `GET /api/files/<id>` | `handleFileUpload`, `handleFileByID`; the download also takes `?token=` for an `img` tag |

There is no `GET /api/messages` and no `DELETE` of a message.

## Agents

`gateway/agent_handlers.go`. An agent row carries no model: every agent
runs through a provider on the person's own key, and the pin is on the
route (below).

```bash
curl -s http://localhost:18789/api/agents -H "Authorization: Bearer $TOKEN"
# {"agents":[{"id":"agent:coder","name":"coder","avatar_emoji":"…","avatar_url":"","icon":"code",
#             "profile":"full","tools":["bash","read_file",…],"sandbox_scope":"workspace","status":"online"}]}
```

| Route | Handler |
|---|---|
| `GET /api/agents` | `handleAgents`: the rows above; agents marked `admin_only` are hidden from non-admins |
| `POST /api/agents` | `handleCreateAgent`, admin only: `{"name","avatar_emoji","description","personality","system_prompt","tools":[…],"sandbox_scope":"user"\|"channel"\|"workspace","learning_enabled","execution_type":"local"\|"remote","remote_config":{"endpoint","model",…}}`; 201 |
| `GET /api/agents/<name>` | `handleAgentByName`: the row plus `description`, `personality`, `system_prompt`, `learning_enabled`, `execution_type` |
| `PUT /api/agents/<name>` | admin only; any subset of `learning_enabled`, `tools`, `sandbox_scope`, `avatar_emoji`, `description`, `personality`, `system_prompt`, `execution_type`, `endpoint`, `admin_only`, `remote_config`; a remote agent must end up with `remote_config.endpoint` and `.model` |
| `DELETE /api/agents/<name>` | admin only |
| `GET`/`POST`/`DELETE /api/agents/<name>/secrets[/<secret>]` | `handleAgentSecrets`, admin only: names the agent's `get_secret` tool can read; `POST {"name","value"}`; values are never returned |

## Users and workspace

| Route | Handler |
|---|---|
| `GET /api/users/<id>` | `handleGetUserByID` (`gateway/user_handlers_crud.go`) |
| `GET /api/users/available-for-dm` | `handleAvailableUsersForDM`: the people and agents sharing a channel with the caller (`id`, `name`, `avatar_emoji`, `icon`) |
| `POST /api/users/avatar` | `handleAvatarUpload` |
| `GET /api/workspace/users` | `handleListWorkspaceUsers`, for mention completion |
| `GET`/`PUT /api/workspace/settings` | `handleWorkspaceSettings` (`gateway/workspace_settings_handlers.go`): `{"settings":{key:value}}`; both ways are the gateway operator's alone (an admin of the first account's workspace, signed in on the gateway's machine, `gatewayOperator`; anyone else, or from the network or through a proxy: 403; the same for `/api/providers/connect` and `POST /api/providers?refresh=1`), and `PUT` accepts the keys in `globalSettingKeys` (`tool_guards`, `approve`, `edit_format`, the `decision_*` switches) |
| `GET /api/workspace/settings/public` (public) | the `language` setting |
| `GET`/`POST /api/admin/users`, `PUT /api/admin/users/<id>` | admin: list, create, change a role |
| `DELETE /api/workspaces/<slug>` | admin cascade delete |

## Model route

`GET`/`POST /api/route` (`gateway/route_handler.go handleRoute`) is what
`/model` in the window and `memdoor model` use: which rung of an agent's
ladder answers a conversation, and the pin. `GET` takes `workspace`,
`channel_id`, `agent` as query parameters; `POST` takes the same in the body
plus one of `tier` (a 1-based rung, `0` or `"auto"`), `model` (an id off the
ladder, with `sort` and `order` for the host) or `effort` (`low`, `medium`,
`high`, `auto`). A pin is kept per person for later conversations
(`gateway/persistent_pin.go`). `GET /api/models`, `/api/models/providers`,
`/api/models/check`, `/api/providers`, `POST /api/providers/connect` back
`memdoor model`, `memdoor providers` and `memdoor connect`
(`gateway/models_handler.go`).

## Logs

`gateway/server_logs.go`, admin only; the structured event store that
`memdoor logs` reads.

```bash
curl -s "http://localhost:18789/api/logs/query?since=5m&regex=coder&limit=20" -H "Authorization: Bearer $TOKEN"
# {"events":[…],"total_matched":N,"query_time":…}
```

| Route | Parameters |
|---|---|
| `GET /api/logs/query` | `since` (`5m`, `2h`), `levels`, `components`, `session`, `run`, `errors_only=true`, `regex`, `workspace`, `limit` |
| `GET /api/logs/errors` | `since`, `limit` |
| `GET /api/logs/stats` | storage counts |
| `GET /api/logs/trace?event_id=` | the causal chain of one event: `{"root_event","chain","total_events","depth"}` |
| `GET /api/logs/session?session_id=` | one session's timeline |
| `POST /api/logs/prune` | delete old events |

## The project's files from another machine

`GET /api/fs` (`gateway/fs_handlers.go handleFS`) is the door a window on a
laptop uses against a gateway on a box (`memdoor tui --gateway <box> --dir
<path there>`): the files panel, `@file` mentions and `/dir` read the
project where the gateway is.

```bash
curl -s "http://localhost:18789/api/fs?op=stat&path=/srv/app" -H "Authorization: Bearer $TOKEN"
# {"path":"/srv/app","dir":true,"exists":true}
curl -s "http://localhost:18789/api/fs?op=list&root=/srv/app"              # {"root":…,"files":[…]}
curl -s "http://localhost:18789/api/fs?op=read&root=/srv/app&path=main.go"  # {"lines":[…]}; &diff=1 for the diff view
```

`op` is `stat`, `list` or `read`; `path` under `read` must be relative to
`root`. What is answered is what the coder can read anyway, for the
authenticated user.

## The phone's link, hosted by the gateway

`/api/remote/<channel>` (`gateway/remote_host.go remoteHosted.handle`) makes
the gateway's own process the terminal end of a `/remote` link, so the link
outlives the window that made it (`~/.memdoor/remote-hosted.json`).

| Method | Body / answer |
|---|---|
| `GET` | `{"hosted":false}` or `{"hosted":true,"id","key","relay"}`, for the person who turned it on |
| `PUT` | `{"workspace","channel_name"?,"dir"?,"id","key","token","relay","relay_token"?}`: the link's id and key, the session token the host follows and posts with, and where the link lives (this gateway's address, or memdoor.ai with the account token for a gateway kept to localhost) |
| `DELETE` | stop hosting; the pages on the link are told |

A gateway without a host (an older binary) answers 404 and the window hosts
the link itself. The relay sockets are `/api/relay` (the terminal end) and
`/api/relay/browser` (the page), `gateway/remote_relay.go`.

## Other routes

One line each; the handler file has the shape.

- `GET`/`POST /api/cron`, `GET`/`DELETE /api/cron/<id>`, `GET /api/cron-history`, `/api/cron-stats`: scheduled checks (`memdoor cron`).
- `/api/workflow`, `/api/workflow/<run>`: run, status, stop, approve a DAG on mario (`gateway/workflow_runs.go`).
- `POST /api/sessions/fresh`, `/compact`, `/handoff`: `/fresh`, `/compact`, `/handoff` in the window (`gateway/session_fresh.go`, `session_compact.go`).
- `/api/mcp`, `/api/mcp/<…>`: the person's MCP servers (`gateway/mcp_handlers.go`).
- `GET`/`POST`/`DELETE /api/secrets[/<name>]`: gateway-wide secrets, admin only, values masked.
- `/api/memory`, `/api/memory/<id>`: an agent's memory store (`gateway/server_memory.go`).
- `POST /api/agent`, `POST /api/message` (`gateway/server_http.go`): the run-queue RPC the agents' own `sessions_*` tools use; a client posts to `/api/messages` instead.
- `/sessions`, `/sessions/detail`, `/sessions/history`, `/sessions/clear`, `/sessions/rewind`, `/queue`: admin-only session introspection behind `memdoor sessions`.
- `POST /api/share`: a sealed transcript behind `https://memdoor.ai/s/<id>#k=<key>` (`gateway/share_store.go`).

## WebSocket: a conversation (`/ws`)

`gateway/server_websocket.go handleWebSocket`. The TUI's socket: one
connection follows one conversation and drives turns on it. The token goes
in `Authorization: Bearer` or `?token=`; `?workspace=<slug>&channel=<id>`
names the conversation (the server builds the session key; a channel
outside the caller's workspace closes the socket). A frame is
`{"type":…,"session_id":…,"data":{…},"error":…}` (`gateway/protocol/types.go
Message`). The types are the constants in that file:

| Direction | `type` | `data` |
|---|---|---|
| server → client | `connected` | on open |
| client → server | `ping` / server `pong` | keepalive |
| client → server | `chat` | `text`; `permission_mode` (`plan`, `acceptEdits`); `workdir`, an absolute directory on the gateway's machine |
| client → server | `cancel` | interrupt the running turn (Esc); the server answers `cancelled` |
| client → server | `answer` | `question_id`, `answer`: the reply to an `ask_user_question` picker |
| server → client | `chat_response`, `agent_response` | the turn's reply |
| server → client | `agent_event` | `run_id`, `seq`, `stream`, `timestamp`, `session_id`, `data` (`event` is `thinking`, a tool call, a tool result…; `gateway/streaming/integration.go`) |
| server → client | `thinking`, `tool_execution` | reserved constants; the TUI reads these from `agent_event` |
| server → client | `error` | `error` text |

Events emitted while a client is disconnected are not replayed.

## WebSocket: channel events (`/chat/ws`)

`gateway/websocket/handler.go HandleWebSocketWithAuth`, the hub the web page
uses. Token as above, `?channel=<id>` to follow one channel. Frames are
`{"type":…,"data":…}` with the `MessageType` constants in
`gateway/websocket/hub.go`: `message.created`, `message.updated`,
`message.deleted`, `execution.started`, `execution.event`,
`execution.completed`, `execution.todo.updated`, `typing.started`,
`typing.stopped`, `presence.updated`, `mention.created`. A cross-origin
upgrade is refused unless the origin is the gateway's own.

## Errors

A handler answers an HTTP status and, in most cases, `{"error":"…"}`; a
few older handlers answer the text alone. The codes in use: 400 (a missing
or malformed field), 401 (no or invalid token), 403 (not a member, not an
admin, or `POST /api/setup` from the network), 404 (no such conversation,
agent or message), 405, 409 (setup already done), 429 (login and register
rate limits), 500. There are no rate-limit headers.

## See also

- [Building Agents](BUILDING_AGENTS.md): what an agent row is and how to shape one
- [CLI Reference](../reference/CLI.md): every command, including `auth token`, `agent`, `messages`, `logs`
- [Remote control](../features/REMOTE.md): the relay, the link, the audience
