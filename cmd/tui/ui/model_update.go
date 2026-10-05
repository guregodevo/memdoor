package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// model_update: the bubbletea Update loop.
// Split out of model.go (2026-08-28), one concern per file;
// Pattern: OpenClaw one-file-per-concern. Same package, same behavior.

// Update handles messages and updates the model
// Update runs the state machine, then flushes whatever that made final to
// terminal scrollback. Splitting it this way keeps the flush at ONE point:
// update has a couple of dozen early returns, and a flush call in each of them
// is a rule that only holds until someone adds the next return.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	wasBusy := m.busy()
	next, cmd := m.update(msg)
	nm, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	// The end of a turn, detected once, where every path through update
	// meets: running before this message, idle after it, and not stopped to
	// ask a question (that has its own "?" in the title).
	if !wasBusy && nm.busy() {
		if c := cmuxTurnStarted(); c != nil {
			cmd = tea.Batch(cmd, c)
		}
	}
	if wasBusy && !nm.busy() && nm.pendingQuestion == nil {
		if ring := (&nm).turnEnded(); ring != nil {
			cmd = tea.Batch(cmd, ring)
		}
	}
	// A page in the background (pages.go) keeps its settled messages: the
	// terminal's scrollback belongs to the page showing.
	if nm.background {
		return nm, cmd
	}
	// THE TAB SAYS WHETHER IT IS RUNNING (window_title.go). Once, here, rather
	// than at every transition — and only for the page that is showing: the
	// terminal has one title, and it belongs to what the person is looking at.
	if spin := (&nm).spin(msg); spin != nil {
		cmd = tea.Batch(cmd, spin)
	}
	if title := (&nm).titleCmd(); title != nil {
		cmd = tea.Batch(cmd, title)
	}
	nm, flush := nm.flushSettled()
	if flush == nil {
		return nm, cmd
	}
	return nm, tea.Sequence(flush, cmd)
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	// Focus, and the person's return, for the end-of-turn signal
	// (window_title.go): a key or the window regaining focus clears the ✓.
	switch msg.(type) {
	case tea.FocusMsg:
		m.blurred = false
		m.seen()
		return m, nil
	case tea.BlurMsg:
		m.blurred = true
		return m, nil
	case tea.KeyMsg:
		m.seen()
	}

	// Watchdog: any run event resets the activity clock. thinkingTickMsg checks it
	// so a lost completion event can never leave the spinner hanging forever.
	switch msg.(type) {
	case assistantThinkingMsg, assistantStreamingMsg, assistantResponseMsg,
		toolCallStartMsg, toolCallCompleteMsg, questionMsg, planProposalMsg, contextUpdateMsg,
		toolProgressMsg, subagentWorkMsg,
		subagentStartMsg, subagentEndMsg, runCompleteMsg:
		m.lastEventAt = time.Now()
	}

	// The /mcp panel (mcp_panel.go) takes its messages and, while open,
	// every key.
	if handled, cmd := m.updateMCPPanel(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.updateWorkflowPanel(msg); handled {
		return m, cmd
	}

	// The model / hosts picker (route_picker.go) takes its messages and,
	// while open, every key.
	if handled, cmd := m.updateRoutePicker(msg); handled {
		return m, cmd
	}

	// Handle autocomplete navigation keys BEFORE updating textarea
	if m.showAutocomplete {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.Type {
			case tea.KeyPgUp:
				m.autocompleteIndex -= 8
				if m.autocompleteIndex < 0 {
					m.autocompleteIndex = 0
				}
				return m, nil
			case tea.KeyPgDown:
				m.autocompleteIndex += 8
				if m.autocompleteIndex > len(m.filteredCommands)-1 {
					m.autocompleteIndex = len(m.filteredCommands) - 1
				}
				return m, nil
			case tea.KeyUp:
				if m.autocompleteIndex > 0 {
					m.autocompleteIndex--
				}
				return m, nil
			case tea.KeyDown:
				if m.autocompleteIndex < len(m.filteredCommands)-1 {
					m.autocompleteIndex++
				}
				return m, nil
			case tea.KeyTab:
				// Tab: autocomplete but don't execute (let user add arguments)
				if len(m.filteredCommands) > 0 {
					selected := m.filteredCommands[m.autocompleteIndex]
					m.input.SetValue(selected + " ")
					m.showAutocomplete = false
					m.input.CursorEnd()
				}
				return m, nil
			case tea.KeyEnter:
				// Enter: select and execute immediately (inline execution)
				if len(m.filteredCommands) > 0 {
					selected := m.filteredCommands[m.autocompleteIndex]
					m.showAutocomplete = false

					// These REQUIRE an argument — complete into the input instead
					// of executing, so the user types it. (/model's arg is
					// optional — selecting it lists models.)
					// Echo the chosen command, then route the special ones the
					// same way the regular Enter path does.
					m.messages = append(m.messages, Message{Role: "user", Content: selected, Timestamp: time.Now()})
					m.input.Reset()
					// /go runs the planner's plan with the coder (autocomplete path).
					if selected == "/go" || selected == "/run" || selected == "/approve" {
						return m, m.runPlan()
					}
					if _, prompt := m.mcpPromptFor(selected); isNetworkCmd(selected) || prompt {
						cmd := m.runNetworkCmd(strings.Fields(selected))
						m.viewport.SetContent(m.renderMessages())
						m.gotoBottom()
						return m, cmd
					}
					// Handle the remaining built-ins (the user message + input
					// reset already happened above).
					expandedText, handled := m.handleSlashCommand(selected)
					if handled {
						// Command handled locally - response already added
						m.viewport.SetContent(m.renderMessages())
						m.gotoBottom()

						// Special handling for /exit command - trigger quit
						if strings.TrimSpace(selected) == "/exit" {
							return m, tea.Quit
						}
					} else {
						// Custom command / skill command — send to the SAME agent a
						// typed message goes to (m.codeAgent), not the companion:
						// a "/skill" invoked from autocomplete must run in the coder
						// session where its receipts and workdir live.
						m.interrupted = false // new turn — accept its events again
						cmd := m.dispatch(m.codeAgent, expandedText)
						m.isThinking = true
						m.thinkingStartTime = time.Now()
						m.viewport.SetContent(m.renderMessages())
						m.gotoBottom()
						return m, tea.Batch(cmd, m.tickThinking())
					}
				}
				return m, nil
			case tea.KeyEsc:
				// Close autocomplete
				m.showAutocomplete = false
				return m, nil
			}
		}
	}

	// SHIFT+TAB CYCLES THE REASONING EFFORT (omp's app.thinking.cycle): auto,
	// low, medium, high, auto. Auto lets the decision model pick per request
	// (gateway/turn_effort.go); a chosen level holds for the conversation.
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyShiftTab && m.pendingQuestion == nil && m.effortOp != nil {
		next := nextEffort(m.status.Effort)
		m.status.Effort = next
		op := m.effortOp
		return m, func() tea.Msg {
			if _, err := op(next); err != nil {
				return noteResultMsg{err: fmt.Errorf("effort not changed: %w", err)}
			}
			return statusTickMsg{}
		}
	}

	// TAB SENDS THE SUGGESTION. The grey text in the empty box is the next
	// thing a person would ask for (suggest.go); Tab puts it in the box and
	// takes the exact path Enter takes, gate and all.
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyTab && !m.showAutocomplete && m.pendingQuestion == nil {
		if s := m.suggestion(); s != "" {
			m.input.SetValue(s)
			return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		}
		return m, nil // a bare Tab never types a tab into a sentence
	}

	// Don't feed Enter to the textarea: the input is single-line and Enter is the
	// submit key, not a newline. Otherwise the textarea inserts a stray newline
	// before the KeyEnter handler reads the value (trailing "\n" in sent text, and
	// a lone "\n" if Enter is pressed on empty input).
	// Interactive question picker (ask_user_question): up/down move, enter selects, a
	// number key selects directly, Esc cancels. Captured before the input field.
	// /connect takes the keys it needs (the kind picker, Enter on a step,
	// Esc) and leaves the rest to the input line (connect_flow.go).
	if k, ok := msg.(tea.KeyMsg); ok && m.connect != nil {
		if handled, cmd := m.connectKey(k); handled {
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
			return m, cmd
		}
	}
	if r, ok := msg.(connectResultMsg); ok {
		m.connectDone(r)
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, nil
	}

	if k, ok := msg.(tea.KeyMsg); ok && m.pendingQuestion != nil {
		q := m.pendingQuestion
		switch {
		case k.Type == tea.KeyUp:
			q.index = (q.index - 1 + len(q.options)) % len(q.options)
			m.viewport.SetContent(m.renderMessages())
			return m, nil
		case k.Type == tea.KeyDown:
			q.index = (q.index + 1) % len(q.options)
			m.viewport.SetContent(m.renderMessages())
			return m, nil
		case k.Type == tea.KeyEnter:
			return m, m.answerQuestion(q.options[q.index])
		case k.Type == tea.KeyEsc:
			return m, m.answerQuestion(q.options[q.index])
		case k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] >= '1' && k.Runes[0] <= '9':
			if n := int(k.Runes[0] - '1'); n < len(q.options) {
				return m, m.answerQuestion(q.options[n])
			}
			return m, nil
		}
	}

	// The @file picker takes the arrow keys, tab, enter and esc while it is
	// open (mentions.go).
	if k, ok := msg.(tea.KeyMsg); ok && m.showFileMentions {
		switch k.Type {
		case tea.KeyUp:
			if m.fileIndex > 0 {
				m.fileIndex--
			}
			return m, nil
		case tea.KeyDown:
			if m.fileIndex < len(m.fileMatches)-1 {
				m.fileIndex++
			}
			return m, nil
		case tea.KeyTab, tea.KeyEnter:
			if len(m.fileMatches) > 0 {
				m.input.SetValue(insertMention(m.input.Value(), m.mentionStart, m.fileMatches[m.fileIndex]))
				m.input.CursorEnd()
			}
			m.showFileMentions = false
			return m, nil
		case tea.KeyEsc:
			m.showFileMentions = false
			return m, nil
		}
	}
	// ctrl+v with an image on the clipboard: the image is saved under
	// .memdoor and mentioned in the prompt (paste.go). Text on the clipboard
	// goes on to the box's own paste.
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyCtrlV && m.pendingQuestion == nil {
		if wd, err := os.Getwd(); err == nil {
			if path, err := pasteImage(wd); err == nil {
				v := m.input.Value()
				if v != "" && !strings.HasSuffix(v, " ") && !strings.HasSuffix(v, "\n") {
					v += " "
				}
				m.input.SetValue(v + "@" + path + " ")
				m.input.CursorEnd()
				m.note("Image attached: " + filepath.Base(path))
				return m, nil
			}
		}
	}
	// A newline in the prompt: alt+enter or ctrl+j (shift+enter is plain
	// enter to most terminals), and a line ending in "\" on enter.
	if k, ok := msg.(tea.KeyMsg); ok && ((k.Type == tea.KeyEnter && k.Alt) || k.Type == tea.KeyCtrlJ) {
		m.input.InsertString("\n")
		m.growInput()
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyEnter && m.pendingQuestion == nil && !m.showAutocomplete {
		if v, cont := newlineOnBackslash(m.input.Value()); cont {
			m.input.SetValue(v)
			m.input.CursorEnd()
			m.growInput()
			return m, nil
		}
	}

	if k, ok := msg.(tea.KeyMsg); !ok || k.Type != tea.KeyEnter {
		m.input, tiCmd = m.input.Update(msg)
		m.growInput()
	}
	m.viewport, vpCmd = m.viewport.Update(msg)
	if m.newBelow > 0 && m.viewport.AtBottom() {
		m.newBelow = 0 // scrolled back down manually — caught up
	}

	// Autocomplete applies only while the command TOKEN is being typed: a
	// leading "/" with no space yet. Once a space is typed (the user is entering
	// arguments, e.g. "/model groq:qwen3.8-27b") or nothing matches, hide it — otherwise
	// an open dropdown with no matches swallows Enter and the command can't run.
	inputValue := m.input.Value()
	if (inputValue == "/" || looksLikeSlashCommand(inputValue)) && !strings.Contains(strings.TrimSpace(inputValue), " ") {
		// Refresh the command list when the dropdown OPENS (bare "/"): skills
		// created mid-session — including by the coder itself — appear without
		// a TUI restart. One readdir per dropdown open, not per keystroke.
		if inputValue == "/" {
			m.slashCommands = loadSlashCommands(m.codeAgent)
		}
		m.filteredCommands = filterCommands(m.slashCommands, inputValue)
		m.showAutocomplete = len(m.filteredCommands) > 0
		if m.autocompleteIndex >= len(m.filteredCommands) {
			m.autocompleteIndex = 0
		}
	} else {
		m.showAutocomplete = false
	}
	m.refreshMentions()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnd:
			// Jump back to the latest output — the "scroll down" button for a user
			// who scrolled up (wheel/PgUp) to read history while a long run streams.
			m.gotoBottom()
			return m, nil

		case tea.KeyCtrlO:
			// Toggle expanded tool frames: show full tool outputs (questions, todo
			// notes, build errors) instead of the truncated preview.
			m.expandTools = !m.expandTools
			if m.expandTools {
				m.note("tool frames expanded — ctrl+o to collapse")
			} else {
				m.note("tool frames collapsed — ctrl+o to expand")
			}
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
			return m, nil

		case tea.KeyEsc:
			if m.showAutocomplete {
				m.showAutocomplete = false
				return m, nil
			}
			// Esc interrupts a running turn: stop the spinner and drop the in-flight
			// run's remaining output, returning to ready. (Ctrl+C quits the app.)
			// busy(), not isThinking: isThinking goes false as soon as a reply
			// streams and no tool is active between rounds, so Esc did nothing
			// for most of a turn and a runaway could not be stopped (live
			// 2026-09-28: "escape dont work", a 60-call turn ran to a gateway
			// restart).
			// Esc is also the way out of this window's workflow run (Greg,
			// 2026-10-02: "the user can escape using its esc"): the gateway
			// stops it and reports it, like any failed or stopped DAG.
			if id := m.workflowRunning; id != "" && m.workflow.Stop != nil {
				m.workflowRunning = ""
				stop := m.workflow.Stop
				return m, func() tea.Msg { r, err := stop(id); return workflowRunMsg{run: r, err: err} }
			}
			if m.busy() || len(m.queued) > 0 {
				m.stopLocally()
				// Tell the gateway to actually stop the generation (not just the UI).
				if m.connected {
					_ = m.ws.SendCancel()
				}
				m.messages = append(m.messages, Message{
					Role:      "system",
					Content:   interruptedNote,
					Timestamp: time.Now(),
				})
				m.viewport.SetContent(m.renderMessages())
				m.gotoBottom()
				return m, nil
			}
			// Nothing running — clear the input instead of quitting.
			m.input.SetValue("")
			return m, nil

		case tea.KeyEnter:
			// Send message (autocomplete selection is handled above). Trim so a
			// stray newline / whitespace-only input never submits.
			if text := strings.TrimSpace(m.input.Value()); text != "" {
				// THE BRAIN GATE: nothing is sent to a brain that cannot answer.
				// Slash commands still run — /usage, /model and the rest are
				// the gateway's, not the brain's. The text stays in the box.
				if m.status.Gate != "" && !looksLikeSlashCommand(text) {
					m.note("⏳ " + m.status.Gate)
					m.viewport.SetContent(m.renderMessages())
					m.viewport.GotoBottom()
					return m, nil
				}
				originalText := text // Keep original for display

				// /go (/run, /approve): execute the planner's plan — hand it to the
				// coder, whose bash/write_file tool events then stream into the TUI.
				if text == "/go" || text == "/run" || text == "/approve" {
					m.input.Reset()
					m.messages = append(m.messages, Message{Role: "user", Content: text, Timestamp: time.Now()})
					return m, m.runPlan()
				}

				// Check if this is a slash command
				if looksLikeSlashCommand(text) {
					// Add user message FIRST to show what they typed
					userMsg := Message{
						Role:      "user",
						Content:   originalText,
						Timestamp: time.Now(),
					}
					m.messages = append(m.messages, userMsg)

					// /model, /usage and /update do network or
					// background work — run them async (a tea.Cmd) so the event
					// loop never blocks.
					if fields := strings.Fields(text); isNetworkCmd(fields[0]) || m.isMCPPrompt(fields[0]) {
						m.input.Reset()
						cmd := m.runNetworkCmd(fields)
						m.viewport.SetContent(m.renderMessages())
						m.gotoBottom()
						return m, cmd
					}

					// Handle slash command expansion/execution
					expandedText, handled := m.handleSlashCommand(text)
					if handled {
						// Command was handled locally (like /compact, /help, /exit)
						// Assistant response already added by handleSlashCommand
						m.input.Reset()
						m.viewport.SetContent(m.renderMessages())
						m.gotoBottom()

						// Special handling for /exit command - trigger quit
						if strings.TrimSpace(originalText) == "/exit" {
							return m, tea.Quit
						}
						return m, nil
					}
					// Command was expanded (replaced with file content)
					text = expandedText
				} else {
					// Regular message (not a slash command)
					userMsg := Message{
						Role:      "user",
						Content:   text,
						Timestamp: time.Now(),
					}
					m.messages = append(m.messages, userMsg)
					m.setTitle(text)
				}

				m.input.Reset()
				m.viewport.SetContent(m.renderMessages())
				m.gotoBottom()

				// OFFLINE: never let a prompt vanish into a dead connection — queue
				// it; the reconnect handler flushes the queue when the link is back.
				if !m.connected {
					m.queued = append(m.queued, text)
					m.note(fmt.Sprintf("⚠ offline — queued (%d); will send when reconnected.", len(m.queued)))
					m.viewport.SetContent(m.renderMessages())
					m.gotoBottom()
					return m, nil
				}
				// If a turn is already running, QUEUE this one (Claude-CLI style):
				// keep accepting input, send it batched when the agent is free.
				// busy(), not isThinking: typed between rounds, a prompt was
				// POSTED as a new turn the gateway held behind the running one —
				// out of the TUI's reach, so Esc could not cancel it either.
				if m.busy() {
					m.queued = append(m.queued, text)
					m.note(fmt.Sprintf("Queued (%d) — will send when the current reply finishes.", len(m.queued)))
					m.viewport.SetContent(m.renderMessages())
					m.gotoBottom()
					return m, nil
				}
				// Idle with input still queued (a turn that ended without a
				// reply to flush it): send it now, this prompt last — never
				// queue behind a turn that is not running.
				if len(m.queued) > 0 {
					m.queued = append(m.queued, text)
					return m, m.flushQueue()
				}
				// One agent (the coder) runs every turn — always-auto, so the
				// permission mode is fixed.
				m.interrupted = false // new turn — accept its events again
				// "@planner outline this" runs the planner, not the coder.
				agent, body := routeByMention(m.codeAgent, text)
				cmd := m.dispatch(agent, body)
				// Show the thinking spinner immediately. The run's first quick
				// event is lifecycle:start (renders nothing); the assistant
				// "thinking" event only fires AFTER forced grounding, which on a
				// cold model is 30–60s — without this the TUI looks frozen.
				m.isThinking = true
				m.thinkingStartTime = time.Now()
				return m, tea.Batch(cmd, m.tickThinking())
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 4 // Reduced from 5 (input now 1 line instead of 3)
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.KeyMap = transcriptKeys()
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom() // start at the latest, not at line 0 of the transcript
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		m.input.SetWidth(msg.Width - 4)

	case websocketConnectedMsg:
		wasDown := m.everConnected && !m.connected
		m.connected = true
		m.everConnected = true
		m.err = nil
		if wasDown {
			// RECONNECTED after a drop. Be honest about the emitter race: events
			// streamed while we were down are GONE — including a running turn's
			// completion, which would otherwise leave the spinner stuck until the
			// watchdog. Reconnect means fresh run state: clear it, tell the user,
			// and flush anything typed while offline.
			m.isThinking = false
			m.turnRunning = false
			m.activeTools = nil
			m.closeOrphanFrames("connection lost — result not received")
			m.activeSubagents = nil
			m.subagentWork = nil
			m.note("✓ reconnected — events sent while offline were missed (a running turn may have finished; re-ask if unsure)")
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
			if len(m.queued) > 0 {
				return m, m.flushQueue()
			}
		}
		// Adapter already started listening in Connect()
		return m, nil

	case websocketDisconnectedMsg:
		// Show the break LOUDLY — a silently dead connection that eats input is
		// worse than a crash. The client auto-reconnects with backoff; typed
		// prompts queue while offline and flush on reconnect.
		if m.connected {
			m.note("⚠ gateway connection lost — reconnecting… (typed prompts will queue)")
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
		}
		m.connected = false
		m.err = msg.err
		return m, nil

	case assistantThinkingMsg:
		if m.interrupted {
			return m, nil
		}
		// A thinking event that carries reasoning is worth SHOWING — it is often
		// the only record of why the model did what it did next, and when a turn
		// is nothing but reasoning it is the only record at all.
		// In the app the deliberation stays behind the spinner: a coach asked
		// for a short, not for the editor's notes to self.
		if strings.TrimSpace(msg.text) != "" {
			m.messages = append(m.messages, Message{
				Role: "thinking", Content: strings.TrimSpace(msg.text), Timestamp: time.Now(),
			})
		}
		// Set thinking state (will display in sticky bar at bottom)
		m.isThinking = true
		m.turnRunning = true
		m.thinkingStartTime = time.Now()
		// Start ticker to refresh thinking indicator
		return m, m.tickThinking()

	case clearedMsg:
		if msg.err != nil {
			m.note("Could not wipe the agent's memory: " + msg.err.Error())
		} else {
			// The screen starts over with the agent. The scrollback watermark
			// and the block cache are indexed by position in m.messages, so
			// both go with it — otherwise message 0 of the new session
			// inherits message 0's cached render and is treated as printed.
			m.messages = make([]Message, 0)
			m.printedThrough = 0
			m.blocks = blockCache{}
			m.note("Cleared: the agent's memory of this conversation is wiped, and its next turn starts from nothing. " +
				"The messages are still there for `memdoor resume`.")
		}
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, nil

	case freshMsg:
		if msg.err != nil {
			m.note("Could not start over: " + msg.err.Error() + " (the conversation is as it was)")
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
			return m, nil
		}
		lead := "Fresh session: the agent forgot this conversation."
		if msg.handoff != "" {
			lead = "Handed off. The next conversation starts from this summary, and the old one is gone:\n\n" + msg.handoff + "\n"
		}
		if msg.task == "" {
			if msg.handoff == "" {
				lead += " Its next turn starts from nothing."
			}
			m.note(lead)
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
			return m, nil
		}
		m.note(lead + " Your request goes into it:")
		return m, m.startTurn(msg.task)

	case cronAnswerMsg:
		// A scheduled check reporting back. It is not a turn of this window —
		// no spinner, no "esc to interrupt" — just what it found, in the flow.
		m.note(msg.text)
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, nil

	case noteResultMsg:
		if msg.err != nil {
			m.note("" + msg.err.Error())
		} else {
			m.note(msg.summary)
		}
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, nil

	case updateResultMsg:
		if msg.err != nil {
			m.note("⚠ Update failed: " + msg.err.Error())
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
			return m, nil
		}
		m.note("✓ " + msg.summary + " Restarting into this conversation …")
		m.restartAfterUpdate = true
		return m, tea.Quit

	case routeResultMsg:
		if msg.err != nil {
			m.note(msg.err.Error())
		} else {
			m.note(msg.summary)
			// The rung changed: the footer says so now, not at the next
			// tick, and the model named is the rung's until a turn answers.
			m.answeredBy = ""
		}
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, func() tea.Msg { return statusTickMsg{} }

	case questionMsg:
		// The agent asked an interactive question and is blocked on the answer.
		m.pendingQuestion = &pendingQuestion{id: msg.id, question: msg.question, options: msg.options}
		m.messages = append(m.messages, Message{Role: "system", Content: "" + msg.question, Timestamp: time.Now()})
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, nil

	case planProposalMsg:
		// exit_plan_mode presented a plan. Render it as a clean proposal block. In
		// plan mode the approval prompt follows at run-complete (a/e/r); in other
		// modes it's just an informational plan the agent proceeds to implement.
		m.messages = append(m.messages, Message{Role: "system", Content: "Proposed plan\n\n" + strings.TrimSpace(msg.plan), Timestamp: time.Now()})
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, nil

	case sendResultMsg:
		// The post itself failed (gateway down / not a member). The run's own
		// events stream separately, so we only surface failures here.
		if msg.err != nil {
			m.isThinking = false
			m.turnRunning = false
			m.note("Couldn't reach the agent: " + msg.err.Error())
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
		}
		return m, nil

	case assistantResponseMsg:
		if m.interrupted {
			return m, nil
		}
		// Clear thinking state
		m.isThinking = false
		// The reply is complete: let it render as markdown again, and reset the
		// think splitter so the next turn starts outside any <think> block.
		if held := m.jsonFilter.flush(); held != "" {
			m.appendStream("assistant", held)
		}
		m.jsonFilter = jsonCallFilter{}
		m.pendingCall = "" // the turn is over; nothing is being written
		m.settleStreaming()
		m.thinkSplit = thinkStreamSplitter{}

		// Check if we already have an assistant message (from streaming)
		if msg.appended && strings.TrimSpace(msg.content) != "" {
			// The gateway sent ONLY what did not stream (the receipt line, a
			// note): it goes under the streamed text. The replace branch below
			// erased a zero-tool turn's whole streamed answer behind its note
			// (live 2026-10-04).
			if n := len(m.messages); n > 0 && m.messages[n-1].Role == "assistant" {
				m.messages[n-1].Content = strings.TrimRight(m.messages[n-1].Content, "\n") + "\n\n" + msg.content
			} else {
				m.messages = append(m.messages, Message{Role: "assistant", Content: msg.content, Timestamp: time.Now()})
			}
		} else if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
			// The final text replaces what streamed — but only when there IS
			// one. A turn cut off at the output cap completes with EMPTY
			// content, and overwriting with that erased every word the reader
			// had just watched arrive. Measured 2026-08-30: a six-minute reply
			// truncated mid-call left a blank assistant message behind.
			if strings.TrimSpace(msg.content) != "" && m.messages[len(m.messages)-1].Content != msg.content {
				m.messages[len(m.messages)-1].Content = msg.content
			}
		} else if tail, dup := m.finalBeyondStreamed(msg.content); dup {
			// The final text is what already streamed, split around a tool
			// frame: "I've transcribed the file…" rendered above the frame as it
			// arrived and again below it as the reply (live 2026-09-03). Only what did not stream is new.
			if tail != "" {
				m.messages = append(m.messages, Message{Role: "assistant", Content: tail, Timestamp: time.Now()})
			}
		} else {
			// Create new message if no existing assistant message
			responseMsg := Message{
				Role:      "assistant",
				Content:   msg.content,
				Timestamp: time.Now(),
			}
			m.messages = append(m.messages, responseMsg)
		}
		m.roundClosed = true
		// Recommend a next command when the answer signals a recall gap.
		// Send anything the user queued while this turn was running, batched.
		flush := m.flushQueue()
		m.refreshFollow()
		return m, flush

	case executionFailedMsg:
		m.isThinking = false
		m.turnRunning = false
		m.activeTools = nil
		m.compacting = false
		if held := m.jsonFilter.flush(); held != "" {
			m.appendStream("assistant", held)
		}
		m.jsonFilter = jsonCallFilter{}
		m.pendingCall = ""
		// AN INTERRUPT IS NOT A FAILURE (battle test, 2026-09-27). Pressing
		// esc cancelled the request, and the window reported it as
		// `oai request failed: Post "https://openrouter.ai/…": context
		// canceled` — a URL and a Go error for something the person just did
		// on purpose.
		failed := "⚠ The turn failed: " + msg.errText
		if wasInterrupted(msg.errText) {
			failed = "⏹ Interrupted. Say what to do instead."
		}
		if n := len(m.messages); n == 0 || m.messages[n-1].Content != failed {
			m.messages = append(m.messages, Message{
				Role:      "assistant",
				Content:   failed,
				Timestamp: time.Now(),
			})
		}
		m.settleStreaming()
		m.refreshFollow()
		return m, nil

	case runCompleteMsg:
		// Which model answered, on every turn: a dim line under the answer
		// and the footer until the next turn (Greg, 2026-09-26).
		if msg.model != "" {
			m.answeredBy = msg.model
			m.status.Model = msg.model
			m.note("answered by " + msg.model)
			m.viewport.SetContent(m.renderMessages())
		}
		// The run ended. Normally the assistant message already cleared these a
		// moment earlier (lifecycle-complete arrives just before the message), so
		// this is a no-op then; its job is the SILENT turn where no message comes.
		// Deliberately does NOT flush the queue — the assistant handler owns that
		// for normal turns (flushing here would send queued input before the
		// answer renders). Queued input after a silent turn waits for the user.
		m.isThinking = false
		m.turnRunning = false
		m.activeTools = nil
		m.compacting = false
		// A SPAWNED run is deliberately NOT cleared here. sessions_spawn
		// enqueues and returns, so the requester's turn ends while the job it
		// asked for is just beginning: clearing on run-complete would blank
		// the screen for exactly the ten minutes the work takes. The child's
		// own "done" event clears it (agent_runtime_progress.go).
		//
		// A turn can end here WITHOUT an assistant message (the silent turn),
		// and if its last output was a half-written tool object the held-call
		// indicator was never cleared — "Writing a glob call… (9m 0s)" pinned
		// over a session doing something else (live, 2026-08-31 12:39). The
		// run is over; nothing is being written.
		if held := m.jsonFilter.flush(); held != "" {
			m.appendStream("assistant", held)
		}
		m.jsonFilter = jsonCallFilter{}
		m.pendingCall = ""
		return m, flushQueueSoon()

	case queueFlushMsg:
		// The run-complete follow-up: input queued during a turn that ended
		// without a reply (the reply's own handler flushes the normal case)
		// goes out once nothing is running.
		if !m.busy() && len(m.queued) > 0 {
			return m, m.flushQueue()
		}
		return m, nil

	case contextUpdateMsg:
		m.contextTokens = msg.tokens
		m.contextLimit = msg.limit
		m.contextPercent = msg.percent
		m.contextParts = msg.parts
		return m, nil

	case thinkingTickMsg:
		// The clock must keep running for a SPAWNED job too. The requester's
		// own turn is over the moment it delegates (sessions_spawn enqueues
		// and returns), so gating the tick on isThinking froze the label and
		// its elapsed at the second the parent finished — the screen would
		// say "coder · bash 0s" for ten minutes.
		// …and for the turn itself: the bar follows turnRunning (model_view.go),
		// so the clock and the spinner frame must move for as long as it does.
		if m.isThinking || m.turnRunning || len(m.subagentWork) > 0 {
			// Watchdog: if no run activity for a while, clear the spinner so a lost
			// completion event can't hang the UI forever. thinkingStartTime is the
			// baseline until the first event arrives; any event resets lastEventAt.
			last := m.lastEventAt
			if last.Before(m.thinkingStartTime) {
				last = m.thinkingStartTime
			}
			// The watchdog is a safety net for a LOST completion event, not a
			// progress timer. A tool call being written streams no text deltas,
			// so a slow model writing a large file legitimately goes minutes with
			// no event — 150s tripped on healthy runs. Give it a realistic
			// ceiling before declaring a stall.
			if time.Since(last) > watchdogStallTimeout {
				m.isThinking = false
				m.turnRunning = false
				m.activeTools = nil
				m.subagentWork = nil
				m.note("No response for several minutes — the run likely finished or stalled. Check messages, or /new to reset the session.")
				m.viewport.SetContent(m.renderMessages())
				m.gotoBottom()
				return m, nil
			}
			// Animate the running-tool spinner: re-render the messages each tick so
			// the braille frame advances. Only while a tool is actually running, and
			// no GotoBottom, so it doesn't yank a scrolled-up user to the bottom.
			if len(m.activeTools) > 0 {
				m.viewport.SetContent(m.renderMessages())
			}
			return m, m.tickThinking()
		}
		return m, nil

	case assistantStreamingMsg:
		if m.interrupted {
			return m, nil
		}
		// Clear thinking state when streaming starts
		m.isThinking = false

		// ROUTE REASONING TO THE THINKING BLOCK WHILE IT STREAMS.
		//
		// The model's <think> block arrives as ordinary deltas, and the reply is
		// only classified as reasoning AFTER the turn ends (replyContent). So a
		// streamed turn rendered the model's whole thought process as if it were
		// the answer — pages of "Let me think about what this means" above the
		// actual reply.
		//
		// splitThinkStream tracks the open/close markers across chunk boundaries
		// (a marker can be split between two deltas) and returns the two parts,
		// so each goes to the role that renders it.
		think, answer := m.thinkSplit.next(msg.content)
		if think != "" {
			m.appendStream("thinking", think)
		}
		// Hide a bare-JSON tool call while it arrives: the gateway turns it into
		// a real frame, so showing the raw object as well is duplication.
		if answer = m.jsonFilter.feed(answer); answer != "" {
			m.appendStream("assistant", answer)
		}
		// Withholding the half-written call is right; withholding it in silence
		// is what looked like a hang for six minutes. Publish what is being held
		// so the activity bar can name it.
		m.pendingCall = ""
		m.pendingCallBytes = 0
		if held := m.jsonFilter.Held(); held > 0 {
			m.pendingCallBytes = held
			if m.pendingCall = m.jsonFilter.PendingName(); m.pendingCall == "" {
				m.pendingCall = pendingCallUnnamed
			}
		}
		// REVERTED to a repaint per delta (2026-08-30).
		//
		// Coalescing these into a 100ms tick was an attempt to stop text tearing
		// on screen. It did not fix it — the tearing came back in a different
		// shape — and it caused a worse regression on the way: tool frames
		// stopped rendering. Every component tests clean (the gateway's own copy
		// of the text has no corruption, the splitter round-trips byte-exactly,
		// appendStream assembles correctly, and the renderer introduces nothing),
		// so the cause is NOT located and coalescing was not it.
		//
		// Back to the known-previous behaviour until there is evidence for a
		// specific fix rather than a plausible one.
		m.refreshFollow()
		return m, nil

	case toolCallStartMsg:
		if m.interrupted {
			return m, nil
		}
		// Track active tool
		if m.activeTools == nil {
			m.activeTools = make(map[string]time.Time)
		}
		m.activeTools[msg.toolName] = time.Now()

		// Update activity start time if this is the first tool
		if len(m.activeTools) == 1 {
			m.activityStartTime = time.Now()
		}

		// Check if this tool call already exists (prevent duplicates). With an
		// id that is an exact question; without one, fall back to the old
		// name+input guess so an older gateway still renders.
		exists := false
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role != "tool_call" {
				continue
			}
			if msg.toolID != "" {
				if m.messages[i].ToolID == msg.toolID {
					exists = true
					break
				}
				continue
			}
			if m.messages[i].ToolName == msg.toolName &&
				m.messages[i].ToolInput == msg.toolInput &&
				!m.messages[i].toolSettled() {
				exists = true
				break
			}
		}

		// Only add if it doesn't exist
		if !exists {
			toolCallMsg := Message{
				Role:      "tool_call",
				ToolID:    msg.toolID,
				ToolName:  msg.toolName,
				ToolInput: msg.toolInput,
				Timestamp: time.Now(),
			}
			m.messages = append(m.messages, toolCallMsg)
			m.refreshFollow()
		}

		// Start animation ticker for spinner (even if duplicate, keep animating)
		return m, m.tickAnimation()

	case toolOutputDeltaMsg:
		if m.interrupted {
			return m, nil
		}
		// Append the chunk to the running tool's live buffer (bash tail). Match
		// the last still-running frame for this tool (ToolOutput == "").
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == "tool_call" && m.messages[i].ToolName == msg.toolName && !m.messages[i].toolSettled() {
				// The event carries the output SO FAR (the whole buffer), so
				// replace — appending would duplicate everything each tick.
				m.messages[i].LiveOutput = msg.chunk
				m.messages[i].Streamed = true
				break
			}
		}
		m.viewport.SetContent(m.renderMessages())
		m.refreshFollow()
		return m, nil

	case toolProgressMsg:
		// The tool is still running. Nothing to render in the transcript —
		// the value is that the run is DEMONSTRABLY alive: this resets the
		// stall watchdog above, and adopts the tool if its "start" never
		// arrived (a reconnect mid-tool used to leave the bar idle).
		if m.interrupted {
			return m, nil
		}
		if m.activeTools == nil {
			m.activeTools = make(map[string]time.Time)
		}
		if _, known := m.activeTools[msg.toolName]; !known {
			m.activeTools[msg.toolName] = time.Now().Add(-time.Duration(msg.seconds) * time.Second)
			if m.activityStartTime.IsZero() {
				m.activityStartTime = m.activeTools[msg.toolName]
			}
		}
		return m, nil

	case subagentWorkMsg:
		// A run this screen spawned, saying what it is doing. It lives in the
		// status line only: its tool frames belong to its own transcript.
		if m.interrupted {
			return m, nil
		}
		if msg.done {
			delete(m.subagentWork, msg.sessionID)
			return m, nil
		}
		if m.subagentWork == nil {
			m.subagentWork = make(map[string]subagentWorkState)
		}
		// Nothing else is animating: start the clock and the tick loop, or the
		// bar appears once and then freezes.
		first := len(m.subagentWork) == 0 && !m.isThinking
		if first && len(m.activeTools) == 0 {
			m.activityStartTime = time.Now()
		}
		m.subagentWork[msg.sessionID] = subagentWorkState{agent: msg.agent, tool: msg.tool, seconds: msg.seconds}
		if first {
			return m, m.tickThinking()
		}
		return m, nil

	case toolCallCompleteMsg:
		if m.interrupted {
			return m, nil
		}
		// Remove from active tools
		delete(m.activeTools, msg.toolName)

		// Attach the result to the frame that ASKED for it.
		//
		// Matching on tool NAME cannot tell concurrent calls apart: it took the
		// last unfilled frame with that name, so with three bash calls in one
		// reply the first result landed on the third frame. Measured
		// 2026-08-30 — `Bash(ls -la && pwd && python3 --version)` rendered
		// "bash: python: command not found", which is another call's output.
		//
		// The id is exact. Name matching remains only as the fallback for a
		// gateway too old to send one.
		if len(m.messages) > 0 {
			for i := len(m.messages) - 1; i >= 0; i-- {
				if m.messages[i].Role != "tool_call" {
					continue
				}
				matched := m.messages[i].ToolID == msg.toolID && msg.toolID != ""
				if msg.toolID == "" {
					matched = m.messages[i].ToolName == msg.toolName && !m.messages[i].ToolDone
				}
				if matched {
					m.messages[i].ToolDone = true
					m.messages[i].ToolOutput = msg.toolOutput
					m.messages[i].ToolError = msg.toolError
					m.messages[i].ToolTook = time.Since(m.messages[i].Timestamp)
					m.messages[i].LiveOutput = "" // the compact final view takes over
					break
				}
			}
		}
		m.refreshFollow()
		return m, nil

	case statusTickMsg:
		if m.statusOp != nil {
			m.status = m.statusOp()
			// The footer names the model that answers this session's agent;
			// once a turn has answered, the model that actually did.
			if mdl := m.status.AgentModels[m.codeAgent]; mdl != "" {
				m.status.Model = mdl
			}
			if m.answeredBy != "" {
				m.status.Model = m.answeredBy
			}
		}
		if m.status.Gate != "" {
			return m, statusTickSoon()
		}
		return m, m.tickStatus()

	case animationTickMsg:
		// Check if any tools are still running
		hasRunningTool := false
		for _, msg := range m.messages {
			if msg.Role == "tool_call" && !msg.toolSettled() {
				hasRunningTool = true
				break
			}
		}

		// Refresh viewport to update spinner animation
		m.viewport.SetContent(m.renderMessages())

		// Continue ticking if tools are running
		if hasRunningTool {
			return m, m.tickAnimation()
		}
		return m, nil

	case SetProgramMsg:
		m.program = msg.Program
		// Set program on adapter so it can send Bubble Tea messages
		if m.ws != nil {
			m.ws.SetSink(m.sender(msg.Program))
		}
		return m, nil

	case subagentStartMsg:
		// Track active subagent
		if m.activeSubagents == nil {
			m.activeSubagents = make(map[string]string)
		}
		m.activeSubagents[msg.sessionID] = msg.task

		// Update activity start time if this is the first subagent
		if len(m.activeSubagents) == 1 && len(m.activeTools) == 0 && !m.isThinking {
			m.activityStartTime = time.Now()
		}
		return m, nil

	case subagentEndMsg:
		// Remove from active subagents
		delete(m.activeSubagents, msg.sessionID)
		return m, nil

	case providerNoticeMsg:
		m.messages = append(m.messages, Message{Role: "system", Content: msg.text, Timestamp: time.Now()})
		m.viewport.SetContent(m.renderMessages())
		m.gotoBottom()
		return m, nil

	case compactionMsg:
		m.compacting = msg.active
		m.compactedTokens = [2]int{msg.before, msg.after}
		if msg.active {
			// Keep the activity bar animating while compaction runs.
			if m.activityStartTime.IsZero() {
				m.activityStartTime = time.Now()
			}
			return m, m.tickAnimation()
		}
		// Completed: leave a persistent dim note so the user sees it happened
		// (the activity bar is transient). Only when we have real numbers.
		if msg.before > 0 && msg.after > 0 {
			m.messages = append(m.messages, Message{
				Role:      "system",
				Content:   fmt.Sprintf("Compacted context — %s → %s tokens", formatTokens(msg.before), formatTokens(msg.after)),
				Timestamp: time.Now(),
			})
			m.viewport.SetContent(m.renderMessages())
			m.gotoBottom()
		}
		return m, nil

	case todoUpdateMsg:
		// Update todo list
		m.todos = msg.todos
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// finalBeyondStreamed compares the turn's final reply with the assistant text
// that streamed since the user's message. When the final text repeats the
// streamed text (possibly with more after it), dup is true and tail is the
// part that did not stream.
func (m *Model) finalBeyondStreamed(final string) (tail string, dup bool) {
	// SEGMENTS SPLIT AROUND TOOL FRAMES ARE ONE REPLY. Each round's prose
	// streams as its own assistant message with a tool frame between; the
	// final reply joins the rounds with a newline. Concatenating the segments
	// with nothing between them made "…it is.Let me" against "…it is.\nLet
	// me", the prefix test failed, and the whole reply was appended again
	// under the last frame — every sentence of the turn, twice (Montebourg,
	// 2026-09-19). Compare on words, not bytes.
	var segments []string
	start := 0
	for i := len(m.messages) - 1; i >= 0; i-- {
		// A slash command typed mid-turn (/copy, /usage) is echoed as a
		// user message but starts no turn: with it as the boundary, every
		// paragraph streamed before it was invisible here and each round's
		// text came back a second time under its frame (2026-09-26, after a
		// /copy during the history-endpoint turn).
		if m.messages[i].Role == "user" && !isSlashEcho(m.messages[i]) {
			start = i + 1
			break
		}
	}
	for _, msg := range m.messages[start:] {
		if msg.Role == "assistant" && !msg.IsThinking {
			// The stream carries the call's tags; the round's text does
			// not. Tags are not words.
			segments = append(segments, msg.Content)
		}
	}
	want := words(final)
	if len(want) == 0 {
		return "", false
	}
	// A ROUND'S OWN TEXT ARRIVES THE SAME WAY. The gateway sends each
	// round's text after its inference, and by then that round's tool frame
	// sits under the streamed prose — so what arrives is the LAST round(s),
	// not the whole turn. Compared only against everything streamed, no
	// round after the first ever matched: each printed again under its
	// frame, and the next round's stream glued onto the copy — every
	// paragraph of the turn twice (the drone reel, 2026-09-19). The longest
	// match wins: the whole turn first, then the last rounds alone.
	for j := 0; j < len(segments); j++ {
		got := words(strings.Join(segments[j:], "\n"))
		if len(got) == 0 || len(got) > len(want) {
			continue
		}
		matched := true
		for i := range got {
			if got[i] != want[i] {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		if len(got) == len(want) {
			return "", true
		}
		// The tail is what did not stream: the final text after its first
		// len(got) words, with the reply's own line breaks kept.
		rest := final
		for n := 0; n < len(got); n++ {
			rest = strings.TrimLeft(rest, " \t\r\n")
			if k := strings.IndexAny(rest, " \t\r\n"); k >= 0 {
				rest = rest[k:]
			} else {
				rest = ""
			}
		}
		return strings.TrimSpace(rest), true
	}
	return "", false
}

// wasInterrupted reads a turn's error text for the marks of a cancelled
// request rather than a broken one. Everything a real failure says — a status
// code, a refusal, a parse error, a missing key — still reaches the person
// unchanged; only the cancellation the person caused is renamed.
func wasInterrupted(errText string) bool {
	e := strings.ToLower(errText)
	for _, mark := range []string{"context canceled", "context cancelled", "request canceled", "operation was canceled", "interrupted"} {
		if strings.Contains(e, mark) {
			return true
		}
	}
	return false
}

// nextEffort is the level after cur in auto → low → medium → high → auto.
func nextEffort(cur string) string {
	switch cur {
	case "":
		return "low"
	case "low":
		return "medium"
	case "medium":
		return "high"
	}
	return ""
}
