// Package webembed exposes the built Vite/React frontend as an
// embed.FS so the Go binary can serve the web UI without requiring
// node, npm, npx, or any on-disk web/dist directory at runtime.
//
// The companion CLI command (`memdoor web start`, see
// cmd/cli/cmd/web.go) hands this FS to net/http.FileServer with an
// SPA-style fallback to index.html.
//
// Build invariant: web/dist/ must exist and be the current
// `vite build` output at compile time, otherwise `//go:embed` errors
// out the Go build. The Makefile's `web-build` step enforces this on
// fresh clones (web/dist/ is .gitignore'd).
//
// The `all:` prefix on the embed directive keeps files whose names
// start with `.` or `_` (Vite occasionally emits dotfiles in
// assets/, and we don't want those silently dropped).
package webembed

import "embed"

//go:embed all:dist
var Assets embed.FS
