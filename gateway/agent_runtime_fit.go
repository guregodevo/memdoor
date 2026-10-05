package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"memdoor/gateway/compaction"
	ctxmgmt "memdoor/gateway/context"
	"memdoor/gateway/infra"
	"memdoor/gateway/logs"
	"memdoor/gateway/memory"
	"memdoor/gateway/providers"
	"memdoor/pkg/llm"
	sharedctx "memdoor/pkg/shared/context"
	"memdoor/tools"
)

// FITTING A CONVERSATION TO THE WINDOW: ONE LADDER.
//
// When a conversation must shrink (compactionDecision says why), it is
// reshaped one step at a time until it fits, giving up the most replaceable
// thing first: old tool output (it can be read again from disk), then old
// messages (a summary keeps what was asked and answered and the receipts of
// the work), and only last the tool output the turn is working from.
//
// There were three ladders — a threshold one, an "emergency" one and one
// after a provider's refusal — with different steps and different
// bookkeeping, and the bookkeeping is where the bugs were: a turn in which
// the conversation was compacted never saved the person's request
// (2026-09-29, three requests missing from one transcript), and the reshaped
// conversation was recomputed every turn, so no prompt prefix stayed cached.

// fitStep is one rung of the ladder.
type fitStep struct {
	kind fitKind
	// keep: the last keep messages are left as they are.
	keep int
	// share (summaries): keep the recent messages that fit in this share of
	// the size that triggers compaction, instead of a count.
	share float64
}

type fitKind int

const (
	fitElide     fitKind = iota // stub superseded and stale tool output (compaction.ElideSpent)
	fitStub                     // stub tool output older than the last keep messages
	fitSummarize                // replace older messages by a summary
)

func (s fitStep) String() string {
	switch {
	case s.kind == fitElide:
		return "stubbed superseded and stale tool output"
	case s.kind == fitStub && s.keep == 0:
		return "stubbed all tool output"
	case s.kind == fitStub:
		return fmt.Sprintf("stubbed tool output older than the last %d messages", s.keep)
	case s.share > 0:
		return fmt.Sprintf("summarized older messages, keeping the last %.0f%% of the threshold", s.share*100)
	}
	return fmt.Sprintf("summarized all but the last %d messages", s.keep)
}

// compactTarget is where a compaction aims, as a share of the size that
// triggers it. Stopping at the first rung under the threshold left no room:
// the next answer crossed it again, and three turns in a row compacted,
// each with a new prefix and nothing cached (live 2026-09-29, DeepSeek:
// 0-37% cache on those turns, 89-91% on the turns between).
const compactTarget = 0.7

var (
	// fitLadder is the ladder on a remote model. Recent messages are kept
	// by tokens: kept by count (15, then 8), long answers left a summary
	// that saved one message in seventeen (2026-09-29).
	fitLadder = []fitStep{{kind: fitElide}, {kind: fitStub, keep: doerCompactionKeepRecent}, {kind: fitStub, keep: 2},
		{kind: fitSummarize, share: 0.5}, {kind: fitSummarize, share: 0.25}, {kind: fitStub}}
	// refusalLadder is after a provider refused the request for its size,
	// mid-turn: only tool output is given up, the newest last.
	refusalLadder = []fitStep{{kind: fitElide}, {kind: fitStub, keep: 2}, {kind: fitStub}}
)

// fit applies steps until fits says the conversation does, and returns it
// with the steps that changed it. Stubbing builds on stubbing; a summary is
// made from the conversation as stubbed, not from an earlier summary.
// beforeDrop, when set, is called once with that conversation before the
// first summary replaces messages. msgs itself is not changed.
func fit(msgs []llm.MessageParam, steps []fitStep, summarize func([]llm.MessageParam, fitStep) []llm.MessageParam,
	fits func([]llm.MessageParam) bool, beforeDrop func([]llm.MessageParam)) ([]llm.MessageParam, []fitStep) {
	cur, base := msgs, msgs
	summarized := false
	var applied []fitStep
	for _, step := range steps {
		if fits(cur) {
			break
		}
		if step.kind != fitSummarize {
			var next []llm.MessageParam
			var changed bool
			if step.kind == fitElide {
				next, changed = compaction.ElideSpent(cur)
			} else {
				next, changed = compaction.ShedToolResults(cur, step.keep)
			}
			if !changed {
				continue
			}
			cur = next
			if !summarized {
				base = cur
			}
			applied = append(applied, step)
			continue
		}
		next := summarize(base, step)
		if len(next) >= len(base) {
			continue
		}
		if !summarized && beforeDrop != nil {
			beforeDrop(base)
		}
		summarized = true
		cur = next
		applied = append(applied, step)
	}
	return cur, applied
}

