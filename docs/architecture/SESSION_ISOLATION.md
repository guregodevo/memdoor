# Session isolation for all agents (OpenClaw parity)

## Principle (from OpenClaw)
Every agent run has its own `sessionKey`; its reasoning transcript is loaded ONLY
from that key's store. No shared-channel history, no RAG reinjection. The channel
message log is for humans/display; the agent's transcript is per-agent-per-context.

## The bug today
The channel session key is `workspace:X:channel:Y` — the AGENT IS NOT ENCODED
(gateway/server_jobs.go:731). So `resolveSessionStorePath` routes every agent in a
channel to ONE shared `workspaces/X/channels/Y/sessions.json`, and
`buildConversation` loads that shared log + RAG. Result: the model sees other agents'
turns and its own past prose → mimics/ruminates instead of acting. Only the
`:subagent:` path is isolated today (session_persistence.go:104-116 documents
exactly this pollution).

## The change (files + functions)
1. **Encode the agent in the channel session key** at EVERY trigger origin:
   - the mention handler (messageService → AgentMentionHandler)
   - A2A (gateway/a2a_integration.go — targetSessionKey)
   - cron (server_jobs.go executeCronJob)
   - HTTP (server_http.go)
   New shape: `workspace:X:channel:Y:agent:Z` (add a builder to
   gateway/routing/session_keys.go next to BuildAgentSessionKey).
2. **resolveSessionStorePath** (session_persistence.go:75): add a branch for the
   agent-encoded channel key → `workspaces/X/channels/Y/agents/Z/sessions.json`.
   Per-(agent,channel) isolated transcript.
3. **buildConversation** (agent_adapter.go:411): treat the agent-encoded channel key
   like the `:subagent:` branch — bounded own transcript, stripOrphanedToolUse, set
   conversation_length, NO RAG.
4. **Channel display unchanged**: agent responses still POST to the channel via
   messageService (for `memdoor messages` / the TUI). Only the *reasoning transcript*
   isolates; the human-visible message log stays shared.
5. **A2A stays correct**: deliveryFunc (a2a_integration.go:61) hands the message as
   INPUT to the target agent's turn, not via shared transcript — so isolation does not
   break cross-agent comms. The mention arrives as the user message.

## Why staged, tested
Changing the session key at every origin risks: (a) breaking channel display if the
post path is coupled to the key; (b) the `conversation_length` slice panic
(seen as `[147:22]`) if a new key accidentally loads another key's store; (c) A2A
regressions if any path reads shared transcript. Each origin must be migrated and
tested (coder first — the biggest beneficiary — then chat/A2A).

## Self-heal (Layer 3b) falls out for free
Once the coder runs on its own isolated key, the verify+repair loop reuses that key:
no channel-history panic, bounded context, so the model can actually fix the line.
