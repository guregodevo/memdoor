package mcp

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestNetReasonSaysWhatIsWrongWithoutTheLocalAddress(t *testing.T) {
	local, remote := &net.TCPAddr{IP: net.IPv4(192, 168, 1, 135), Port: 56777}, &net.TCPAddr{IP: net.IPv4(104, 20, 23, 154), Port: 443}
	reset := &url.Error{Op: "Post", URL: "https://example.com/mcp", Err: &net.OpError{
		Op: "read", Net: "tcp", Source: local, Addr: remote, Err: errors.New("read: connection reset by peer")}}
	got := netReason(reset)
	if got != "connection reset by peer" {
		t.Fatalf("reason = %q", got)
	}
	if strings.Contains(got, "192.168") || strings.Contains(got, "Post") {
		t.Fatalf("the person's own address must not appear: %q", got)
	}

	if got := netReason(&url.Error{Op: "Post", URL: "http://localhost:1/mcp", Err: &net.OpError{
		Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}}); got != "connection refused" {
		t.Fatalf("refused = %q", got)
	}
	if got := netReason(&url.Error{Err: &net.DNSError{Name: "nope.invalid"}}); got != "no such host: nope.invalid" {
		t.Fatalf("dns = %q", got)
	}
	if got := netReason(context.DeadlineExceeded); got != "it did not answer in time" {
		t.Fatalf("timeout = %q", got)
	}
}
