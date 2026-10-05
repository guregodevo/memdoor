package gateway

import "sync"

// questionBroker coordinates interactive ask_user_question round-trips. The tool
// runs server-side inside the agent loop: it registers a pending question and
// blocks on the returned channel; the WebSocket handler delivers the client's
// answer via answer(). This turns a synchronous tool call into a request/response
// over the async event stream (emit the question as an event, wait for the reply).
type questionBroker struct {
	mu      sync.Mutex
	pending map[string]chan string
}

func newQuestionBroker() *questionBroker {
	return &questionBroker{pending: make(map[string]chan string)}
}

// register creates a pending question and returns the channel to wait on plus a
// cleanup func (call via defer so an abandoned question doesn't leak).
func (b *questionBroker) register(id string) (<-chan string, func()) {
	ch := make(chan string, 1) // buffered so answer() never blocks
	b.mu.Lock()
	b.pending[id] = ch
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
	}
}

// answer delivers the client's selection to a waiting question. Returns false if
// no question with that id is pending (already answered, timed out, or unknown).
func (b *questionBroker) answer(id, value string) bool {
	b.mu.Lock()
	ch, ok := b.pending[id]
	if ok {
		delete(b.pending, id)
	}
	b.mu.Unlock()
	if !ok {
		return false
	}
	ch <- value // buffered: non-blocking
	return true
}
