package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"memdoor/gateway/logs"
)

// Batch is the wire format the HTTP client POSTs to the monitoring
// endpoint. HostID is an anonymous, install-stable UUID generated
// the first time the gateway starts with telemetry enabled; it lets
// the server group events from the same install without identifying
// the user.
type Batch struct {
	HostID  string        `json:"host_id"`
	Version string        `json:"version"`
	Events  []*logs.Event `json:"events"`
}

// Client posts batches to the monitoring server. Interface so tests
// can substitute a fake that records calls without touching the
// network.
type Client interface {
	// Send transmits one batch synchronously. Returns nil on 2xx,
	// an error on any other outcome (network failure, non-2xx
	// status, marshal error). The flusher decides whether to
	// retry — Client is intentionally not retry-aware.
	Send(ctx context.Context, batch Batch) error
}

// NewHTTPClient returns a Client that POSTs to the given endpoint
// with the optional bearer token. Factory per Memdoor convention.
//
// httpClient is exposed for testing (substitute a stub Transport);
// production callers pass nil to get a default http.Client with
// a 10-second timeout — short enough that a stalled monitoring
// endpoint never starves the flusher loop.
func NewHTTPClient(endpoint, bearerToken string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &httpClient2{
		endpoint:    endpoint,
		bearerToken: bearerToken,
		http:        httpClient,
	}
}

// httpClient2 is named with a trailing 2 to dodge the imported
// http.Client type collision. The exported factory hides the name.
type httpClient2 struct {
	endpoint    string
	bearerToken string
	http        *http.Client
}

func (c *httpClient2) Send(ctx context.Context, batch Batch) error {
	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal batch: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer func() {
		// Drain the body before close so net/http can return the
		// connection to its idle pool for the next flush. Without
		// this, every send opens a fresh connection — wasteful and
		// can run into the server's connection table on a busy box.
		// 1 KiB max in case the server ever returns a non-empty body
		// (today it returns `{"received":N}` which is tiny).
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
		_ = resp.Body.Close()
	}()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("telemetry HTTP %d", resp.StatusCode)
	}
	return nil
}
