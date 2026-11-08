package gateway

import (
	"context"
	"errors"
	"testing"
)

func TestWorkflowReviewStopsBeforeNextStep(t *testing.T) {
	for _, returnsError := range []bool{false, true} {
		t.Run(map[bool]string{false: "runner returns nil", true: "runner returns cancellation"}[returnsError], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			err := runWorkflowReview(ctx, []string{"fix", "verify"}, func(stepCtx context.Context, _ int, _ string) error {
				calls++
				cancel()
				if returnsError {
					return stepCtx.Err()
				}
				return nil
			})
			if !errors.Is(err, context.Canceled) || calls != 1 {
				t.Fatalf("stopped review: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestWorkflowReviewCanceledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runWorkflowReview(ctx, []string{"fix"}, func(context.Context, int, string) error {
		t.Fatal("canceled review started a step")
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestWorkflowReviewFinishesOrPropagatesFailure(t *testing.T) {
	want := errors.New("step failed")
	for _, fail := range []bool{false, true} {
		calls := 0
		err := runWorkflowReview(context.Background(), []string{"fix", "verify"}, func(context.Context, int, string) error {
			calls++
			if fail {
				return want
			}
			return nil
		})
		if fail && (!errors.Is(err, want) || calls != 1) {
			t.Fatalf("failure lost: calls=%d err=%v", calls, err)
		}
		if !fail && (err != nil || calls != 2) {
			t.Fatalf("successful review: calls=%d err=%v", calls, err)
		}
	}
}
