package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memdoor/pkg/llm"
)

// A reply that keeps streaming past the client's total is cut, unless the
// call carries its own clock (a workflow's llm task): then its deadline alone
// bounds it. Marked but with no deadline, the total still applies.
func TestACallWithItsOwnClockOutlivesTheClientTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 8; i++ {
			fmt.Fprintf(w, "data: %d\n\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	t.Cleanup(srv.Close)
	client := stallBoundClient(time.Second, 150*time.Millisecond)
	read := func(ctx context.Context) error {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		return err
	}
	if err := read(context.Background()); err == nil {
		t.Error("a turn's reply streaming past the total was not cut")
	}
	ctx, cancel := context.WithTimeout(llm.WithOwnClock(context.Background()), 5*time.Second)
	defer cancel()
	if err := read(ctx); err != nil {
		t.Errorf("a call on its own clock was cut: %v", err)
	}
	if err := read(llm.WithOwnClock(context.Background())); err == nil {
		t.Error("a call marked with no deadline was not bounded by the total")
	}
}
