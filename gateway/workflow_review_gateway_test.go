package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"memdoor/pkg/workflow"
)

func TestStopCancelsRequestedChangesAndKeepsStoppedState(t *testing.T) {
	for _, returnNil := range []bool{false, true} {
		t.Run(map[bool]string{false: "runner honors cancellation", true: "runner returns nil"}[returnNil], func(t *testing.T) {
			s, run, _ := workflowServerWithRun(t)
			run.state = "waiting"
			run.asked = map[string]workflow.TaskRun{"tests": {}, "changelog": {}, "deploy": {}}
			started, canceled := make(chan struct{}), make(chan struct{})
			calls := make(chan struct{}, 3)
			_, err := s.requestChangesWithRunner(run.ID, "tests", "fix the tests", func(ctx context.Context, _ workflow.TaskRun) (string, error) {
				calls <- struct{}{}
				close(started)
				<-ctx.Done()
				close(canceled)
				if returnNil {
					return "", nil
				}
				return "", ctx.Err()
			})
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("review step did not start")
			}
			if _, err := s.stopWorkflow(run.ID); err != nil {
				t.Fatal(err)
			}
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("Stop did not cancel the active review step")
			}
			deadline := time.Now().Add(time.Second)
			for run.snapshot().State != "stopped" {
				if time.Now().After(deadline) {
					t.Fatalf("review overwrote Stop: %s", run.snapshot().State)
				}
				time.Sleep(time.Millisecond)
			}
			if len(calls) != 1 {
				t.Fatalf("a later review step started: %d calls", len(calls))
			}
			if _, err := s.requestChanges(run.ID, "tests", "again"); err == nil {
				t.Fatal("a stopped run accepted another change request")
			}
		})
	}
}

func TestRequestedChangesReturnToReviewAfterCompletionOrFailure(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failure], func(t *testing.T) {
			s, run, _ := workflowServerWithRun(t)
			run.state = "waiting"
			run.asked = map[string]workflow.TaskRun{"tests": {}, "changelog": {}, "deploy": {}}
			if _, err := s.requestChangesWithRunner(run.ID, "tests", "fix it", func(context.Context, workflow.TaskRun) (string, error) {
				if failure {
					return "", errors.New("test step failed")
				}
				return "", nil
			}); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for run.snapshot().State != "waiting" {
				if time.Now().After(deadline) {
					t.Fatalf("review did not return to its gate: %s", run.snapshot().State)
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}
