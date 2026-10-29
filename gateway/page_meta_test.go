package gateway

import (
	"strings"
	"testing"
	"testing/fstest"
)

const sampleIndex = `<html><head><title>Home title</title>
<meta name="description" content="Home description" />
<meta property="og:title" content="Home title" /><meta property="og:description" content="Home description" /><meta property="og:url" content="https://memdoor.ai" />
<link rel="canonical" href="https://memdoor.ai/" />
<meta name="twitter:title" content="Home title" /><meta name="twitter:description" content="Home description" />
</head><body><div id="root"><main class="prerender-fallback" style="x"><h1>Home</h1><p>home text</p></main></div></body></html>`

func TestEveryRouteCarriesItsOwnHead(t *testing.T) {
	dist := fstest.MapFS{"docs/workflows.md": {Data: []byte("# Workflows\n\nA *workflow* is a [graph](x) of tasks. Each one has a target.\n\nMore.\n")}}
	r := &indexRenderer{index: []byte(sampleIndex), dist: dist}

	home := string(r.For("/"))
	if home != sampleIndex {
		t.Fatal("the home is served as built")
	}
	enterprise := string(r.For("/enterprise"))
	for _, want := range []string{
		"<title>Enterprise — Memdoor is open source: self-host it, or let memdoor.ai host the gateway</title>",
		`<link rel="canonical" href="https://memdoor.ai/enterprise" />`,
		`<meta property="og:url" content="https://memdoor.ai/enterprise" />`,
		"<h1>Enterprise — Memdoor",
	} {
		if !strings.Contains(enterprise, want) {
			t.Fatalf("/enterprise lacks %q", want)
		}
	}
	if strings.Contains(enterprise, "home text") || strings.Contains(enterprise, "Home description") {
		t.Fatal("/enterprise still carries the home's text")
	}
	doc := string(r.For("/docs/workflows/"))
	if !strings.Contains(doc, "<title>Workflows — Memdoor docs</title>") || !strings.Contains(doc, `content="A workflow is a graph of tasks. Each one has a target."`) {
		t.Fatalf("a doc page takes its heading and first paragraph: %s", doc[:300])
	}
	cmp := string(r.For("/compare/pi"))
	if !strings.Contains(cmp, "<title>Memdoor vs pi: an open-source alternative on your own API key</title>") {
		t.Fatal("a comparison page names its rival")
	}
	download := string(r.For("/download"))
	if !strings.Contains(download, "<title>Install Memdoor") || !strings.Contains(download, `<link rel="canonical" href="https://memdoor.ai/download" />`) || strings.Contains(download, "noindex") {
		t.Fatal("/download has its own head, indexed")
	}
	for _, p := range []string{"/login", "/welcome", "/forgot-password"} {
		page := string(r.For(p))
		if !strings.Contains(page, `<meta name="robots" content="noindex" />`) || !strings.Contains(page, `<link rel="canonical" href="https://memdoor.ai`+p+`" />`) || strings.Contains(page, "Home description") {
			t.Fatalf("%s is an account page: its own head, noindex", p)
		}
	}
	if string(r.For("/no-such-route")) != sampleIndex {
		t.Fatal("an unknown route gets the home head (the catch-all answers 404 for it anyway)")
	}
	if &r.For("/enterprise")[0] != &r.For("/enterprise")[0] {
		t.Fatal("a route is rendered once and kept")
	}
}
