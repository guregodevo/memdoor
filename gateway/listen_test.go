package gateway

import (
	"net"
	"strings"
	"testing"
)

// A gateway on localhost has no address for a phone; one address is itself;
// every interface is this machine's addresses, the private network first,
// then a VPN's, then the rest, never loopback or link-local, never IPv6.
func TestListenAddressesAreWhatAPhoneCouldDial(t *testing.T) {
	if got := listenAddresses("localhost"); got != nil {
		t.Fatalf("localhost: %v", got)
	}
	if got := listenAddresses("127.0.0.1"); got != nil {
		t.Fatalf("127.0.0.1: %v", got)
	}
	if got := listenAddresses("192.168.1.9"); len(got) != 1 || got[0] != "192.168.1.9" {
		t.Fatalf("one address: %v", got)
	}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("203.0.113.7"), net.ParseIP("fe80::1"), net.ParseIP("100.100.1.2"), net.ParseIP("192.168.1.9"), net.ParseIP("2001:db8::1"), net.ParseIP("169.254.1.1")}
	if got := strings.Join(orderAddresses(ips), " "); got != "192.168.1.9 100.100.1.2 203.0.113.7" {
		t.Fatalf("order: %q", got)
	}
	for _, a := range listenAddresses("0.0.0.0") {
		if ip := net.ParseIP(a); ip == nil || ip.IsLoopback() || ip.To4() == nil {
			t.Fatalf("every interface: %q is not a phone's address", a)
		}
	}
}