// fitRequest is what runInference knows when a conversation must shrink.
type fitRequest struct {
	runID, sessionID string
	why              compaction.CompactionType
	steps            []fitStep
	// check measures a conversation against the answering model's window.
	check func([]llm.MessageParam) *ctxmgmt.CheckResult
	// flush saves notes before messages are dropped; nil to skip.
	flush func([]llm.MessageParam)
}

// fitConversation reshapes conversation to fit, saves it as sent, and tells
// the screen once. It fails only when nothing is left to give up.
func (ar *AgentRuntime) fitConversation(ctx context.Context, conversation []llm.MessageParam, req fitRequest) ([]llm.MessageParam, *ctxmgmt.CheckResult, error) {
	log := logs.New("Agent").WithSession(req.sessionID).WithRun(req.runID)
	before := ar.compactor.CountConversationTokens(conversation)
	ar.emitCompaction(req, "compaction_start", before, 0)

	var last *ctxmgmt.CheckResult
	fits := func(c []llm.MessageParam) bool {
		last = req.check(c)
		switch {
		case last == nil:
			return true
		case req.why == compaction.CompactionOverLimit:
			return last.CanProceed
		}
		return last.CanProceed && float64(last.TotalTokens) <= compactTarget*float64(last.CompactAt)
	}
	flush := req.flush
	if flush != nil {
		flush = func(c []llm.MessageParam) {
			if chk := req.check(c); chk == nil || chk.CanProceed { // else it cannot be sent
				req.flush(c)
			}
		}
	}
	// A share is of the size that triggers compaction; over the limit,
	// of the whole effective limit.
	budgetOf := func(chk *ctxmgmt.CheckResult) int {
		if chk == nil {
			return 0
		}
		if req.why == compaction.CompactionOverLimit || chk.CompactAt == 0 {
			return chk.EffectiveLimit
		}
		return chk.CompactAt
	}
	budget := budgetOf(req.check(conversation))
	fitted, applied := fit(conversation, req.steps, func(c []llm.MessageParam, step fitStep) []llm.MessageParam {
		keep := step.keep
		if step.share > 0 {
			keep = ar.compactor.KeepForTokens(c, int(step.share*float64(budget)), 2)
		}
		_, out := ar.compactor.Compact(c, keep)
		return out
	}, fits, flush)
	last = req.check(fitted)

	if len(applied) > 0 {
		ar.saveAsSent(ctx, conversation, fitted)
		after := ar.compactor.CountConversationTokens(fitted)
		names := make([]string, len(applied))
		for i, s := range applied {
			names[i] = s.String()
		}
		log.Info(fmt.Sprintf("Compaction: %s — %d → %d tokens in %d → %d messages",
			strings.Join(names, ", then "), before, after, len(conversation), len(fitted)))
		ar.emitCompaction(req, "compaction_complete", before, after)
	} else {
		ar.emitCompaction(req, "compaction_complete", before, before)
	}

	if last != nil && !last.CanProceed {
		reason := last.Error
		if reason == "" {
			reason = last.Warning
		}
		// Nothing left to give up means the BASELINE does not fit — system
		// prompt plus tool definitions plus a few stubbed messages — which
		// no amount of trimming can fix. Say that, rather than blaming the
		// conversation.
		return fitted, last, fmt.Errorf("the window cannot hold this turn even with every tool output shed (%s) — the system prompt and tool definitions alone are too large for the serving model", reason)
	}
	return fitted, last, nil
}

func (ar *AgentRuntime) emitCompaction(req fitRequest, event string, before, after int) {
	if ar.events == nil || req.runID == "" {
		return
	}
	ar.events.EmitEvent(req.runID, infra.EventStreamLifecycle, req.sessionID, map[string]interface{}{
		"event":         event,
		"before_tokens": before,
		"after_tokens":  after,
	})
}

