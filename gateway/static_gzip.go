package gateway

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

// gzipStatic serves the embedded site compressed ONCE per file. nginx can
// gzip on the fly, but the landing's bundle is 1 MB and compressing it per
// request made the box CPU-bound at ~30 requests/s (measured 2026-10-05
// before the launch); a file that never changes is compressed the first time
// it is asked for and the bytes are kept. nginx leaves a response that
// already carries Content-Encoding alone.
//
// The cache headers belong here too, not only in nginx: a hashed asset is
// immutable, a recording changes rarely, and a gateway served without nginx
// (a developer's own) should say the same.
type gzipStatic struct {
	fsys  fs.FS
	plain http.Handler
	mu    sync.Mutex
	gz    map[string][]byte
}

func newGzipStatic(fsys fs.FS, plain http.Handler) *gzipStatic {
	return &gzipStatic{fsys: fsys, plain: plain, gz: map[string][]byte{}}
}

// compressible is decided by extension: text, not images or binaries.
var compressible = map[string]bool{
	".js": true, ".css": true, ".html": true, ".svg": true, ".json": true,
	".cast": true, ".md": true, ".txt": true, ".xml": true, ".map": true,
}

const gzipMinBytes = 1024

func (g *gzipStatic) cacheControl(p string) string {
	switch {
	case strings.HasPrefix(p, "assets/"):
		return "public, max-age=31536000, immutable"
	case strings.HasSuffix(p, ".cast"):
		return "public, max-age=86400"
	}
	return ""
}

func (g *gzipStatic) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if cc := g.cacheControl(p); cc != "" {
		w.Header().Set("Cache-Control", cc)
	}
	ext := path.Ext(p)
	if !compressible[ext] || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		g.plain.ServeHTTP(w, r)
		return
	}
	body, ok := g.compressed(p)
	if !ok {
		g.plain.ServeHTTP(w, r)
		return
	}
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "text/plain; charset=utf-8"
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("Content-Encoding", "gzip")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	h.Add("Vary", "Accept-Encoding")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(body)
}

// compressed is the gzip of a file, made once. false when the file is
// missing, a directory, or too small to be worth it.
func (g *gzipStatic) compressed(p string) ([]byte, bool) {
	g.mu.Lock()
	body, seen := g.gz[p]
	g.mu.Unlock()
	if seen {
		return body, body != nil
	}
	raw, err := fs.ReadFile(g.fsys, p)
	if err != nil || len(raw) < gzipMinBytes {
		g.remember(p, nil)
		return nil, false
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	body = buf.Bytes()
	g.remember(p, body)
	return body, true
}

func (g *gzipStatic) remember(p string, body []byte) {
	g.mu.Lock()
	g.gz[p] = body
	g.mu.Unlock()
}
