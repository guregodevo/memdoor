# Where your data goes

For a security reviewer: every host the `memdoor` binary can name, what it is,
and when it is contacted. `gateway/egress_test.go` fails the build when the
binary names a host this page does not list, so the table is complete by
construction (2026-10-02).

Memdoor is one static binary on the developer's machine. Files, sessions,
memory, the project index and the commands the agent runs stay there. What
leaves is what a model has to read: the prompt and the excerpts the agent
chose. It goes to one of the places below, chosen by configuration, never
mixed:

| Mode | Set by | Where prompts go | Nothing else is contacted? |
|---|---|---|---|
| **Your company's AI gateway** | `AI_GATEWAY_BASE_URL` + `AI_GATEWAY_TOKEN` (or any `*_AI_GATEWAY_TOKEN`: a personal or service-account token) + `MEMDOOR_MODEL` = the provider slug | `POST <base>/v1/responses` on that gateway (the OpenAI Responses shape; `AI_GATEWAY_API=chat` for chat/completions) | **Yes.** Nothing but the gateway. Comes before any vendor key. |
| **Your company's vendor key** | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `XAI_API_KEY`, `BASETEN_API_KEY`, `GROQ_API_KEY` or `DEEPSEEK_API_KEY` (+ `*_BASE_URL` for a proxy in front) | That vendor, or the proxy in front of it | **Yes.** No OpenRouter, no memdoor.ai (it never sees a model request), no catalogue, no web search. `MEMDOOR_VENDOR_HEADERS` adds attribution headers to every request. |
| Your own OpenRouter key | `OPENROUTER_API_KEY` | OpenRouter, with `data_collection: deny` and hosts that train on inputs excluded; a `:free` model is the exception, pinned or the one an account with no credit answers on (`allow`, warned at the pin or the connect) | OpenRouter also serves the model catalogue and web search. |
| A provider you connected | `memdoor connect`, kept in `~/.memdoor/providers.json` | That provider | Used when no key above is set, or when you pin one of its models. A host that lists apps (Vercel AI Gateway, LLM Gateway, EUrouter, Requesty) gets the same app headers as OpenRouter, Google its `x-goog-api-client`; the rest see only the User-Agent (`pkg/attribution`). |
| Your ChatGPT plan | `memdoor connect chatgpt` (Sign in with ChatGPT, in the browser; the tokens kept encrypted in `~/.memdoor/chatgpt-login.json`) | OpenAI's Responses API, with your plan's bearer token, `store: false` | Used like a connected provider. No API key, no client secret; `memdoor connect --remove chatgpt` signs out. |

No memdoor.ai call carries a prompt. The hosts table lists each call.

## Every host the binary names

