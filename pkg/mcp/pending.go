package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// pending matches the responses that arrive on a stream (a local server's
// stdout, an SSE stream) to the requests waiting for them.
type pending struct {
	mu   sync.Mutex
	m    map[int64]chan message
	dead error // set once the stream ended
}

func newPending() *pending { return &pending{m: map[int64]chan message{}} }

// route handles one message from the stream: a response goes to its
// request, a request from the server is answered through reply, a
// notification is dropped.
func (p *pending) route(m message, reply func([]byte)) {
	switch {
	case m.Method != "" && len(m.ID) > 0:
		reply(replyTo(m))
	case m.Method != "":
	default:
		if id, ok := idOf(m.ID); ok {
			p.mu.Lock()
			ch := p.m[id]
			delete(p.m, id)
			p.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		}
	}
}

// fail ends every waiting request with err; later requests fail with it.
func (p *pending) fail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dead = err
	for id, ch := range p.m {
		close(ch)
		delete(p.m, id)
	}
}

// call registers id, sends the request, and waits for its response.
func (p *pending) call(ctx context.Context, id int64, method, server string, send func() error) (json.RawMessage, error) {
	ch := make(chan message, 1)
	p.mu.Lock()
	if p.dead != nil {
		err := p.dead
		p.mu.Unlock()
		return nil, err
	}
	p.m[id] = ch
	p.mu.Unlock()
	drop := func() {
		p.mu.Lock()
		delete(p.m, id)
		p.mu.Unlock()
	}
	if err := send(); err != nil {
		drop()
		return nil, err
	}
	select {
	case m, ok := <-ch:
		if !ok {
			p.mu.Lock()
			err := p.dead
			p.mu.Unlock()
			return nil, err
		}
		if m.Error != nil {
			return nil, m.Error
		}
		return m.Result, nil
	case <-ctx.Done():
		drop()
		return nil, fmt.Errorf("%s: no answer from %s: %w", method, server, ctx.Err())
	}
}
