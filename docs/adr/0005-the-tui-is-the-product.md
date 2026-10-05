# ADR 0005 — The TUI is the product; backends are pluggable; the marketplace is a backend

Status: ACCEPTED (2026-09-01, Greg's go-ahead the same evening) · Reframes ADR 0001/0002 · Deliberately reverses
the 2026-08-20 BYOK removal · Affects positioning, `docs/roadmap/MUST.md`,
`gateway/providers`, the billing scope of `gateway/billingsvc`

## Context

Three bodies of evidence converged on 2026-09-01.

**The competition study** ([memory: competition-study-2026-09-01], published
2026-09-01) found the paid GPU-hour lane's sharpest competitor is not the $200
incumbent but the **$18/month open-model coding subscription** (Z.ai GLM
Coding Plan; Kimi K3 from ¥49) — which plugs the same open weights we serve
into Claude Code, Cline, and OpenCode. The market has already decided that
**agents are clients and models are backends**. Per-token open-model APIs
(DeepSeek V4 Pro at $1.30/M input, $0.10 cached, on DeepInfra) externally
confirm the internal arithmetic (`MUST.md` #0): an interactive session idles
~98% of the time, so one-renter-one-GPU pays for 100% to use 2%, and a 3.7×
cheaper supply chain loses to a ~50× worse utilization ratio. The exact wedge
— agent + managed marketplace GPU + hourly meter — is unclaimed *because* the
solo version of it loses on arithmetic; it is only worth standing in with
multi-tenancy (head-on against better-capitalized multiplexers, on their
terms) or with buyers who pay for dedication itself.

**The TUI reached the bar.** The 2026-09-01 receipts: a hybrid
linear-attention 9B running **in-process** at 11.5 tok/s on a 16 GB Mac —
faster than the 8B it replaced — completing fix-existing coding tasks end to
end. No other multi-model client has a built-in local engine; for Cline,
OpenCode, Aider and Roo, "local" means an Ollama/LM Studio integration. For
memdoor it is the floor, in one signed binary, on the locked corporate Mac
where none of the alternatives install.

**The moat generalized.** The same day's coder work showed the durable
software is not GPU babysitting specifically: driving Qwen3.5 with the wrong
tool protocol produced a model that read files and flailed; giving it its
native protocol produced one that diagnosed, patched, and verified. The
ToolProtocol layer, per-family templates, grammar constraints, absorbers and
honesty guards are **software that makes heterogeneous models behave** — the
general form of "software that makes unreliable hardware behave," and per-model
work nobody else in the client field does under a contract rather than
heuristics.

Prior research already pointed here: there is no profitable solo subscription
tier (2026-06-27); the payers are teams, enterprises, and regulated buyers
(2026-06-27); the moat is the same-gateway network effect at N>1 (2026-05-27);
the user picks the model (2026-06-27); Tabnine-class air-gapped demand is
served at ~10× what a marketplace-provisioned deployment would cost.

## Decision

1. **The product is the TUI + gateway** — the harness: sessions, memory,
   skills, multi-seat workspaces, and the model-behavior layer. Free for solo
   use, forever; solo is the adoption wedge, not the revenue line.

2. **Backends are pluggable** behind one provider contract, spanning a dial:
   - *local* — the in-process engine (unique to us; the free floor);
   - *remote endpoint* — OpenAI-compatible per-token APIs and coding plans
     (GLM, Kimi, DeepSeek, DeepInfra-class). This deliberately reverses the
     2026-08-20 BYOK removal: that removal was correct **when the business was
     margin on resold GPU time** — a user-held key bypassed the business. When
     the business is the harness, a backend credential is just a backend.
   - *dedicated* — a marketplace GPU rented and babysat by the broker: the
     privacy/no-throttling notch of the dial, and the provisioning engine for
     enterprise deployments into the customer's own account.

3. **The marketplace is a backend, not the business.** Metered resale of GPU
   hours stops being the revenue model. Revenue is the team gateway (shared
   workspaces, shared agent memory, seats) and enterprise private deployments
   (the broker provisioning into the customer's cloud/marketplace account, at
   cost, paid for as software).

## Consequences

- **Positioning.** We compete as *the client that makes every open model
  behave*, spanning local → plan → dedicated. The hero stops claiming a
  price-per-hour that beats subscriptions (the claim `MUST.md` #0 shows has no
  winning row); "your machine doesn't throttle you" belongs to the dedicated
  notch, not the masthead.
- **The production-grade track shrinks.** If strangers stop holding prepaid
  balances against metered resale, the bounded-liability invariant, crash-only
  money, and the waste-rate solvency bound stop blocking launch (they remain
  correct engineering for the enterprise provisioning engine, later). Real
  accounts, onboarding, TLS, and one-VPS ops remain.
- **Multi-tenancy is demoted** from "decides whether the price claim can be
  made" to an efficiency option for team deployments. We do not out-multiplex
  DeepInfra; we stop needing to.
- **Engineering scope (new):** an OpenAI-compatible backend client in
  `gateway/providers`; ToolProtocol + chat-template entries per adopted family
  (GLM, Kimi, DeepSeek — the qwen35 protocol shipped 2026-09-01 is the
  pattern); credential UX; `/model` spanning backend types (the live-swap
  machinery already exists).
- **Nothing built is discarded.** The broker, reliability layer, catalogue and
  receipts become the dedicated notch and the enterprise engine; the free
  local floor keeps its role unchanged.

## Amended the same evening (2026-09-01): verticals arrive as skills

The decision was stress-tested the night it was written. Greg's direction —
a **multimodel TUI for creative verticals** (video clipping first; game
building judged a weaker fit) — turned out to be this ADR's consequence, not
a competitor to it: a clipping vertical shipped as `~/.memdoor/skills/
clipping.md` alone, zero code, and the TUI + local 9B executed ffmpeg
probe/extract/render mechanics from it the same evening (with babysitting;
each failure became a skill rule). Two additions to the record:

- **Batch work repairs the marketplace economics.** Clipping saturates a GPU
  while it runs and tolerates preemption, so the per-hour dedicated model
  that *loses* for interactive coding is the correct billing unit here. The
  2026-08-24 retraction rejected batch as a *coding* market, not batch
  economics. The "dedicated" notch of the backend dial is therefore
  strongest exactly where the workload is batch.
- **"Multimodel" includes modalities.** The adopted 9B is a multimodal
  checkpoint whose vision tower `llm/qwen35` deliberately skips; loading it
  is the engine step that turns skill-driven video work from
  file-size-heuristic selection into actual frame understanding.

## What would falsify this

- Plan/API terms of service closing to third-party agent clients (GLM's plan
  is *marketed* on such compatibility today — watch it).
- The team gateway failing to convert at N>1 — the network-effect moat is a
  thesis with receipts pending.
- Enterprise buyers demanding metered resale after all (then the billing track
  returns at enterprise scope, not launch scope).
