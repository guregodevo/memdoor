package gateway

import (
	iofs "io/fs"
)

// noWebUIPage answers the web routes of a binary built without `vite build`:
// the API, the TUI and every door work without the React bundle.
const noWebUIPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Memdoor</title></head>
<body><h1>Memdoor</h1>
<p>This gateway was built without the web UI. The API and <code>memdoor tui</code> work.</p>
<p>To serve the web UI, run <code>make web-build</code> and rebuild.</p>
</body></html>
`

// webBundle returns the built frontend under dist/ and its index.html. With
// no index.html it returns the fallback page and false.
func webBundle(assets iofs.FS) (iofs.FS, []byte, bool) {
	dist, err := iofs.Sub(assets, "dist")
	if err != nil {
		return assets, []byte(noWebUIPage), false
	}
	if index, err := iofs.ReadFile(dist, "index.html"); err == nil {
		return dist, index, true
	}
	return dist, []byte(noWebUIPage), false
}
