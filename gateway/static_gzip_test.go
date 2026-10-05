package gateway

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// TestTheSiteIsCompressedOnceAndCached: a browser that accepts gzip gets the
// bundle compressed, with the immutable cache header, and the second request
// is served from the bytes kept the first time; a client that does not accept
// gzip, and a small or binary file, go through the plain file server.
func TestTheSiteIsCompressedOnceAndCached(t *testing.T) {
	js := []byte(strings.Repeat("const memdoor = 'workflows';\n", 200))
	fsys := fstest.MapFS{
		"assets/index-abc.js": {Data: js},
		"favicon.png":         {Data: bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 400)},
		"review.cast":         {Data: []byte(strings.Repeat(`[0.1,"o","x"]`+"\n", 100))},
		"tiny.js":             {Data: []byte("1")},
	}
	h := newGzipStatic(fsys, http.FileServer(http.FS(fsys)))

	get := func(p, enc string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/"+p, nil)
		if enc != "" {
			req.Header.Set("Accept-Encoding", enc)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	first := get("assets/index-abc.js", "gzip, deflate, br")
	if first.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("the bundle should be gzipped for a client that accepts it: %v", first.Header())
	}
	if first.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("a hashed asset is immutable: %q", first.Header().Get("Cache-Control"))
	}
	if first.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("content type: %q", first.Header().Get("Content-Type"))
	}
	firstBytes := append([]byte(nil), first.Body.Bytes()...)
	zr, err := gzip.NewReader(bytes.NewReader(firstBytes))
	if err != nil {
		t.Fatal(err)
	}
	back, _ := io.ReadAll(zr)
	if !bytes.Equal(back, js) {
		t.Fatal("the compressed body does not decompress to the file")
	}
	if len(firstBytes) >= len(js) {
		t.Fatalf("compression gained nothing: %d >= %d", len(firstBytes), len(js))
	}

	second := get("assets/index-abc.js", "gzip")
	if !bytes.Equal(second.Body.Bytes(), firstBytes) {
		t.Fatal("the second request should be the bytes kept from the first")
	}
	if len(h.gz) != 1 || h.gz["assets/index-abc.js"] == nil {
		t.Fatalf("one file compressed once, kept: %d entries", len(h.gz))
	}

	plain := get("assets/index-abc.js", "")
	if plain.Header().Get("Content-Encoding") != "" || !bytes.Equal(plain.Body.Bytes(), js) {
		t.Fatal("a client without gzip gets the file as is")
	}
	if plain.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatal("the cache header does not depend on the encoding")
	}

	cast := get("review.cast", "gzip")
	if cast.Header().Get("Content-Encoding") != "gzip" || cast.Header().Get("Cache-Control") != "public, max-age=86400" {
		t.Fatalf("a recording is compressed and cached a day: %v", cast.Header())
	}

	png := get("favicon.png", "gzip")
	if png.Header().Get("Content-Encoding") != "" {
		t.Fatal("a PNG is not recompressed")
	}
	tiny := get("tiny.js", "gzip")
	if tiny.Header().Get("Content-Encoding") != "" || tiny.Body.String() != "1" {
		t.Fatal("a file under the minimum is served plain")
	}
	if get("assets/missing.js", "gzip").Code != http.StatusNotFound {
		t.Fatal("a missing file is still a 404")
	}
}
