package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// NewSSE is a client for a server on the old HTTP+SSE transport (MCP
// 2024-11-05): a GET opens an event stream whose first event names the URL
// to POST messages to, and the answers come back on the stream.
func NewSSE(spec Spec, logger Logger) Client {
	c := newClient(nil, spec.Name, logger)
	c.t = &sseTransport{spec: spec, logger: c.logger, pending: newPending(), http: http.DefaultClient}
	return c
}

type sseTransport struct {
	spec    Spec
	logger  Logger
	http    *http.Client
	pending *pending

	mu       sync.Mutex
	endpoint string
	cancel   context.CancelFunc
}

func (t *sseTransport) open(ctx context.Context) error {
	streamCtx, cancel := context.WithCancel(context.Background())
	req, err := newAuthedRequest(streamCtx, t.spec, http.MethodGet, t.spec.URL, nil, func(h http.Header) {
		h.Set("Accept", "text/event-stream")
		h.Del("Content-Type")
	})
	if err != nil {
		cancel()
		return err
	}
	type opened struct {
		resp *http.Response
		err  error
	}
	got := make(chan opened, 1)
	go func() { resp, err := t.http.Do(req); got <- opened{resp, err} }()
	var resp *http.Response
	select {
	case o := <-got:
		if o.err != nil {
			cancel()
			return fmt.Errorf("%s did not answer: %s", t.spec.URL, netReason(o.err))
		}
		resp = o.resp
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
	if err := refused(resp, t.spec.URL); err != nil {
		resp.Body.Close()
		cancel()
		return err
	}
	endpoint := make(chan string, 1)
	go t.read(resp.Body, endpoint)
	select {
	case e := <-endpoint:
		base, _ := url.Parse(t.spec.URL)
		ref, err := url.Parse(e)
		if err != nil {
			cancel()
			return fmt.Errorf("%s named an unreadable endpoint %q", t.spec.URL, e)
		}
		t.mu.Lock()
		t.endpoint, t.cancel = base.ResolveReference(ref).String(), cancel
		t.mu.Unlock()
		return nil
	case <-ctx.Done():
		cancel()
		return fmt.Errorf("%s sent no endpoint event: %w", t.spec.URL, ctx.Err())
	}
}

// read is the only reader of the stream: the endpoint event once, then
// messages routed to the requests waiting for them.
func (t *sseTransport) read(body io.ReadCloser, endpoint chan<- string) {
	defer body.Close()
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
	var event string
	var data strings.Builder
	sentEndpoint := false
	dispatch := func() {
		defer func() { event = ""; data.Reset() }()
		switch event {
		case "endpoint":
			if !sentEndpoint {
				sentEndpoint = true
				endpoint <- strings.TrimSpace(data.String())
			}
		case "", "message":
			var m message
			if json.Unmarshal([]byte(data.String()), &m) == nil {
				t.pending.route(m, func(b []byte) { _ = t.post(context.Background(), b) })
			}
		}
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			dispatch()
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	err := sc.Err()
	if err == nil {
		err = io.EOF
	}
	t.pending.fail(fmt.Errorf("the event stream from %s ended (%v)", t.spec.Name, err))
}

func (t *sseTransport) post(ctx context.Context, body []byte) error {
	t.mu.Lock()
	target := t.endpoint
	t.mu.Unlock()
	req, err := newAuthedRequest(ctx, t.spec, http.MethodPost, target, body, nil)
	if err != nil {
		return err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s did not answer: %s", target, netReason(err))
	}
	defer resp.Body.Close()
	return refused(resp, target)
}

func (t *sseTransport) call(ctx context.Context, id int64, method string, params interface{}) (json.RawMessage, error) {
	b, err := encodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}
	return t.pending.call(ctx, id, method, t.spec.Name, func() error { return t.post(ctx, b) })
}

func (t *sseTransport) notify(ctx context.Context, method string, params interface{}) error {
	b, err := encodeNotification(method, params)
	if err != nil {
		return err
	}
	return t.post(ctx, b)
}

func (t *sseTransport) negotiated(string) {}

func (t *sseTransport) close() error {
	t.mu.Lock()
	cancel := t.cancel
	t.cancel = nil
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}
