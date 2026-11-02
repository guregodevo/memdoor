# Roadmap

Memdoor is the workflow harness that makes a small model beat a frontier
one, on your machine. The runs are published. The terminal, a shell, CI,
cron, an editor and a phone are doors onto it.

Decided on 2026-11-02 after the LiveCodeBench Hard thread. The harness is the
product. The people who want it run local models. The other surfaces are
kept and not extended until a user asks.

## Now

1. **The receipt on a local model.** team-solve on Qwen3.8-27B on a 24 GB
   card and on a 32 GB Mac, through Ollama, llama.cpp and janis. Each run
   published like the OpenRouter one. Done when the result page shows each
   setup with its score and its hours.
2. **Ten users.** The people who came back (version checks, issues, PRs):
   what they ran it on, what broke, what they wanted next. Done when ten
   answers are written down.
3. **The first run is a workflow on a local model.** `memdoor connect
   local`, then `memdoor workflow run team-solve --wait` in an empty folder.
   The folder gets the sample problem (built in, PR #11). `pr-review` runs
   in CI from one `uses:` line. Done when the README's first screen and the
   getting-started page open on that and a stranger's first run is on the
   result page.
4. **Lean review on a key.** `review-lean` sets thinking off; a model that
   refuses (HTTP 400, reasoning mandatory) fails the run. Retry without the
   flag. Done when the lean path passes on glm-5.3-flash.
5. **The metric.** Machines that ran the binary again, per day, over 14
   days. No usage ping. Done when `make metrics` leads with it.

## Next

- Hosted workflow state for a team: one table per workspace on memdoor.ai.
  A step on one machine waits on a step from another. Built. Sold when a
  team asks. $10 a month.
- The decision model: message gate on by default, a stop reason on every cut
  reply, "which tool did it mean" before giving up on an unparseable call.
- Provider listings: DeepSeek's agent list, Hugging Face hub docs, Vercel's
  OSS program, OpenAI's Codex for Open Source, the Groq badge.
- SSH peers (`@box:coder`, PR #10) and the editor door (ACP): kept as they
  are.

## Not now

- Enterprise: SSO, SAML, SCIM, audit logs, SOC 2. Built when a deal is
  signed.
- Voice, messaging channels, plugins: dropped.
- A hosted box (memdoor.ai runs the gateway for you): after a team pays for
  the state.

## Moving an item

Every item has a done-when. An item moves to Next or Now on evidence: a user
asked, a run showed it, or a number moved. The item names which.
