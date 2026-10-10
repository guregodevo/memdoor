# Memdoor in the editor: `memdoor acp`

`memdoor acp` is an [Agent Client Protocol](https://agentclientprotocol.com)
agent over stdio (JSON-RPC, ACP v1): the editor spawns it, and the coder
answers inside the editor's chat with its tool calls as the editor's frames.
Zed and JetBrains host ACP agents natively; VS Code through an ACP client
extension (for example "ACP Client", id `formulahendry.acp-client`):

```json
"acp.agents": { "Memdoor": { "command": "memdoor", "args": ["acp"] } }
```

What it is (2026-10-10, Greg: "i want to be able to use the agents with VS
code", "this acp should not be too intrusive"): a thin client of the local
gateway, like the window and the remote-control page — `cmd/cli/cmd/acp.go`
and `acp_agent.go`, nothing in the gateway, the tools or the TUI changed.

- A `session/new` opens a conversation on the gateway (a channel named
  `acp-<time>`, listed by `memdoor resume`), rooted at the editor's folder.
- A `session/prompt` posts one turn of the coder (`POST /api/messages`, the
  folder as the working directory) and reads the turn's events off the
  websocket: text as `agent_message_chunk`, a tool as `tool_call` (kind read /
  edit / execute / fetch, titled by its command or file, placed at the file
  for a patch) and `tool_call_update` under the same id, the receipt line
  last, then `end_turn`.
- A question (`ask_user_question`) or an approval (`MEMDOOR_APPROVE=changes`)
  is a `session/request_permission` with the gateway's options; the choice
  goes back as the option's exact text; the editor's cancel answers No.
- The editor's cancel sends the websocket `cancel` the window's Esc sends,
  and the prompt ends as `cancelled`.
- The first run sets the machine up as `memdoor tui` does; stdout carries
  only the protocol, everything else goes to stderr.

Verified with an in-process fake editor (`acp_agent_test.go`) and live: a
minimal ACP client drove a coder turn (ls, read, two patches, go test) on a
scratch gateway; the files were written and the test green.