// saveAsSent records a reshaped conversation so the next turn sends the same
// prefix (MessageRecord): first the messages of before not yet in the
// transcript — the person's request among them, which a compaction used to
// lose — then after, as a block the next turn loads from. The cursor then
// counts after: what the turn adds from here is saved at its end.
func (ar *AgentRuntime) saveAsSent(ctx context.Context, before, after []llm.MessageParam) {
	session, _ := ctx.Value(ctxSession).(*Session)
	if session == nil {
		return
	}
	if ar.persistence != nil {
		key := transcriptKeyOf(session)
		if unsaved := before[sessionCursor(session, len(before)):]; len(unsaved) > 0 {
			if err := ar.persistence.SaveMessages(key, unsaved); err != nil {
				logs.New("Agent").WithSession(session.ID).WithError(err).Error("Failed to save messages before a reshape")
			}
		}
		if err := ar.persistence.SaveConversation(key, after); err != nil {
			logs.New("Agent").WithSession(session.ID).WithError(err).Error("Failed to save the reshaped conversation")
		}
	}
	session.SetMetadata("conversation_length", len(after))
}

// sessionCursor is how many of a conversation's n messages are in the
// transcript already, never outside 0..n.
func sessionCursor(session *Session, n int) int {
	raw, _ := session.GetMetadataValue("conversation_length")
	cursor, _ := raw.(int)
	return max(0, min(cursor, n))
}

// transcriptKeyOf is the key a session's turns are saved under: its agent's
// own transcript on a channel (buildConversation stashes the agent).
func transcriptKeyOf(session *Session) string {
	if an, _ := session.GetMetadataValue("transcript_agent"); an != nil {
		if name, ok := an.(string); ok {
			return transcriptKey(session.ID, name)
		}
	}
	return session.ID
}

// capToolResult cuts a tool result to the transcript's limit as it enters
// the conversation, so the model is sent what the transcript keeps: cut only
// when saved, the next turn sent a different text than this one had, and no
// prefix stayed cached.
func (ar *AgentRuntime) capToolResult(block llm.ContentBlockParamUnion) llm.ContentBlockParamUnion {
	if ar.persistence == nil || block.OfToolResult == nil {
		return block
	}
	return ar.persistence.truncateLargeToolResults(llm.NewUserMessage(block)).Content[0]
}

// flushNotesHeading heads the notes entry the pre-compaction flush writes.
const flushNotesHeading = "Before compaction"

// flushNotes asks the model, once per conversation, what is worth keeping
// before a compaction replaces messages by a summary, and keeps the answer
// in the conversation's notes. The entry is the record that it ran: a
// turn's session metadata does not outlive the turn, and /fresh and /clear
// delete the notes with the conversation.
func (ar *AgentRuntime) flushNotes(ctx context.Context, conversation []llm.MessageParam) {
	settings := memory.ResolveFlushSettings(ar.compactionConfig)
	key := conversationOf(ctx)
	if settings == nil || !settings.Enabled || key == "" || tools.NotesHaveEntry(key, flushNotesHeading) {
		return
	}
	log := logs.New("Agent").WithSession(key.String())

	flushConv := append(append([]llm.MessageParam{}, conversation...), llm.NewUserMessage(llm.NewTextBlock(settings.Prompt)))
	// A utility call, and one that does not compact: it is the compaction.
	flushCtx, cancel := context.WithTimeout(llm.WithUtilityCall(cheapestRung(ctx)), 30*time.Second)
	defer cancel()
	flushCtx = context.WithValue(flushCtx, ctxSkipCompaction, true)

	result, _, err := ar.runInference(flushCtx, flushConv, "")
	if err != nil || result == nil {
		if err != nil {
			log.WithError(err).Warn("Memory flush failed")
		}
		return
	}
	kept := flushReplyText(result)
	if kept == "" {
		kept = "Nothing new worth keeping at this point."
	}
	if err := tools.AppendNotes(key, flushNotesHeading, kept); err != nil {
		log.Debug("Memory flush: notes not written", slog.String("error", err.Error()))
		return
	}
	log.Info("Memory flush kept notes before compaction", slog.Int("bytes", len(kept)))
}

// manualKeepTokens is how much recent conversation /compact keeps verbatim
// (omp's keepRecentTokens is 20000; a manual compaction is asked for when
// the conversation is too long, so it keeps less).
const manualKeepTokens = 12_000

// keepRecentTokens is how much /compact keeps word for word:
// compaction.keepRecentTokens, else manualKeepTokens.
func (ar *AgentRuntime) keepRecentTokens() int {
	if c := ar.compactionConfig; c != nil && c.KeepRecentTokens > 0 {
		return c.KeepRecentTokens
	}
	return manualKeepTokens
}