| Host | What | Contacted when |
|---|---|---|
| *(your gateway's host)* | The company's own AI gateway — not a literal in the binary; it comes from `AI_GATEWAY_BASE_URL` | Company-gateway mode, every model call |
| *(a box of yours, over ssh)* | The ssh door: `--gateway ssh://you@box` runs your own `ssh`, with its keys and config, to tunnel to the gateway on that box; over the same connection it reads the login that gateway generated for itself (`~/.memdoor/local-login` there). Not a literal in the binary | Only when you name the box, for the length of the command; nothing goes anywhere else |
| `api.anthropic.com` | Anthropic's Messages API | Company-key mode with `ANTHROPIC_API_KEY`; `ANTHROPIC_BASE_URL` replaces it with the company gateway |
| `auth.openai.com` | Sign in with ChatGPT: the authorize page (opened in your browser), the token endpoint, the signing keys (JWKS) | Only during `memdoor connect chatgpt`, and when the plan's access token is renewed (hourly, with the refresh token) |
| `api.openai.com` | OpenAI's Responses API on an `OPENAI_API_KEY` (since 2026-10-10; the chat endpoint refuses the gpt-6 models with tools); a signed-in ChatGPT plan's Responses requests and its model list | Company-key mode with `OPENAI_API_KEY`; `OPENAI_BASE_URL` replaces it. A ChatGPT plan: every model call. Also a row in the built-in model table (`pkg/domain/models.go`), never a call by itself |
| `generativelanguage.googleapis.com` | Gemini's OpenAI-compatible endpoint | Company-key mode with `GEMINI_API_KEY`; `GEMINI_BASE_URL` replaces it |
| `api.x.ai` | xAI's Grok API (OpenAI-compatible) | Company-key mode with `XAI_API_KEY`; `XAI_BASE_URL` replaces it |
| `inference.baseten.co` | Baseten's inference API (OpenAI-compatible) | Company-key mode with `BASETEN_API_KEY`; `BASETEN_BASE_URL` replaces it |
| `api.groq.com` | Groq's inference API (OpenAI-compatible) | Company-key mode with `GROQ_API_KEY`; `GROQ_BASE_URL` replaces it |
| `api.deepseek.com` | DeepSeek's API (OpenAI-compatible) | Company-key mode with `DEEPSEEK_API_KEY`; `DEEPSEEK_BASE_URL` replaces it |
| `accounts.google.com`, `oauth2.googleapis.com`, `www.googleapis.com` | Sign in with Google on the web sign-in page: the consent page (the browser goes there), the token endpoint, the signing keys (JWKS) | Only on a gateway with `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` set, and only while someone clicks that button. Off on memdoor.ai (`MEMDOOR_HOSTED=off`). Never from the CLI or a model call |
| `github.com`, `api.github.com` | Sign in with GitHub on the web sign-in page: the authorize page, the token endpoint, the signed-in user's name and verified e-mail | Only on a gateway with `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` set, and only while someone clicks that button. Off on memdoor.ai (`MEMDOOR_HOSTED=off`). `github.com` is also the address of the source and the install one-liners in `--help` text. `memdoor connect local` reads janis's latest release from `github.com/guregodevo/janis/releases`: the `.sha256` of this machine's build, then the installer when you pick janis or the janis there is older. Only when no model server answers on this machine |
| `openrouter.ai` | Chat on your own OpenRouter key (each request names the app — `X-Title: Memdoor`, `HTTP-Referer: https://memdoor.ai` — for OpenRouter's apps page; never the person. Every request to every provider carries `User-Agent: Memdoor/<version> (+https://memdoor.ai)`; which other hosts read an app header is `pkg/attribution`, researched in docs/features/PROVIDERS.md); its model catalogue (`/model`, `/api/models`); web search; the decisions endpoint on a decision key; Sign in with OpenRouter (`memdoor connect openrouter-login`, `/connect`): the browser opens `openrouter.ai/auth`, and one POST to `/api/v1/auth/keys` exchanges the code for the person's own key; `/api/v1/key` (on that key, when the engine is chosen) says whether the account has credit, a free one answering on a `:free` model | Only with `OPENROUTER_API_KEY` (or `OPEN_ROUTER_API_KEY`) set and no company key, or when the person signs in with OpenRouter. Never in company-key mode (`byok_catalog.go` refuses without a key) |
| `memdoor.ai` | The installers (`install.sh`, `install.ps1`), the version check and the binaries (`/dl/VERSION`, `/dl/SHA256SUMS`, `/dl/memdoor-…`; `memdoor upgrade`), opt-in telemetry (`/api/telemetry`) | The installers and `memdoor upgrade`: only when you run them. Telemetry: only with `MEMDOOR_TELEMETRY_ENABLED=1` (default off) |
| `api.typesafe.ai` | The decision model, direct | Only with `MEMDOOR_SYSTEMONE_API_KEY` set on the gateway (self-service decisions) |
| `models.dev` | The public model catalogue (context window and output cap per model), read when a provider's own list states no window | At most once a day, outside company-key mode only; kept in `~/.memdoor/models-dev.json`, which a company-key gateway reads without contacting the host. Carries no prompt |
| `registry.modelcontextprotocol.io` | The MCP server registry | Only `/mcp search` |
| `mcp.linear.app` | An example URL in the `/mcp` panel's help text | Never contacted by the binary; shown as an example |
| `ai-gateway.example.com` | The prefilled example in `memdoor connect`'s base-URL prompt | Never contacted; replaced by what you type |
| `api.stripe.com`, `api.resend.com` | Stripe, transactional e-mail | Only the `memdoor-billing` service on memdoor.ai; never from a developer's machine |

The decision model is a hosted model: Jev, by TypeSafe. It answers the typed
questions (is this search hit relevant, is this turn still making progress)
with a probability. What leaves the machine to it: the search hit, the file
section or the log lines being judged. Path: OpenRouter on your OpenRouter
key, or direct on a decision key. On a vendor key or a company gateway alone
it is off; every read returns unjudged. `memdoor connect typesafe` turns it
on with your key.

## Verifying a download

| File | Covers | Checked by |
|---|---|---|
| `https://memdoor.ai/dl/SHA256SUMS` | A corrupted or swapped binary. A host that serves both the manifest and the binaries can change both. | `install.sh`, `install.ps1`, always |
| `https://memdoor.ai/dl/SHA256SUMS.minisig` | A manifest changed on the host. The private key is not on the server. | `install.sh`, when `minisign` is installed |

Public key: `RWSK9n9HNsGeFjIZtMEamdZ6jMRnnmMCi/4lAzMOy65pfOxvDa5I631B`

```bash
curl -fsSLO https://memdoor.ai/dl/SHA256SUMS
curl -fsSLO https://memdoor.ai/dl/SHA256SUMS.minisig
minisign -Vm SHA256SUMS -P RWSK9n9HNsGeFjIZtMEamdZ6jMRnnmMCi/4lAzMOy65pfOxvDa5I631B
```

A bad signature refuses the install. A release without a signature, or a machine without `minisign`, installs on the checksum alone and says so. `MEMDOOR_REQUIRE_SIGNATURE=1` refuses both. `install.ps1` checks the checksum only. Releases made before signing began have no signature.

## Attribution headers

`MEMDOOR_ATTRIBUTION="user=…;team=…"` and `MEMDOOR_ATTRIBUTION_HEADERS=
"user=X-Email;team=X-Team"` put the person's fields on every vendor and
gateway request under the names the company's gateway expects. The project's
directory name and the conversation's session id travel the same way
(`X-Memdoor-Project`, `X-Memdoor-Session` unless renamed). No prompt content.
Model endpoint only.

## Providers you add

`memdoor connect` keeps a provider in `~/.memdoor/providers.json` (0600, the
directory 0700). The credential is stored as given: a literal, the NAME of an
environment variable, or `!command`, so a secret in your company's tooling is
never copied onto disk. The gateway probes a provider (its model list, one
small call) before keeping it and contacts it afterwards only when one of its
models is pinned or its list is read. A local runtime (`memdoor connect
local`: janis on this machine or on the box, or Ollama, llama.cpp, LM Studio) is
kept with no credential; its requests go to the address you gave, normally
localhost.

## Where the gateway listens

This machine and its network, port 18789, since 2026-10-23. A phone on the
same network reaches a `/remote` link on the gateway with no relay. Every
door takes a token; `/health` and the sealed relay pages are the only
unauthenticated ones. The first account can only be made from the machine; a
request from the network is refused. Plain HTTP: a network you trust, or
behind ssh, Tailscale or a reverse proxy. `MEMDOOR_HOST=localhost` (or
`--host localhost`) keeps it to this machine; a company preset can set that.

For a `/remote` link the gateway's process is the terminal end of the relay
(since 2026-10-23), so the link outlives the window that made it. State in
`~/.memdoor/remote-hosted.json` on the gateway's machine, mode 0600, nowhere
else: the link's id and key, the gateway session token of the person who
turned it on (the gateway follows and posts as them), and where the link
lives. `/remote off` deletes the entry.

## No "yolo" mode

Default: always-auto. `MEMDOOR_APPROVE=changes` (or the workspace setting
`approve`) makes every command, file write, MCP tool call, and workflow
start or resume wait for the person's yes in the terminal; no answer in five
minutes is a no. A workflow approval covers the delegated execution of that
invocation: its steps may run commands, change files, and make model calls.
Workflow execution asks again for each invocation and has no session-wide
"always" grant. List, status, and Stop do not ask.

An agent asking to approve a human workflow gate always needs a fresh,
explicit Yes, even with approval mode off. The prompt names the run and gate,
defaults to No, and cannot be satisfied by a previous workflow approval.
Approving the gate may start dependent steps. A person using the CLI or the
terminal's approval control directly is already giving that authorization.

Implementation: `gateway/approval.go` and `gateway/workflow_approval.go`.
Rules that refuse a call outright
(`memdoor guards`: never `.env`, never `git push`):
[Boundaries](/docs/boundaries).

## What a company reviewer can check

- Set the three variables, start `memdoor gateway` and read its first lines:
  `your company's <vendor> key serves this gateway; nothing else is
  contacted`. `memdoor logs query --regex "LLM request"` shows every model
  call with the model and the message count. No other host appears in the
  log.
- `MEMDOOR_VENDOR=off` is the only way out of company-key mode: an
  environment variable the company's preset controls. The same preset can set
  `MEMDOOR_APPROVE=changes`, which wins over the workspace setting.
- The decision model, the catalogue and web search are not degraded silently.
  The agent reads more, as without a decision key or an OpenRouter key. The
  catalogue is the one model the key names. `web_search` answers that it has
  no key.
- `go test ./gateway/ -run TestEveryHostTheBinaryNamesIsDocumented` proves
  the table is complete for the build in hand.
