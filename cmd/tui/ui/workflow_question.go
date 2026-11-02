package ui

import "time"

// A workflow step's ask_user_question reaches every window of the project as
// a workflow_event (gateway: workflow_runs.go relayQuestion). It opens the
// same picker a turn's question does, and the answer is sent back by its
// question ID. The first window to answer closes the picker in the others
// ("answered"). Questions that arrive while a picker is open queue in order.

// askFromStep shows a step's question under the step in the run's graph and
// in the picker, or queues it when a picker is already open.
func (m *Model) askFromStep(t workflowEventMsg) {
	if t.questionID == "" || t.text == "" || len(t.options) == 0 {
		return
	}

	m.noteTask(t.runID, t.task, "? "+t.text)
	m.messages = append(m.messages, Message{Role: "system", Content: t.task + " asks: " + t.text, Timestamp: time.Now()})

	q := pendingQuestion{id: t.questionID, question: t.text, options: t.options}
	if m.pendingQuestion == nil {
		m.pendingQuestion = &q
	} else {
		m.questionQueue = append(m.questionQueue, q)
	}

	if m.viewport.Height > 0 {
		m.refreshFollow()
	}
}

// questionAnswered closes the picker, or removes the question from the queue,
// when another window answered it first.
func (m *Model) questionAnswered(id string) {
	if m.pendingQuestion != nil && m.pendingQuestion.id == id {
		m.pendingQuestion = nil
		m.note("Answered in another window.")
		m.nextQuestion()
		return
	}

	for i, q := range m.questionQueue {
		if q.id == id {
			m.questionQueue = append(m.questionQueue[:i], m.questionQueue[i+1:]...)
			return
		}
	}
}

// nextQuestion opens the oldest queued question when no picker is open.
func (m *Model) nextQuestion() {
	if m.pendingQuestion != nil || len(m.questionQueue) == 0 {
		return
	}

	q := m.questionQueue[0]
	m.questionQueue = m.questionQueue[1:]
	m.pendingQuestion = &q
}

// noteTask puts a line under a task in its run's graph: the tool it called,
// or the question it asks.
func (m *Model) noteTask(runID, task, text string) {
	last, ok := m.workflowLast[runID]
	if !ok {
		return
	}

	for i := range last.Tasks {
		if last.Tasks[i].Name == task {
			last.Tasks[i].Note = text
		}
	}
	m.workflowLast[runID] = last
	m.upsertWorkflowGraph(last)

	if m.viewport.Height > 0 {
		m.refreshFollow()
	}
}
