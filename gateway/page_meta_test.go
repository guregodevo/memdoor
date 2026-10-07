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
	pricing := string(r.For("/pricing"))
	for _, want := range []string{
		"<title>Pricing — Memdoor: the agent is free, Pro is $10 a month, Enterprise by invoice</title>",
		`<link rel="canonical" href="https://memdoor.ai/pricing" />`,
		`<meta property="og:url" content="https://memdoor.ai/pricing" />`,
		"<h1>Pricing — Memdoor",
	} {
		if !strings.Contains(pricing, want) {
			t.Fatalf("/pricing lacks %q", want)
		}
	}
	if strings.Contains(pricing, "home text") || strings.Contains(pricing, "Home description") {
		t.Fatal("/pricing still carries the home's text")
	}
	doc := string(r.For("/docs/workflows/"))
	if !strings.Contains(doc, "<title>Workflows — Memdoor docs</title>") || !strings.Contains(doc, `content="A workflow is a graph of tasks. Each one has a target."`) {
		t.Fatalf("a doc page takes its heading and first paragraph: %s", doc[:300])
	}
	cmp := string(r.For("/compare/pi"))
	if !strings.Contains(cmp, "<title>Memdoor vs pi: an open-source alternative on your own API key</title>") {
		t.Fatal("a comparison page names its rival")
	}
	if string(r.For("/no-such-route")) != sampleIndex {
		t.Fatal("an unknown route gets the home head (the catch-all answers 404 for it anyway)")
	}
	if &r.For("/pricing")[0] != &r.For("/pricing")[0] {
		t.Fatal("a route is rendered once and kept")
	}
}
