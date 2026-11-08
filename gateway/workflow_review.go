package gateway

import "context"

// runWorkflowReview checks cancellation even when a runner returns nil after
// cancellation, so a stopped review cannot begin another task.
func runWorkflowReview(ctx context.Context, chain []string, step func(context.Context, int, string) error) error {
	for i, name := range chain {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := step(ctx, i, name); err != nil {
			return err
		}
	}
	return ctx.Err()
}
