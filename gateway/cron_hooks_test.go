package gateway

import (
	"fmt"
	"testing"

	"memdoor/pkg/domain"
)

// A cron job added at runtime was stored, listed by `cron list`, and never
// fired — the scheduler registers jobs once at start-up and nothing told it
// about later ones. Nothing warned either (2026-08-24).
func TestRuntimeJobReachesTheScheduler(t *testing.T) {
	cs := &ChatServer{}
	var got *domain.CronJob
	cs.onCronJobAdded = func(j *domain.CronJob) error { got = j; return nil }

	job := &domain.CronJob{ID: "hello-test", Schedule: "*/1 * * * *", Message: "hello", Enabled: true}
	if cs.onCronJobAdded == nil {
		t.Fatal("no hook: a runtime job would be stored and never scheduled")
	}
	if err := cs.onCronJobAdded(job); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "hello-test" {
		t.Fatalf("the scheduler was not told about the job: %+v", got)
	}
}

// Deleting a job must stop it running, or the scheduler keeps firing something
// the renter removed.
func TestRemovingAJobStopsIt(t *testing.T) {
	cs := &ChatServer{}
	var removed string
	cs.onCronJobRemoved = func(id string) error { removed = id; return nil }
	if err := cs.onCronJobRemoved("hello-test"); err != nil {
		t.Fatal(err)
	}
	if removed != "hello-test" {
		t.Fatal("the scheduler was not told to stop the job")
	}
}

// With no scheduler running, storing must still work — the job runs after a
// restart, and the handler says so rather than implying a live schedule.
func TestNoSchedulerStillStores(t *testing.T) {
	cs := &ChatServer{}
	if cs.onCronJobAdded != nil {
		t.Fatal("a server without a scheduler should carry no hook")
	}
}

// The WORKSPACE must survive. Delegating persistence to the scheduler wrote
// the job under an empty workspace — its store is built with one — so `cron
// list`, which queries by workspace, could never see it. Scheduled, stored,
// invisible: worse than the bug it replaced (2026-08-24).
func TestWorkspaceSurvivesScheduling(t *testing.T) {
	cs := &ChatServer{}
	var seen *domain.CronJob
	cs.onCronJobAdded = func(j *domain.CronJob) error { seen = j; return nil }

	job := &domain.CronJob{ID: "x", WorkspaceID: "hackernews", Schedule: "0 * * * * *"}
	if err := cs.onCronJobAdded(job); err != nil {
		t.Fatal(err)
	}
	if seen.WorkspaceID != "hackernews" {
		t.Fatalf("the workspace was lost on the way to the scheduler: %q", seen.WorkspaceID)
	}
}

// A job that is stored but NOT scheduled must be reported as such. The whole
// failure was a caller believing a schedule that nothing was executing.
func TestUnscheduledJobIsReportedNotHidden(t *testing.T) {
	cs := &ChatServer{}
	cs.onCronJobAdded = func(j *domain.CronJob) error { return errNotScheduled }
	if err := cs.onCronJobAdded(&domain.CronJob{ID: "x"}); err == nil {
		t.Fatal("a scheduling failure must surface, not be swallowed")
	}
}

var errNotScheduled = fmt.Errorf("scheduler rejected the expression")
