# ADR-0015: Decisions through a decision model, on the person's own key

Date: 2026-09-25 · Status: accepted; amended 2026-10-03 (the broker route is
gone: "jev decision model is not pro") and 2026-10-04 (a seat never supplies
a key). Record: `docs/features/DECIDE.md`.

## Context

The harness makes dozens of small judgments per turn: which grep hits
matter, which log lines, which tools a request needs, how much reasoning a
turn deserves, whether an agent should answer a message at all. Written as
regexes and phrase lists they overfit; asked of the chat model they cost a
full turn of reading. A decision model answers them as typed questions with
a probability per option, no text to parse, in under half a second, at a
price two orders of magnitude under the chat model's.

Jev (`~typesafe/jev-latest`, through OpenRouter's decisions endpoint) was
the one that answered the test tickets correctly and was already reachable
on the key most people have. A local engine in the binary was tried and
dropped with the local model (2026-10-03: API only).

## Decision

- **Decisions go to a decision model from the gateway, on a key of the
  person's own.** `gateway/decision_model.go` (`autoDecisionModel`) picks
  the route per call: an explicit decision key (`MEMDOOR_SYSTEMONE_API_KEY`,
  with `MEMDOOR_SYSTEMONE_URL` and `_MODEL` to point it at TypeSafe directly
  or at a local server), else the person's `OPEN_ROUTER_API_KEY`, else off.
  Never the broker: a seat never supplies a key, so the decision model is
  not part of Pro.
- **On a company vendor key, only an explicit decision key.** A gateway
  that runs on `ANTHROPIC_API_KEY` or a company AI gateway sends nothing to
  OpenRouter unless a decision key is set; decisions are off.
- **Decisions only.** The key is never used for a chat completion through
  this path; chat goes through a provider (ADR-0020).
- **The contract.** `pkg/decision` is the slot: typed questions, a
  probability per option; an unavailable decision is a reason the caller
  handles, never a fallback to the chat model.
- **No on/off switch.** The key decides. The two settings read per call,
  `decision_tool_routing` and `decision_message_gate`, default to on.

## What it decides

- the tools a turn gets (`decision_tool_routing`, `gateway/turn_effort.go`
  for the reasoning effort when the window is on `auto`);
- whether a message in a channel is for the agent (`decision_message_gate`);
- which lines of a grep, a read or a log matter (`jgrep`, `jread`, `jlogs`);
- `memdoor savings` counts the judged reads, which is the proof the
  decisions run.

## Consequences

- **A per-token bill the person pays**, small (a judged grep is a fraction
  of a cent) and on the same key as their coding.
- **Text leaves the machine** to the decision model's vendor: the state
  judged (code hunks, log lines, the task). Without a key nothing does, and
  every consumer behaves as before the decision model existed: tools return
  their unjudged output, routing and the gate do nothing.
- **No fallback by design.** When the vendor is down, decisions answer
  `unavailable` and the harness runs as it did before.
