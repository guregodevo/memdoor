package context

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNoProgress is the cause of a context ended by WithIdleTimeout.
var ErrNoProgress = errors.New("no progress")

type progressKey struct{}

// WithIdleTimeout is a context that ends when nothing reports progress
// (Progress) for d, not d after it starts. A turn's backstop exists to catch
// a HUNG turn: as a total, it ended a coder turn at exactly 30:00 while the
// model was answering every few seconds (live 2026-09-30, 19:49:52 →
// 20:19:52). Progress reaches every idle timeout the context is under, so an
// outer net set the same way is kept alive by the same reports.
func WithIdleTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	timer := time.AfterFunc(d, func() {
		cancel(fmt.Errorf("%w for %s", ErrNoProgress, d))
	})
	outer, _ := parent.Value(progressKey{}).(func())
	report := func() {
		timer.Reset(d)
		if outer != nil {
			outer()
		}
	}
	return context.WithValue(ctx, progressKey{}, report), func() {
		timer.Stop()
		cancel(context.Canceled)
	}
}

// Progress reports that the work under ctx moved on: a model answered, a
// tool returned. Without an idle timeout above it, it does nothing.
func Progress(ctx context.Context) {
	if report, ok := ctx.Value(progressKey{}).(func()); ok {
		report()
	}
}
