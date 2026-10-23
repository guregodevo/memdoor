package gateway

import (
	"net"
	"sort"
)

// listenAddresses is where another device could reach a gateway bound to
// host: nothing for localhost, the host itself when it is one address, and
// this machine's own addresses when it listens on every interface, the
// private network ones first (a phone on the same Wi-Fi), then a VPN's
// (Tailscale's 100.64/10), then the rest. A window puts a /remote link on
// the first (2026-10-23, Greg: "/remote should not go through memdoor.ai
// anymore unless it's needed").
func listenAddresses(host string) []string {
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return nil
		}
		var ips []net.IP
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ips = append(ips, ipn.IP)
		}
		return orderAddresses(ips)
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	if host == "localhost" {
		return nil
	}
	return []string{host}
}

// orderAddresses keeps the addresses a phone could dial, in the order a
// window should try them.
func orderAddresses(ips []net.IP) []string {
	type ranked struct {
		s    string
		rank int
	}
	var out []ranked
	for _, ip := range ips {
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
			continue
		}
		v4 := ip.To4()
		if v4 == nil {
			continue // a phone types an IPv4 address; IPv6 needs brackets and rarely routes on a home LAN
		}
		rank := 2
		switch {
		case v4[0] == 100 && v4[1] >= 64 && v4[1] < 128: // Tailscale and other CGNAT ranges
			rank = 1
		case v4.IsPrivate():
			rank = 0
		}
		out = append(out, ranked{v4.String(), rank})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].rank < out[j].rank })
	res := make([]string, 0, len(out))
	for _, r := range out {
		res = append(res, r.s)
	}
	return res
}
