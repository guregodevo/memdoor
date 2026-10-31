package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// step runs one login/* action and decodes the reply.
func step(t *testing.T, action, id, value string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	signInStep(w, httptest.NewRequest(http.MethodPost, "/", nil), action, id, value)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// A fake browser sign-in: what is pasted ends it, "refuse" with an error.
func fakeSignIn() string {
	pasted := make(chan string, 1)
	return startSignIn(func(v string) error {
		if v == "" {
			return errors.New("paste something")
		}
		pasted <- v
		return nil
	}, func(ctx context.Context) signInOutcome {
		select {
		case v := <-pasted:
			if v == "refuse" {
				return signInOutcome{err: errors.New("refused"), again: true}
			}
			return signInOutcome{finish: func(context.Context) any { return map[string]any{"done": true, "got": v} }}
		case <-ctx.Done():
			return signInOutcome{err: ctx.Err()}
		}
	})
}

// The steps every browser sign-in shares (ChatGPT, OpenRouter): pending
// while nobody answered, a paste ends it with the provider's finish reply,
// and the sign-in is then gone.
func TestSignInSteps(t *testing.T) {
	saved := signInWaitFor
	signInWaitFor = 50 * time.Millisecond
	t.Cleanup(func() { signInWaitFor = saved })

	id := fakeSignIn()
	if _, out := step(t, "login/wait", id, ""); out["pending"] != true {
		t.Fatalf("nobody answered: want pending, got %v", out)
	}
	if code, _ := step(t, "login/paste", id, ""); code != http.StatusBadRequest {
		t.Fatalf("an empty paste is refused in words, got %d", code)
	}
	if _, out := step(t, "login/paste", id, "the-code"); out["delivered"] != true {
		t.Fatalf("paste: %v", out)
	}
	if _, out := step(t, "login/wait", id, ""); out["done"] != true || out["got"] != "the-code" {
		t.Fatalf("want the finish reply, got %v", out)
	}
	if code, _ := step(t, "login/wait", id, ""); code != http.StatusNotFound {
		t.Fatalf("a finished sign-in is gone, got %d", code)
	}
}

// A refused sign-in says why and whether to start again; a cancelled one
// ends; an unknown id is 404.
func TestSignInRefusedCancelledUnknown(t *testing.T) {
	id := fakeSignIn()
	step(t, "login/paste", id, "refuse")
	if _, out := step(t, "login/wait", id, ""); out["error"] != "refused" || out["again"] != true {
		t.Fatalf("refused: %v", out)
	}
	id = fakeSignIn()
	if _, out := step(t, "login/cancel", id, ""); out["cancelled"] != true {
		t.Fatalf("cancel: %v", out)
	}
	if _, out := step(t, "login/wait", id, ""); out["error"] != context.Canceled.Error() {
		t.Fatalf("after cancel: %v", out)
	}
	if code, _ := step(t, "login/wait", "nope", ""); code != http.StatusNotFound {
		t.Fatalf("unknown id: %d", code)
	}
}
