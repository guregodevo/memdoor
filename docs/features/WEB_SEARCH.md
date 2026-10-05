# Web search

`web_search` looks something up on the web and answers with its sources. The
coder has it, with `web_fetch` to read one of those pages in full. It runs on
the person's own OpenRouter key: no other key, no other account.

## What it does

One call is one request to OpenRouter with its web search server tool
(`{"type": "openrouter:web_search"}`) among the request's tools:

- **The search** is OpenRouter's. The engine is Parallel, the cheapest one
  OpenRouter offers ($0.001-0.005 a search, on top of the tokens). Models
  without a search of their own (GLM, DeepSeek) get it the same way.
- **One search per call** (`max_uses: 1`). Without it, one call ran two
  searches, each billed. The agent calls again for another query.
- **The answer** is written from the results by the first rung of the coder's
  ladder (`gateway/providers/byok.go`).
- **The result** is the answer, then its sources numbered, each once: title,
  URL and an excerpt.

```
Latest LTS: Node.js v24.21.0, released September 7, 2026 …

Sources:
[1] Node.js — Run JavaScript Everywhere — https://nodejs.org/en/download/archive/current
    # Node.js® Download Archive ## Node.js Logo v26.3.0 ## First released …
[2] …
```

`max_results` (default 5, max 10) sets how many results the search returns.

## When it refuses

- **An answer with no source** is refused: it is the model answering from
  memory, not a search. The agent is told to search again or rephrase.
- **No OpenRouter key** (`OPEN_ROUTER_API_KEY`): the tool says so.

## Who has it

- **The coder**: `web_search` and `web_fetch` are in its palette. With a
  decision model, tool routing shows them in the `research` family (look
  something up on the web) and the `investigate` family (an error message or
  a library this machine has no answer for); see [DECIDE.md](DECIDE.md).
- **Any agent** given the `group:web` tools
  ([AUTHORIZATION.md](../reference/AUTHORIZATION.md)).

The embedded `chrome` skill (`skills/chrome.md`) sends searches here and
reading to `web_fetch`, and keeps the browser for pages that need one.

## Why not search through Chrome

The `chrome` skill used to search DuckDuckGo in a headless Chrome. Search
engines answer a headless browser with a bot challenge ("Select all squares
containing a duck", live 2026-09-30), and solving it is not something to
automate. The skill now says to stop at a challenge page.

## Why not the accounts other CLIs log in with

omp borrows the search that other subscriptions include: the ChatGPT Codex
backend with a ChatGPT login, Gemini's Code Assist endpoint with a Gemini
CLI login. Memdoor's users bring an OpenRouter key, and those endpoints are
the vendors' private backends for their own CLIs. What Memdoor keeps from
omp is the refusal of an answer without sources.

## Receipt

Test gateway, 2026-09-30: "What is the latest stable Node.js release right
now?" The coder called `web_search`, then `web_fetch` on
`nodejs.org/dist/index.json`, and answered in 36 s with the LTS and Current
versions and their dates.

Code: `tools/web_search.go`, tests `tools/web_search_test.go`.
