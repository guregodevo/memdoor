package billingsvc

import "testing"

// THE BROKER NEVER FACES THE INTERNET: it listens on the loopback and nginx
// fronts it. Mutation check: ":%d" fails this.
func TestTheBrokerListensOnTheLoopbackOnly(t *testing.T) {
	if got := listenAddr(18790); got != "127.0.0.1:18790" {
		t.Fatalf("listen on the loopback only, got %q", got)
	}
}
