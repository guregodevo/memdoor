package mcp

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
)

// WHY A PERSON SHOULD NOT READ A GO ERROR. A URL that is not an MCP server
// used to say, in the add box:
//
//	Post "https://example.com/mcp": read tcp 192.168.1.135:56777->104.20.23.154:443:
//	read: connection reset by peer
//
// — the local address of the machine included. netReason keeps what tells the
// person what to do and drops the rest.
func netReason(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "it did not answer in time"
	case errors.Is(err, context.Canceled):
		return "the attempt was stopped"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "no such host: " + dns.Name
	}
	// A *url.Error wraps the verb and the URL the caller already names.
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var op *net.OpError
	if errors.As(err, &op) && op.Err != nil {
		// "read tcp <local>-><remote>: read: connection reset by peer" ->
		// "connection reset by peer": the syscall's own words, no addresses.
		s := op.Err.Error()
		if i := strings.LastIndex(s, ": "); i >= 0 {
			s = s[i+2:]
		}
		return s
	}
	return err.Error()
}
