package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"memdoor/pkg/decision"
)

// The shared relevance judge behind every jev tool (jgrep, jlogs, jread): one boolean question per item against a task, in
// parallel batches through the workspace decision model. Any batch coming
// back unavailable fails the whole judgment, so callers fall back to their
// unjudged output instead of silently dropping unjudged items.

const (
	jevBatch          = 24 // questions per decision request
	jevParallel       = 8
	jevKeep           = 0.5 // probability at or above which an item is kept
	judgeBatchRetries = 1   // extra asks on top of the service's own retry
)

// judgeRetryWait spaces the judge's extra ask; a var only so tests shorten it.
var judgeRetryWait = 2 * time.Second

type relevanceSpec struct {
	Task     string // what the agent is trying to do
	Framing  string // one sentence on what the items are
	Question string // asked of every item
	True     string // what "yes" means
	False    string // what "no" means
	Purpose  string // for the decision log
}

// judgeRelevance returns P(true) for every item, in order.
func judgeRelevance(ctx context.Context, svc decision.Service, spec relevanceSpec, items []string) ([]float64, error) {
	if svc == nil {
		return nil, fmt.Errorf("no decision model configured")
	}
	state := "An agent is working on this task:\n" + spec.Task + "\n\n" + spec.Framing
	ps := make([]float64, len(items))
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, jevParallel)
	)
	for lo := 0; lo < len(items); lo += jevBatch {
		hi := min(lo+jevBatch, len(items))
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			req := decision.Request{State: state, Questions: map[string]decision.Question{}}
			for i := lo; i < hi; i++ {
				q, err := decision.Boolean(spec.Question+"\n"+items[i], spec.True, spec.False)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				req.Questions[fmt.Sprintf("i%d", i)] = q
			}
			var res decision.Result
			for try := 0; ; try++ {
				res = svc.Evaluate(ctx, req, decision.Options{Purpose: spec.Purpose})
				if res.OK() || res.Reason != decision.ReasonOverloaded || try >= judgeBatchRetries || ctx.Err() != nil {
					break
				}
				select {
				case <-ctx.Done():
				case <-time.After(time.Duration(try+1) * judgeRetryWait):
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if !res.OK() {
				if firstErr == nil {
					firstErr = fmt.Errorf("decision model %s: %s", res.Reason, res.Detail)
				}
				return
			}
			for i := lo; i < hi; i++ {
				ps[i] = res.Answers[fmt.Sprintf("i%d", i)].ProbabilityTrue
			}
		}(lo, hi)
	}
	wg.Wait()
	return ps, firstErr
}

// JudgeProjectSections scores each section of a project's instruction file
// (AGENTS.md) against the task the conversation opened with. The
// gateway keeps the sections the task needs in the system prompt and lists
// the rest (gateway/project_instructions.go).
func JudgeProjectSections(ctx context.Context, task string, sections []string) ([]float64, error) {
	return judgeRelevance(ctx, decisionService(), relevanceSpec{
		Task:     task,
		Framing:  "Each question shows one section of the project's instructions file (AGENTS.md), which a coding agent is given before it starts work.",
		Question: "Does the agent need this section of the instructions for the task?",
		True:     "The section holds a rule, command or fact the task needs: how to build or test, a convention for the code involved, or a constraint on the change.",
		False:    "The section is about something the task does not touch.",
		Purpose:  "project_instructions",
	}, sections)
}
