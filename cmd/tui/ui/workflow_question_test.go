package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func stepQuestion(id, text string) workflowEventMsg {
	return workflowEventMsg{runID: "r1", task: "plan", state: "question", text: text, questionID: id, options: []string{"A", "B"}}
}

// A workflow step's question opens the standard picker, and the first answer,
// from this window or another, closes it.
func TestAWorkflowStepsQuestionOpensThePicker(t *testing.T) {
	m := Model{workflowLast: map[string]WorkflowRun{"r1": {ID: "r1", State: "running", Tasks: []WorkflowTask{{Name: "plan", State: "running"}}}}}

	(&m).updateWorkflowPanel(stepQuestion("q1", "Which?"))
	if m.pendingQuestion == nil || m.pendingQuestion.id != "q1" || m.pendingQuestion.question != "Which?" {
		t.Fatalf("the question must open the picker: %+v", m.pendingQuestion)
	}
	if note := m.workflowLast["r1"].Tasks[0].Note; note != "? Which?" {
		t.Errorf("the question shows under its step, got %q", note)
	}

	// A second question waits behind the first.
	(&m).updateWorkflowPanel(stepQuestion("q2", "And?"))
	if m.pendingQuestion.id != "q1" || len(m.questionQueue) != 1 {
		t.Fatalf("a second question waits: pending=%s queue=%d", m.pendingQuestion.id, len(m.questionQueue))
	}

	// Answered here: the next one opens.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = next.(Model)
	if m.pendingQuestion == nil || m.pendingQuestion.id != "q2" {
		t.Fatalf("after an answer the waiting question opens: %+v", m.pendingQuestion)
	}

	// Answered in another window: the picker closes here.
	(&m).updateWorkflowPanel(workflowEventMsg{runID: "r1", task: "plan", state: "answered", questionID: "q2"})
	if m.pendingQuestion != nil || len(m.questionQueue) != 0 {
		t.Fatalf("an answer from another window closes the picker: %+v", m.pendingQuestion)
	}

	// NEGATIVE: an answer to a question this window never showed changes nothing.
	(&m).updateWorkflowPanel(stepQuestion("q3", "Last?"))
	(&m).updateWorkflowPanel(workflowEventMsg{state: "answered", questionID: "other"})
	if m.pendingQuestion == nil || m.pendingQuestion.id != "q3" {
		t.Fatal("another question's answer must not close this picker")
	}
}
