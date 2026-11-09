package gateway

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestWebBundleServesTheBuiltIndex(t *testing.T) {
	assets := fstest.MapFS{
		"dist/index.html":    {Data: []byte("<html>built</html>")},
		"dist/assets/app.js": {Data: []byte("x")},
	}
	dist, index, ok := webBundle(assets)
	if !ok || string(index) != "<html>built</html>" {
		t.Fatalf("webBundle = %q, %v; want the built index", index, ok)
	}
	if _, err := dist.Open("assets/app.js"); err != nil {
		t.Fatalf("dist is not rooted at dist/: %v", err)
	}
}

func TestWebBundleWithoutIndexServesThePlaceholder(t *testing.T) {
	for name, assets := range map[string]fstest.MapFS{
		"empty":      {},
		"stamp only": {"dist/.stamp": {}},
	} {
		t.Run(name, func(t *testing.T) {
			_, index, ok := webBundle(assets)
			if ok {
				t.Fatal("webBundle reported a web UI with no index.html")
			}
			if !strings.Contains(string(index), "built without the web UI") {
				t.Fatalf("placeholder page = %q", index)
			}
		})
	}
}
