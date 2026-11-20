package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/llm"
)

// brokenStream is a stream the transport cut (silence past the stall, a
// reset, an unexpected EOF), as opposed to one the provider ended with an
// error event of its own. Its text is the wrapped error's.
type brokenStream struct{ error }

func (b brokenStream) Unwrap() error { return b.error }

// askAgainAfterBrokenStream is what the Anthropic and Responses shapes do
// with a stream that broke after its headers, the same as the chat shape
// (oai_native.go): with nothing shown yet and an attempt left, the person
// is told, the pause is taken and again is true. Otherwise final is the
// error the turn ends on: silence in the chat shape's words, anything else
// as it came. An error that is not a broken stream comes back untouched.
func askAgainAfterBrokenStream(ctx context.Context, err error, shown bool, attempt int, model string) (again bool, final error) {
	var broken brokenStream
	if err == nil || !errors.As(err, &broken) || ctx.Err() != nil {
		return false, err
	}
	idle := errors.Is(err, errStreamIdle)
	quiet, what, sent := oaiStallAfter, "went silent", "sent nothing"
	if errors.Is(err, errKeepAliveOnly) {
		quiet, what, sent = keepAliveOnlyAfter, "sent only keep-alive comments", "sent only keep-alive comments"
	}
	if !shown && attempt < len(oaiRetryAfter) {
		if idle {
			logs.New("Remote").Warn("provider "+what+" before any text; asking again", slog.Duration("silent_for", quiet),
				slog.Int("attempt", attempt+1), slog.String("model", model))
			llm.Notify(ctx, fmt.Sprintf("The model's provider %s for %s — asking again.", what, quiet))
		} else {
			logs.New("Remote").Warn("connection dropped before any text; asking again",
				slog.Int("attempt", attempt+1), slog.String("model", model), slog.String("error", truncate(err.Error(), 200)))
			llm.Notify(ctx, "The connection to the model's provider dropped — asking again.")
		}
		time.Sleep(oaiRetryAfter[attempt])
		return true, nil
	}
	if idle {
		return false, fmt.Errorf("the model's provider %s for %s", sent, quiet)
	}
	return false, err
}

// shownBy wraps a stream callback so the caller knows whether any text
// reached the window: a stream that broke after that is not asked again
// (the person would read the start twice).
func shownBy(cb llm.StreamCallback, shown *bool) llm.StreamCallback {
	return func(delta string) {
		*shown = true
		if cb != nil {
			cb(delta)
		}
	}
}
