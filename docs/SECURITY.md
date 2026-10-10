# Where your data goes

The page for a security reviewer: every host the `memdoor` binary can name,
what it is, and when — if ever — it is contacted. `gateway/egress_test.go`
fails the build when the binary names a host this page does not list, so
the table is complete by construction (2026-10-02).

Memdoor is one static binary running on the developer's machine. Files,
sessions, memory, the index of the project and every command the agent runs
stay on that machine. What leaves is what a model has to read — the prompt
and the excerpts the agent chose — and it goes to exactly one of three
places, chosen by configuration, never mixed:

| Mode | Set by | Where prompts go | Nothing else is contacted? |
|---|---|---|---|
| **Your company's AI gateway** | `AI_GATEWAY_BASE_URL` + `AI_GATEWAY_TOKEN` (or any `*_AI_GATEWAY_TOKEN`: a personal or service-account token) + `MEMDOOR_MODEL` = the provider slug | `POST <base>/v1/responses` on that gateway (the OpenAI Responses shape; `AI_GATEWAY_API=chat` for chat/completions) | **Yes.** Nothing but the gateway. Comes before any vendor key. |
| **Your company's vendor key** | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `XAI_API_KEY`, `BASETEN_API_KEY`, `GROQ_API_KEY` or `DEEPSEEK_API_KEY` (+ `*_BASE_URL` for a proxy in front) | That vendor, or the proxy in front of it | **Yes.** No OpenRouter, no memdoor.ai broker, no catalogue, no web search. `MEMDOOR_VENDOR_HEADERS` adds attribution headers to every request. |
| Your own OpenRouter key | `OPEN_ROUTER_API_KEY` | OpenRouter, with `data_collection: deny` and hosts that train on inputs excluded; a pinned `:free` model is the exception (`allow`, warned at the pin) | OpenRouter also serves the model catalogue and web search. |
| A provider you connected | `memdoor connect`, kept in `~/.memdoor/providers.json` | That provider | Used when no key above is set, or when you pin one of its models. |
| Your ChatGPT plan | `memdoor connect chatgpt` (Sign in with ChatGPT, in the browser; the tokens kept encrypted in `~/.memdoor/chatgpt-login.json`) | OpenAI's Responses API, with your plan's bearer token, `store: false` | Used like a connected provider. No API key, no client secret; `memdoor connect --remove chatgpt` signs out. |

A Pro seat changes nothing about where prompts go: it never supplies a model
key. No memdoor.ai call carries a prompt in the clear (the hosts table below
lists them; the remote-control relay is sealed end to end).

## Every host the binary names

