package gateway

import (
	"html"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"sync"
)

// The site is one HTML page. Until 2026-10-07 the gateway sent the home's
// title, description, canonical and crawler text for every route and only the
// browser's JavaScript swapped them; Google crawled /pricing and /workflows and
// filed them "crawled, currently not indexed" — copies of the home. The server
// now writes each route's own head and fallback text into the HTML it sends.
// The words are the same ones web/src/meta.ts and the pages set client-side.

const siteURL = "https://memdoor.ai"

type pageMeta struct{ Title, Description, Path string }

var homeMeta = pageMeta{
	Title:       "Memdoor — AI coding agent in your terminal, on your own API key",
	Description: "A coding agent for the terminal on your own API key (OpenRouter, Anthropic, OpenAI…). Describe the steps; it runs them as a checked, resumable workflow. Free.",
	Path:        "/",
}

var routeMetas = map[string]pageMeta{
	"/pricing": {
		Title:       "Pricing — Memdoor: the agent is free, Pro is $10 a month, Enterprise by invoice",
		Description: "The coding agent, the decision model and workflows are free on your own provider key. Pro, $10 a month: remote control from your phone and workflows across machines. Enterprise: the same served for your company with full support.",
	},
	"/features": {
		Title:       "Features — Memdoor: workflows, judged reads, every coding-agent table stake",
		Description: "What Memdoor does: workflows with a proof per step and a gate, a decision model that cuts what the agent reads, and every feature the other coding agents document, with who else has it.",
	},
	"/workflows": {
		Title:       "Workflows — a catalog of agent workflows that have run, each with its real session",
		Description: "Every entry is a workflow that exists as task files and has run: ship, review, tui-check, issue-to-pr, release-check, digest, karpathy-recipe. A graph of steps with a proof per step and a gate where a person decides, recorded live.",
	},
}

var compareRivals = map[string]string{
	"claude-code": "Claude Code", "codex-cli": "Codex CLI", "gemini-cli": "Gemini CLI", "aider": "Aider", "pi": "pi",
}

// routeMeta is the head of a route: a known page, a comparison, a doc (its
// first heading and first paragraph, as the client's docMeta reads them), or
// the home for anything else.
func routeMeta(p string, dist fs.FS) pageMeta {
	p = strings.TrimSuffix(path.Clean("/"+strings.TrimPrefix(p, "/")), "/")
	if p == "" {
		return homeMeta
	}
	if m, ok := routeMetas[p]; ok {
		m.Path = p
		return m
	}
	if name, ok := compareRivals[strings.TrimPrefix(p, "/compare/")]; ok && strings.HasPrefix(p, "/compare/") {
		return pageMeta{
			Title:       "Memdoor vs " + name + ": an open-source alternative on your own API key",
			Description: name + " and Memdoor side by side: the features both document, what only Memdoor has (workflows with a target per step, a decision model), and when to pick each.",
			Path:        p,
		}
	}
	if p == "/docs" || strings.HasPrefix(p, "/docs/") {
		slug := strings.TrimPrefix(strings.TrimPrefix(p, "/docs"), "/")
		if slug == "" {
			slug = "README"
		}
		if md, err := fs.ReadFile(dist, "docs/"+slug+".md"); err == nil {
			return docMetaFromMarkdown(string(md), p)
		}
	}
	return homeMeta
}

// docMetaFromMarkdown mirrors web/src/meta.ts docMeta: the first H1 as the
// title, the first paragraph after it as the description, cut at a sentence
// end near 160 characters.
func docMetaFromMarkdown(md, p string) pageMeta {
	lines := strings.Split(md, "\n")
	h1, para, seen := "Docs", "", false
	for _, l := range lines {
		if strings.HasPrefix(l, "# ") {
			if !seen {
				h1 = strings.TrimSpace(l[2:])
				seen = true
			}
			continue
		}
		if !seen {
			continue
		}
		t := strings.TrimSpace(l)
		if t == "" {
			if para != "" {
				break
			}
			continue
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "!") || strings.HasPrefix(t, "```") || strings.HasPrefix(t, ">") {
			continue
		}
		if para != "" {
			para += " "
		}
		para += t
	}
	para = strings.NewReplacer("`", "", "*", "", "_", "").Replace(para)
	para = mdLink.ReplaceAllString(para, "$1")
	if len(para) > 160 {
		cut := para[:160]
		end := strings.LastIndex(cut, ". ")
		if i := strings.LastIndex(cut, "; "); i > end {
			end = i
		}
		if end > 60 {
			para = cut[:end+1]
		} else {
			if i := strings.LastIndex(cut, " "); i > 0 {
				cut = cut[:i]
			}
			para = cut + "…"
		}
	}
	if para == "" {
		para = homeMeta.Description
	}
	return pageMeta{Title: h1 + " — Memdoor docs", Description: para, Path: p}
}

var (
	mdLink        = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	reTitle       = regexp.MustCompile(`<title>[^<]*</title>`)
	reDescription = regexp.MustCompile(`(<meta name="description" content=")[^"]*(")`)
	reOGTitle     = regexp.MustCompile(`(<meta property="og:title" content=")[^"]*(")`)
	reOGDesc      = regexp.MustCompile(`(<meta property="og:description" content=")[^"]*(")`)
	reOGURL       = regexp.MustCompile(`(<meta property="og:url" content=")[^"]*(")`)
	reTwTitle     = regexp.MustCompile(`(<meta name="twitter:title" content=")[^"]*(")`)
	reTwDesc      = regexp.MustCompile(`(<meta name="twitter:description" content=")[^"]*(")`)
	reCanonical   = regexp.MustCompile(`(<link rel="canonical" href=")[^"]*(")`)
	reFallback    = regexp.MustCompile(`(?s)(<main class="prerender-fallback"[^>]*>).*?(</main>)`)
)

// renderIndex is the HTML page with this route's head and crawler text.
func renderIndex(index []byte, m pageMeta) []byte {
	if m.Path == "/" || m.Path == "" {
		return index
	}
	url := siteURL + m.Path
	t, d := html.EscapeString(m.Title), html.EscapeString(m.Description)
	s := string(index)
	s = reTitle.ReplaceAllLiteralString(s, "<title>"+t+"</title>")
	s = reDescription.ReplaceAllString(s, "${1}"+d+"${2}")
	s = reOGTitle.ReplaceAllString(s, "${1}"+t+"${2}")
	s = reOGDesc.ReplaceAllString(s, "${1}"+d+"${2}")
	s = reOGURL.ReplaceAllString(s, "${1}"+url+"${2}")
	s = reTwTitle.ReplaceAllString(s, "${1}"+t+"${2}")
	s = reTwDesc.ReplaceAllString(s, "${1}"+d+"${2}")
	s = reCanonical.ReplaceAllString(s, "${1}"+url+"${2}")
	s = reFallback.ReplaceAllString(s, "${1}<h1>"+t+"</h1><p>"+d+"</p><p><a href=\"/\">Memdoor</a> · <a href=\"/docs/getting-started\">Docs</a> · <a href=\"/pricing\">Pricing</a></p>${2}")
	return []byte(s)
}

// indexRenderer keeps one rendering per route; the page never changes while
// the gateway runs.
type indexRenderer struct {
	index []byte
	dist  fs.FS
	cache sync.Map
}

func (r *indexRenderer) For(p string) []byte {
	m := routeMeta(p, r.dist)
	key := m.Path
	if key == "" {
		key = "/"
	}
	if v, ok := r.cache.Load(key); ok {
		return v.([]byte)
	}
	out := renderIndex(r.index, m)
	r.cache.Store(key, out)
	return out
}
