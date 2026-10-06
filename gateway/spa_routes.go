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
