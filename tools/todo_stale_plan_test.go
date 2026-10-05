package tools

import (
	"strings"
	"testing"
)

// A plan made for an earlier request does not end this one when its boxes
// are ticked (live 2026-09-28: the previous prompt's six steps, ticked in a
// new request, said "Nothing is left to do … STOP" with two of its three
// asks undone). A plan made in this run still ends it.
func TestAFinishedPlanFromAnEarlierRequestDoesNotEndThisOne(t *testing.T) {
	const key = "chan-stale-plan"
	old := []TodoItem{{Content: "delete the tunnel", Status: "pending"}, {Content: "run the tests", Status: "pending"}}
	NotePlanWrite(key, "run-1", nil, old)

	done := []TodoItem{{Content: "delete the tunnel", Status: "completed"}, {Content: "run the tests", Status: "completed"}}
	NotePlanWrite(key, "run-2", old, done) // the same plan, ticked in the next request
	out := FormatTodosFor(done, PlanFromEarlierRun(key, "run-2"))
	if strings.Contains(out, "Nothing is left to do") || !strings.Contains(out, "made for an earlier request") {
		t.Fatalf("a plan from run-1 must not stop run-2:\n%s", out)
	}

	fresh := []TodoItem{{Content: "wire the account token", Status: "completed"}}
	NotePlanWrite(key, "run-2", done, fresh) // a new plan, made in this run
	if out := FormatTodosFor(fresh, PlanFromEarlierRun(key, "run-2")); !strings.Contains(out, "Nothing is left to do") {
		t.Fatalf("a plan finished in the run that made it ends the turn:\n%s", out)
	}
	if !PlanFromEarlierRun(key, "run-3") {
		t.Fatal("the next run sees it as an earlier request's plan")
	}
}
