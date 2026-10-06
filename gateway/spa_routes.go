package gateway

import "strings"

// The routes the web app renders (web/src/App.tsx). Anything else the
// catch-all serves with a 404 status, so a crawler files it as gone instead
// of as another copy of the home page.
var spaRoutes = map[string]bool{
	"/": true, "/devs": true, "/docs": true, "/download": true, "/features": true,
	"/forgot-password": true, "/login": true, "/pricing": true, "/reset-password": true,
	"/setup": true, "/verify-email": true, "/workflows": true,
	"/extension": true, "/extension/install": true, "/extension/connect": true,
}

var spaPrefixes = []string{"/docs/", "/r/", "/s/", "/topup", "/pro/"}

// legacyRedirects are URLs the engines still hold from earlier versions of
// the site, sent where the same thing lives now (Google showed
// /how-it-works with a snippet from the fitness era, 2026-10-06).
var legacyRedirects = map[string]string{
	"/how-it-works":                  "/docs/how-it-works",
	"/getting-started":               "/docs/getting-started",
	"/docs/creators/getting-started": "/docs/getting-started",
}

func isSPARoute(p string) bool {
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		p = "/"
	}
	if spaRoutes[p] {
		return true
	}
	for _, pre := range spaPrefixes {
		if strings.HasPrefix(p+"/", pre) || strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}
