package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"memdoor/pkg/llm"
)

const interruptedDelta = `{"choices":[{"delta":{"content":"partial answer"},"finish_reason":null}]}`

func TestChatInterruptedAfterTextPreservesPartialAndReturnsError(t *testing.T) {
	for _, mode := range []string{"eof", "cancel", "deadline", "reset"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			c, done := oaiTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: "+interruptedDelta+"\n\n")
				w.(http.Flusher).Flush()
				if mode == "cancel" || mode == "deadline" {
					<-r.Context().Done()
				} else if mode == "reset" {
					panic(http.ErrAbortHandler)
				}
			})
			defer done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "deadline" {
				var timeoutCancel context.CancelFunc
				ctx, timeoutCancel = context.WithTimeout(ctx, 5*time.Second)
				defer timeoutCancel()
			}
			ctx = llm.WithStreamCallback(ctx, func(string) {
				if mode == "cancel" {
					cancel()
				}
			})
			msg, err := c.Messages().New(ctx, llm.MessageNewParams{MaxTokens: 64,
				Messages: []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("hi"))}})
			if err == nil || msg == nil || msg.StopReason != llm.StopReasonInterrupted {
				t.Fatalf("interrupted stream reported completion: msg=%+v err=%v", msg, err)
			}
			if len(msg.Content) != 1 || msg.Content[0].Text != "partial answer" || calls.Load() != 1 {
				t.Fatalf("partial output lost or retried: msg=%+v calls=%d", msg, calls.Load())
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation cause lost: %v", err)
			}
			if mode == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("deadline cause lost: %v", err)
			}
		})
	}
}

func TestChatFinishedStreamStillCompletes(t *testing.T) {
	for _, ending := range []string{
		"data: [DONE]\n\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n",
	} {
		c, done := oaiTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: "+interruptedDelta+"\n\n"+ending)
		})
		msg, err := ask(c)
		done()
		if err != nil || msg == nil || msg.StopReason != llm.StopReasonEndTurn {
			t.Fatalf("completed response was interrupted: msg=%+v err=%v", msg, err)
		}
	}
}
