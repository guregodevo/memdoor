// Package webembed exposes the built Vite/React frontend as an
// embed.FS so the Go binary can serve the web UI without requiring
// node, npm, npx, or any on-disk web/dist directory at runtime.
//
// The companion CLI command (`memdoor web start`, see
// cmd/cli/cmd/web.go) hands this FS to net/http.FileServer with an
// SPA-style fallback to index.html.
//
// web/dist/ is .gitignore'd except web/dist/.keep, which is tracked so a
// fresh clone or `git worktree add` compiles with `go build ./...`
// before any `vite build` (the gateway then serves a page that says the
// web UI was not built, gateway/web_bundle.go). The Makefile's
// `web-build` step fills it and touches .keep again after vite empties
// the folder.
//
// The `all:` prefix on the embed directive keeps files whose names
// start with `.` or `_` (Vite occasionally emits dotfiles in
// assets/, and we don't want those silently dropped).
package webembed

import "embed"

//go:embed all:dist
var Assets embed.FS
