package providers

import (
	"net/http"
	"strings"
)

// redirectSafeHeaders are what a request still carries once a redirect has
// taken it off the host it was made for. Everything else stays behind: the
// key under any name (Authorization, x-api-key), the app headers, a
// company's MEMDOOR_VENDOR_HEADERS and its attribution headers.
var redirectSafeHeaders = map[string]bool{
	"Accept":       true,
	"Content-Type": true,
	"User-Agent":   true,
}

// keepHeadersOnTheirHost is the CheckRedirect of every client that talks to
// a provider. Go drops only Authorization and cookies on a redirect to
// another domain, keeps them on a subdomain, and forwards every other header
// anywhere. Here a chain that left the first request's host, or went from
// https to http, carries redirectSafeHeaders only, even after it comes back.
// keepAIMLAPIOnItsHost runs first: AI/ML API's own rule for its host (#407),
// and the limit of 10 redirects.
func keepHeadersOnTheirHost(req *http.Request, via []*http.Request) error {
	if err := keepAIMLAPIOnItsHost(req, via); err != nil || len(via) == 0 {
		return err
	}
	left := leftHost(via[0], req)
	for _, hop := range via[1:] {
		left = left || leftHost(via[0], hop)
	}
	if !left {
		return nil
	}
	for name := range req.Header {
		if !redirectSafeHeaders[http.CanonicalHeaderKey(name)] {
			delete(req.Header, name)
		}
	}
	return nil
}

// leftHost reports whether hop is on another host than first, a subdomain
// included, or on the same host without the TLS first had. A port is not
// compared: http on 80 redirected to https on 443 is the same host.
func leftHost(first, hop *http.Request) bool {
	if !strings.EqualFold(first.URL.Hostname(), hop.URL.Hostname()) {
		return true
	}
	return first.URL.Scheme == "https" && hop.URL.Scheme != "https"
}

// providerHTTPClient is c with keepHeadersOnTheirHost.
func providerHTTPClient(c *http.Client) *http.Client {
	c.CheckRedirect = keepHeadersOnTheirHost
	return c
}
