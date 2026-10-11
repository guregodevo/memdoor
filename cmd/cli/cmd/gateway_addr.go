package cmd

import (
	"net"
	"net/url"
	"strings"
)

// canonicalGatewayAddr makes `--gateway 192.168.1.10`, `host:18799` and
// `https://box` all work (Greg, 2026-10-11: "use --gateway ip_address
// work too"): no scheme is http, no port is the gateway's default.
func canonicalGatewayAddr(s string) string {
	s = strings.TrimSpace(strings.TrimRight(s, "/"))
	if s == "" {
		return s
	}
	if !strings.Contains(s, "://") {
		// A bare IPv6 address has colons of its own: bracket it first.
		if ip := net.ParseIP(s); ip != nil && ip.To4() == nil {
			s = "[" + s + "]"
		}
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return s
	}
	if u.Port() == "" {
		host := u.Hostname()
		if strings.Contains(host, ":") { // a bare IPv6 address
			host = "[" + host + "]"
		}
		u.Host = net.JoinHostPort(strings.Trim(host, "[]"), "18789")
	}
	return u.String()
}