// CompactResult is what /compact did.
type CompactResult struct {
	BeforeTokens   int `json:"before_tokens"`
	AfterTokens    int `json:"after_tokens"`
	BeforeMessages int `json:"before_messages"`
	AfterMessages  int `json:"after_messages"`
	// Summarized: older messages were replaced by a summary (the focus
	// leads it). False when stubbing tool output was all it took, and all
	// the rest fits in manualKeepTokens.
	Summarized bool `json:"summarized"`
	// Written: the model wrote that summary; false when it could not be
	// asked and the digest stands in.
	Written bool `json:"written"`
}

// CompactNow is /compact: stub superseded and stale tool output, then
// replace all but the last manualKeepTokens of the conversation as last
// sent by a summary the answering model writes — the focus (what to keep in
// mind) at its head — and save it as what the next turn sends. The one
// model call of compaction, asked for by the person; when it fails the
// digest stands in. The caller makes sure no turn is running on it.
func (ar *AgentRuntime) CompactNow(ctx context.Context, transcript, agent, focus string) (CompactResult, error) {
	if ar.persistence == nil {
		return CompactResult{}, fmt.Errorf("no transcript store")
	}
	conv, err := ar.persistence.LoadRecentMessages(transcript, wholeTranscript)
	if err != nil {
		return CompactResult{}, err
	}
	res := CompactResult{BeforeTokens: ar.compactor.CountConversationTokens(conv), BeforeMessages: len(conv)}
	out, _ := compaction.ElideSpent(conv)
	keep := ar.compactor.KeepForTokens(out, ar.keepRecentTokens(), 2)
	focus = strings.TrimSpace(focus)
	if summary, compacted := ar.compactor.Compact(out, keep); summary.MessagesRemoved > 0 {
		res.Summarized = true
		instructions := summaryInstructions
		if focus != "" {
			instructions += "\n\nThe person asked you to focus on this: " + focus
		}
		if written, err := ar.writeSummary(ctx, agent, out[:summary.MessagesRemoved], instructions); err == nil {
			_, compacted = ar.compactor.CompactWritten(out, keep, written)
			res.Written = true
		} else {
			logs.New("Agent").WithError(err).Warn("/compact: the summary could not be written; the digest stands in")
		}
		out = compacted
		if focus != "" {
			head := "FOCUS (asked for with /compact — keep this in mind from here on): " + focus + "\n\n"
			out[0] = llm.NewUserMessage(llm.NewTextBlock(head + out[0].Content[0].OfText.Text))
		}
	}
	res.AfterTokens, res.AfterMessages = ar.compactor.CountConversationTokens(out), len(out)
	if float64(res.AfterTokens) > 0.9*float64(res.BeforeTokens) {
		// Next to nothing to give up: a new block would only change the
		// prefix the provider has cached.
		return CompactResult{BeforeTokens: res.BeforeTokens, AfterTokens: res.BeforeTokens,
			BeforeMessages: res.BeforeMessages, AfterMessages: res.BeforeMessages}, nil
	}
	return res, ar.persistence.SaveConversation(transcript, out)
}

// cheapestRung is ctx for a call made FOR the conversation, not in it (a
// summary, a handoff, the notes flush): the first rung of the agent's
// ladder, the cheapest, whatever model the conversation is pinned to or
// has escalated to. A pinned expensive model wrote summaries at its own
// price (2026-09-29).
func cheapestRung(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, sharedctx.ModelKey, "")
	return context.WithValue(ctx, sharedctx.TierKey, 0)
}

// summaryInstructions is what /compact asks the model for.
const summaryInstructions = `Summarize the conversation above so the work can continue without it: what the person asked for; decisions made and why; what was done (files changed, commands run, their results); what failed and why; what is still open. Keep exact file names, paths, commands, numbers and errors. Plain text, short sections, no preamble.`

// handoffInstructions is what /handoff asks the model for.
const handoffInstructions = `Write a handoff for a fresh session that will continue this work without seeing the conversation above. Sections, in order: Goal. Decisions (and why). Progress (what is done and verified — files, commands, results). Open problems. Next steps (numbered, concrete). Keep exact file names, paths, commands and numbers. No preamble.`