| Host | What | Contacted when |
|---|---|---|
| *(your gateway's host)* | The company's own AI gateway — not a literal in the binary; it comes from `AI_GATEWAY_BASE_URL` | Company-gateway mode, every model call |
| `api.anthropic.com` | Anthropic's Messages API | Company-key mode with `ANTHROPIC_API_KEY`; `ANTHROPIC_BASE_URL` replaces it with the company gateway |
| `auth.openai.com` | Sign in with ChatGPT: the authorize page (opened in your browser), the token endpoint, the signing keys (JWKS) | Only during `memdoor connect chatgpt`, and when the plan's access token is renewed (hourly, with the refresh token) |
| `api.openai.com` | OpenAI's API (OpenAI-compatible client); a signed-in ChatGPT plan's Responses requests and its model list | Company-key mode with `OPENAI_API_KEY`; `OPENAI_BASE_URL` replaces it. A ChatGPT plan: every model call. Also a row in the built-in model table (`pkg/domain/models.go`), never a call by itself |
| `generativelanguage.googleapis.com` | Gemini's OpenAI-compatible endpoint | Company-key mode with `GEMINI_API_KEY`; `GEMINI_BASE_URL` replaces it |
| `api.x.ai` | xAI's Grok API (OpenAI-compatible) | Company-key mode with `XAI_API_KEY`; `XAI_BASE_URL` replaces it |
| `inference.baseten.co` | Baseten's inference API (OpenAI-compatible) | Company-key mode with `BASETEN_API_KEY`; `BASETEN_BASE_URL` replaces it |
| `api.groq.com` | Groq's inference API (OpenAI-compatible) | Company-key mode with `GROQ_API_KEY`; `GROQ_BASE_URL` replaces it |
| `api.deepseek.com` | DeepSeek's API (OpenAI-compatible) | Company-key mode with `DEEPSEEK_API_KEY`; `DEEPSEEK_BASE_URL` replaces it |
| `openrouter.ai` | Chat on your own OpenRouter key (each request names the app — `X-Title: Memdoor`, `HTTP-Referer: https://memdoor.ai` — for OpenRouter's apps page; never the person); its model catalogue (`/model`, `/api/models`); web search; the decisions endpoint on a decision key | Only with `OPEN_ROUTER_API_KEY` set and no company key. Never in company-key mode (`byok_catalog.go` refuses without a key) |
| `memdoor.ai` | Billing and sign-in (`memdoor login`, `memdoor account`), the `/remote` relay (sealed end to end), the hosted workflow state (`/state`, Pro: a run's task names, statuses and outputs' names, never your code), `/share` links, the upgrade check (`memdoor upgrade`), opt-in telemetry | Sign-in, account status, `/remote`, `/share`, `memdoor upgrade`: only when you run them. Telemetry: only with `MEMDOOR_TELEMETRY_ENABLED=1` (default off). Also appears as e-mail addresses (`hello@`, `noreply@`) in the seat-request and billing code that runs on memdoor.ai itself |
| `api.typesafe.ai` | The decision model, direct | Only with `MEMDOOR_SYSTEMONE_API_KEY` set on the gateway (self-service decisions) |

**The decision model is a hosted model.** Jev, by TypeSafe, answers the typed
questions (is this search hit relevant, is this turn still making progress)
with a probability; what it is asked about — the search hit, the file section,
the log lines being judged — leaves the machine to it, through OpenRouter on
your OpenRouter key or directly on a decision key. On a vendor key or a company
gateway alone it is **off**: nothing is judged off-machine, every read returns
unjudged, the agent works without the savings. `memdoor connect typesafe` turns
it on with a key of your own.
| `models.dev` | The public model catalogue (context window and output cap per model), read when a provider's own list states no window | At most once a day, outside company-key mode only; kept in `~/.memdoor/models-dev.json`, which a company-key gateway reads without contacting the host. Carries no prompt |
| `registry.modelcontextprotocol.io` | The MCP server registry | Only `/mcp search` |
| `mcp.linear.app` | An example URL in the `/mcp` panel's help text | Never contacted by the binary; shown as an example |
| `ai-gateway.example.com` | The prefilled example in `memdoor connect`'s base-URL prompt | Never contacted; replaced by what you type |
| `api.stripe.com`, `api.resend.com` | Stripe, transactional e-mail | Only the `memdoor-billing` service on memdoor.ai; never from a developer's machine |

## Attribution headers

`MEMDOOR_ATTRIBUTION="user=…;team=…"` and `MEMDOOR_ATTRIBUTION_HEADERS=
"user=X-Email;team=X-Team"` put the person's fields on every vendor and
gateway request under the names the company's gateway expects; the
project's directory name and the conversation's session id travel the
same way (`X-Memdoor-Project`, `X-Memdoor-Session` unless renamed). No
prompt content is in them; they go only to the model endpoint.

## Providers you add

`memdoor connect` keeps a provider in `~/.memdoor/providers.json` (0600,
the directory 0700). The credential is stored as given — a literal, the
NAME of an environment variable, or `!command` — so a secret in your
company's tooling is never copied onto disk. The gateway probes a provider
(its model list, one small call) before keeping it, and contacts a provider
you added only when a model of its is pinned or its list is read.

## No "yolo" mode

Memdoor's default is always-auto. `MEMDOOR_APPROVE=changes` (or the
workspace setting `approve`) makes every command, file write and MCP tool
call wait for the person's yes in the terminal; no answer in five minutes
is a no. Reads never ask. `gateway/approval.go`.

## What a company reviewer can check

- Set the three variables, start `memdoor gateway`, and read its first lines:
  `your company's <vendor> key serves this gateway; nothing else is
  contacted`. Then `memdoor logs query --regex "LLM request"` shows every
  model call with the model and the message count, and nothing to any other
  host appears in the log.
- `MEMDOOR_VENDOR=off` is the only way out of company-key mode, and it is
  an environment variable the company's preset controls. The same preset can
  set `MEMDOOR_APPROVE=changes`, which wins over the workspace setting.
- The decision model, the catalogue and web search are not degraded
  silently: the agent reads more instead (the free tier's behaviour), the
  catalogue is the one model the key names, and `web_search` answers that it
  has no key.
- `go test ./gateway/ -run TestEveryHostTheBinaryNamesIsDocumented` is the
  proof this table is complete for the build in hand.
