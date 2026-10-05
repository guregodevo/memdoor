package ui

import "strings"

// A suggestion in grey, and Tab sends it.
//
// "Recommend texts in the input, Claude CLI style: if there is a suggestion
// we show it in grey; when the user presses Tab it just submits this text
// as the user's" (Greg, 2026-09-19). The empty box shows the next thing a
// person would most likely ask for, from what just happened; Tab is the
// whole gesture — no arguments, no confirmation, the text goes as if typed.
// Nothing is suggested while the person is typing or a turn is running.

// busy reports whether a turn is running: the same signals the footer uses.
func (m Model) busy() bool {
	// turnRunning covers the gaps the other flags leave: between a tool's
	// end and the next round's first token, nothing is "thinking" and no
	// tool is active, and the suggestion appeared mid-turn with "tab sends
	// it" under a running job (the drone reel, 2026-09-19).
	return m.turnRunning || m.isThinking || m.pendingCall != "" || len(m.activeTools) > 0 || len(m.activeSubagents) > 0 || len(m.subagentWork) > 0
}

// suggestion is what Tab sends from the empty box, or "" when nothing is.
func (m Model) suggestion() string {
	_, send := m.hint()
	return send
}

// hint is the grey placeholder for the empty box and what Tab sends for it.
//
// A HINT IS THE NEXT THING, NOT A MENU (Greg, 2026-09-27: "contextual hint in
// the input. the next action. The hint for free user to upgrade. The hint for
// slash command that can be useful with jev"). One line, chosen from what just
// happened, and only while the box is empty and nothing is running:
//
//	after a turn that changed code   "Commit the change with a message that says why"   (Tab sends it)
//	after any turn, with a seat      "/usage — what the decision model kept …"           (Tab runs /usage)
//	after any turn, free             "Workflows run free here · …"                       (Tab does nothing)
//
// A pointer line is information, never a prompt: send is empty, so Tab can
// never hand a shell command to the model as a task. Nothing is
// shown before the first turn — the welcome line covers that — and nothing when
// the plan is unknown, because a nudge on a guess is a nag.
func (m Model) hint() (text, send string) {
	if strings.TrimSpace(m.input.Value()) != "" || m.busy() || m.pendingQuestion != nil {
		return "", ""
	}
	changed, replied := m.sinceLastAsk()
	if !replied {
		return "", ""
	}
	if changed {
		const commit = "Commit the change with a message that says why."
		return commit, commit
	}
	if m.decisionsOff {
		// IF THERE IS NO JEV, SAY HOW TO GET ONE (Greg, 2026-10-03: "if
		// it's not we should have a hint for users to connect"). Decisions
		// need no OpenRouter account: a TypeSafe key works with any
		// provider. Information only: Tab sends nothing.
		return "Decisions are off · memdoor connect typesafe turns them on with any provider (cheaper reads, a stop when stuck)", ""
	}
	// The decision model is on whatever the plan (2026-10-03): once the
	// status is known, /usage shows what it kept out of the bill.
	if m.plan != "" {
		return "/usage — what the decision model kept out of your bill this month", "/usage"
	}
	return "", ""
}

// sinceLastAsk reports whether the turn since the person last spoke changed
// code, and whether it answered at all.
func (m Model) sinceLastAsk() (changed, replied bool) {
	start := -1
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "user" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return false, false
	}
	for _, msg := range m.messages[start:] {
		switch msg.Role {
		case "tool_call":
			if msg.ToolName == "apply_patch" || msg.ToolName == "search_replace" || msg.ToolName == "write_file" {
				changed = true
			}
		case "assistant":
			if strings.TrimSpace(msg.Content) != "" {
				replied = true
			}
		}
	}
	return changed, replied
}
