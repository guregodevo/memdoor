package cmd

import "testing"

func TestNormalizeGateway(t *testing.T) {
	cases := map[string]string{
		"192.168.1.10":            "http://192.168.1.10:18789",
		"192.168.1.10:18799":      "http://192.168.1.10:18799",
		"box.local":               "http://box.local:18789",
		"https://box.example":     "https://box.example:18789",
		"http://localhost:18789":  "http://localhost:18789",
		"http://localhost:18789/": "http://localhost:18789",
		"::1":                     "http://[::1]:18789",
		"[::1]:18799":             "http://[::1]:18799",
	}
	for in, want := range cases {
		if got := canonicalGatewayAddr(in); got != want {
			t.Errorf("canonicalGatewayAddr(%q) = %q, want %q", in, got, want)
		}
	}
}
