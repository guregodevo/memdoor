package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NewHTTP is a client for a Streamable HTTP server at spec.URL: every
// message is a POST, answered with JSON or with an event stream.
func NewHTTP(spec Spec, logger Logger) Client {
	c := newClient(nil, spec.Name, logger)
	c.t = &httpTransport{spec: spec, logger: c.logger, http: http.DefaultClient}
	return c
}

type httpTransport struct {
	spec   Spec
	logger Logger
	http   *http.Client

	mu       sync.Mutex
	session  string // Mcp-Session-Id the server gave at initialize
	protocol string // negotiated MCP-Protocol-Version
}

func (t *httpTransport) open(context.Context) error { return nil }

func (t *httpTransport) negotiated(v string) {
	t.mu.Lock()
	t.protocol = v
	t.mu.Unlock()
}

func (t *httpTransport) newRequest(ctx context.Context, method string, body []byte) (*http.Request, error) {
	return newAuthedRequest(ctx, t.spec, method, t.spec.URL, body, func(h http.Header) {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.session != "" {
			h.Set("Mcp-Session-Id", t.session)
		}
		if t.protocol != "" {
			h.Set("MCP-Protocol-Version", t.protocol)
		}
	})
}

// newAuthedRequest is a request to target with the server's configured
// headers and, when it signs in, its bearer token; extra adds the
// transport's own headers.
func newAuthedRequest(ctx context.Context, spec Spec, method, target string, body []byte, extra func(http.Header)) (*http.Request, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return nil, err
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if extra != nil {
		extra(req.Header)
	}
	if spec.Token != nil {
		tok, err := spec.Token(ctx)
		if err != nil {
			return nil, err
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	return req, nil
}

// refused is a response's status as an error: a sign-in error for 401 (or
// 403 with a challenge), the body's start for anything else not 2xx.
func refused(resp *http.Response, target string) error {
	switch {
	case resp.StatusCode == http.StatusUnauthorized,
		resp.StatusCode == http.StatusForbidden && resp.Header.Get("WWW-Authenticate") != "":
		return &AuthRequiredError{Status: resp.StatusCode, WWWAuthenticate: resp.Header.Get("WWW-Authenticate")}
	case resp.StatusCode >= 300:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &HTTPError{Status: resp.StatusCode, Target: target, Body: truncate(strings.TrimSpace(string(b)), 300)}
	}
	return nil
}

// HTTPError is a server answering a status that is neither success nor a
// sign-in request.
type HTTPError struct {
	Status int
	Target string
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s answered HTTP %d: %s", e.Target, e.Status, e.Body)
}

// post sends one message and returns the response, refused statuses
// turned into errors.
func (t *httpTransport) post(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := t.newRequest(ctx, http.MethodPost, body)
	if err != nil {
		return nil, err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s did not answer: %s", t.spec.URL, netReason(err))
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.session = sid
		t.mu.Unlock()
	}
	if err := refused(resp, t.spec.URL); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func (t *httpTransport) call(ctx context.Context, id int64, method string, params interface{}) (json.RawMessage, error) {
	body, err := encodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}
	resp, err := t.post(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mt == "text/event-stream" {
		return t.readStream(ctx, resp.Body, id, method)
	}
	var m message
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: unreadable answer: %w", method, err)
	}
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Result, nil
}

// readStream reads server-sent events until the response to id arrives,
// answering the server's own requests on the way.
func (t *httpTransport) readStream(ctx context.Context, r io.Reader, id int64, method string) (json.RawMessage, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64*1024*1024)
	var data strings.Builder
	dispatch := func() (json.RawMessage, bool, error) {
		defer data.Reset()
		if data.Len() == 0 {
			return nil, false, nil
		}
		var m message
		if json.Unmarshal([]byte(data.String()), &m) != nil {
			return nil, false, nil
		}
		switch {
		case m.Method != "" && len(m.ID) > 0:
			if resp, err := t.post(ctx, replyTo(m)); err == nil {
				resp.Body.Close()
			}
		case m.Method != "":
		default:
			if got, ok := idOf(m.ID); ok && got == id {
				if m.Error != nil {
					return nil, true, m.Error
				}
				return m.Result, true, nil
			}
		}
		return nil, false, nil
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if res, done, err := dispatch(); done {
				return res, err
			}
			continue
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(v, " "))
		}
	}
	if res, done, err := dispatch(); done {
		return res, err
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: the stream broke: %w", method, err)
	}
	return nil, fmt.Errorf("%s: the stream ended without an answer", method)
}

func (t *httpTransport) notify(ctx context.Context, method string, params interface{}) error {
	body, err := encodeNotification(method, params)
	if err != nil {
		return err
	}
	resp, err := t.post(ctx, body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// close ends the session on the server, when it gave one.
func (t *httpTransport) close() error {
	t.mu.Lock()
	sid := t.session
	t.session = ""
	t.mu.Unlock()
	if sid == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := t.newRequest(ctx, http.MethodDelete, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Mcp-Session-Id", sid)
	if resp, err := t.http.Do(req); err == nil {
		resp.Body.Close()
	}
	return nil
}