// writeSummary asks the answering model to summarize msgs: one utility call,
// no tools. Tool output is clipped: the summary is of the work, not of what
// a tool printed.
func (ar *AgentRuntime) writeSummary(ctx context.Context, agent string, msgs []llm.MessageParam, instructions string) (string, error) {
	if ar.clientFactory == nil {
		return "", fmt.Errorf("no model client")
	}
	ctx = cheapestRung(ctx)
	client, err := ar.clientFactory.GetClientFor(ctx, agent)
	if err != nil {
		return "", err
	}
	// The model's own reply cap: a model that reasons first spent a fixed
	// 2048 tokens reasoning and wrote nothing (live 2026-09-29, GLM 5.3
	// Flash: finish=length, no text).
	model, _ := ar.answeringModel(context.WithValue(ctx, "buddy_agent_name", agent))
	ctx, cancel := context.WithTimeout(llm.WithUtilityCall(ctx), 3*time.Minute)
	defer cancel()
	resp, err := client.Messages().New(ctx, llm.MessageNewParams{
		Model:     llm.Model(providers.ModelName),
		System:    []llm.TextBlockParam{{Type: "text", Text: "You write summaries of a coding conversation so that the work can continue without it."}},
		Messages:  []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock(conversationText(msgs) + "\n\n---\n\n" + instructions))},
		MaxTokens: maxOutputTokens(model),
		Agent:     agent,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range resp.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		if resp.StopReason == llm.StopReasonMaxTokens {
			return "", fmt.Errorf("the model used its whole reply on reasoning and wrote no summary")
		}
		return "", fmt.Errorf("the model wrote no summary")
	}
	return text, nil
}

// conversationText renders a conversation for a summary: every text, each
// tool call on one line, each tool result clipped.
func conversationText(msgs []llm.MessageParam) string {
	const resultChars = 1500
	var b strings.Builder
	for _, m := range msgs {
		for _, c := range m.Content {
			switch {
			case c.OfText != nil && m.Role == llm.MessageParamRoleUser:
				fmt.Fprintf(&b, "PERSON: %s\n\n", c.OfText.Text)
			case c.OfText != nil:
				fmt.Fprintf(&b, "AGENT: %s\n\n", c.OfText.Text)
			case c.OfToolUse != nil:
				fmt.Fprintf(&b, "AGENT CALLED %s(%s)\n", c.OfToolUse.Name, truncateForLog(string(c.OfToolUse.Input), 300))
			case c.OfToolResult != nil:
				for _, r := range c.OfToolResult.Content {
					if r.OfText != nil {
						fmt.Fprintf(&b, "RESULT: %s\n\n", truncateForLog(r.OfText.Text, resultChars))
					}
				}
			}
		}
	}
	return b.String()
}

// Handoff is /handoff: the answering model writes a handoff of the
// conversation as last sent (goal, decisions, progress, open problems, next
// steps). The caller wipes the conversation and starts the next from it.
func (ar *AgentRuntime) Handoff(ctx context.Context, transcript, agent string) (string, error) {
	if ar.persistence == nil {
		return "", fmt.Errorf("no transcript store")
	}
	conv, err := ar.persistence.LoadRecentMessages(transcript, wholeTranscript)
	if err != nil {
		return "", err
	}
	if len(conv) == 0 {
		return "", fmt.Errorf("this conversation has nothing to hand off yet")
	}
	conv, _ = compaction.ElideSpent(conv)
	return ar.writeSummary(ctx, agent, conv, handoffInstructions)
}

// handoffHead opens the first message of a session started by /handoff.
const handoffHead = "HANDOFF FROM THE PREVIOUS SESSION (written by the model at /handoff — the conversation it summarizes is gone; build on this):\n\n"

// StartFrom starts a wiped conversation with text as its first message
// (/handoff).
func (ar *AgentRuntime) StartFrom(transcript, text string) error {
	if ar.persistence == nil {
		return fmt.Errorf("no transcript store")
	}
	return ar.persistence.SaveMessages(transcript, []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock(text))})
}

// recallOutput finds the output of call id in a transcript's history: what
// the recall tool returns for a stub.
func (ar *AgentRuntime) recallOutput(transcript, id string) (string, error) {
	if ar.persistence == nil {
		return "", fmt.Errorf("recall: no transcript store")
	}
	history, err := ar.persistence.LoadMessages(transcript)
	if err != nil {
		return "", err
	}
	for i := len(history) - 1; i >= 0; i-- {
		for _, b := range history[i].Content {
			if b.OfToolResult == nil || b.OfToolResult.ToolUseID != id {
				continue
			}
			var out strings.Builder
			for _, c := range b.OfToolResult.Content {
				if c.OfText != nil {
					out.WriteString(c.OfText.Text)
				}
			}
			return out.String(), nil
		}
	}
	return "", fmt.Errorf("recall: no call %q in this conversation's history (a call of the current turn is still in the conversation as it was)", id)
}
