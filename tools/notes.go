package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memdoor/pkg/notes"
)

// notes — an agent's working memory for the conversation (pkg/notes).
//
// Every long run that died on Sara's recording (2026-09-12/13) had already
// worked out the two intros, the five sections with their line numbers,
// the translations — and every retry and every new turn started from
// zero, because that work lived only in a reply the guard cut. A to-do
// list is a checklist, not a place for findings. So: a notes file in the
// project folder the model APPENDS to as it works and READS first when it
// exists. The harness also saves a guard-cut analysis into it, so nothing
// the model figured out is lost (AppendNotes below).
//
// One set per conversation (pkg/notes): per folder, a rule noted for one
// task steered every later one (2026-09-29).

const notesReadMax = 48 << 10

// notesRepo is where every conversation's notes are kept; wired at startup.
var notesRepo notes.Repository

// SetNotesRepository wires the store the notes tool reads and writes.
func SetNotesRepository(r notes.Repository) { notesRepo = r }

// ruleBook writes kept rules into the project's instructions file.
var ruleBook = notes.NewMarkdownRuleBook()

// DeleteNotes forgets a conversation's notes (/fresh, /clear).
func DeleteNotes(key notes.ConversationKey) error {
	if notesRepo == nil {
		return nil
	}
	return notesRepo.Delete(key)
}

func loadNotes(key notes.ConversationKey) string {
	if notesRepo == nil {
		return ""
	}
	s, _ := notesRepo.Load(key)
	return s
}

type NotesInput struct {
	Append string `json:"append,omitempty" jsonschema_description:"Text to add to the notes (a section map with line numbers, a translation, a decision). Markdown; keep each entry short and factual."`
	Read   bool   `json:"read,omitempty" jsonschema_description:"Return the notes that matter for this turn's task (all of them when short)."`
	All    bool   `json:"all,omitempty" jsonschema_description:"With read: return the last 48 KB unfiltered instead."`
	Rule   string `json:"rule,omitempty" jsonschema_description:"A project rule to keep for every LATER conversation (how to run the tests, what never to do). It is written into the project's AGENTS.md, read at every turn."`
	// Conversation and RulesFile are set by the harness, never by the model:
	// whose notes, and which instructions file a rule goes into.
	Conversation string `json:"conversation,omitempty" jsonschema:"-"`
	RulesFile    string `json:"rules_file,omitempty" jsonschema:"-"`
}

var notesSchema = GenerateSchema[NotesInput]()

var NotesDefinition = ToolDefinition{
	Name: "notes",
	Description: "Working memory for THIS conversation only — gone when it ends: append what you work out " +
		"as you go (the section map with transcript line numbers, which take is the good one, decisions) and " +
		"read it back after a cut reply or when earlier turns were compacted. " +
		"A rule the person wants kept for LATER conversations (\"always run tests with -race\", \"never add " +
		"dependencies\") goes in {\"rule\": \"…\"}: it is written into the project's AGENTS.md, read at every turn. " +
		"Use {\"append\": \"…\"} to add, {\"read\": true} to read, {\"rule\": \"…\"} to keep a rule.",
	InputSchema: notesSchema,
	Function:    Notes,
}

// AppendNotes adds one dated entry under a heading. Best effort: a notes
// write must never fail a turn.
func AppendNotes(key notes.ConversationKey, heading, text string) error {
	if key == "" || notesRepo == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	return appendNotesOnce(key, heading, text, false)
}

// errAlreadyNoted is the answer to an entry the notes already hold.
type errAlreadyNoted struct{ when string }

func (e errAlreadyNoted) Error() string {
	return "already noted (entry of " + e.when + ") — nothing new to add; act on what you know, do not note it again"
}

// appendNotesOnce writes the entry unless the notes already say it.
//
// THE SAME RULE FORTY TIMES IS A LOOP, NOT A MEMORY. Live 2026-09-18: the
// model appended "RULE: better quality = re-encode at high bitrate" with
// small rewordings forty-one times in one turn, 10 KB of notes that every
// later turn reads back. The byte count in the reply changed each time,
// so the repeat guard (keyed on the result) never saw a repeat. An entry
// whose words are nearly all in an entry already there is refused, and the
// refusal repeats the same reason — which is what retires the tool.
func appendNotesOnce(key notes.ConversationKey, heading, text string, force bool) error {
	text = strings.TrimSpace(text)
	if !force {
		if when, dup := notesAlreadySay(key, text); dup {
			return errAlreadyNoted{when: when}
		}
	}
	return notesRepo.Append(key, fmt.Sprintf("\n## %s — %s\n\n%s\n", heading, time.Now().Format("2006-01-02 15:04"), text))
}

// notesAlreadySay reports whether one of the last notesDedupWindow entries
// says what text says (the same words, near enough), and when it was noted.
func notesAlreadySay(key notes.ConversationKey, text string) (string, bool) {
	b := loadNotes(key)
	if b == "" {
		return "", false
	}
	entries := strings.Split(b, "\n## ")
	if len(entries) > notesDedupWindow {
		entries = entries[len(entries)-notesDedupWindow:]
	}
	want := wordSet(text)
	if len(want) < notesDedupMinWords {
		return "", false
	}
	for _, e := range entries {
		head, body, _ := strings.Cut(e, "\n")
		if wordOverlap(want, wordSet(body)) >= notesDedupOverlap {
			when := head
			if i := strings.Index(head, " — "); i >= 0 {
				when = head[i+len(" — "):]
			}
			return strings.TrimSpace(when), true
		}
	}
	return "", false
}

const (
	notesDedupWindow   = 40
	notesDedupMinWords = 6
	notesDedupOverlap  = 0.8
)

// wordSet is the distinct lower-case words of a text, punctuation dropped.
func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x80)
	}) {
		out[w] = true
	}
	return out
}

// wordOverlap is how much of `a` is in `b`: 1 when every word of a appears
// in b.
func wordOverlap(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 0
	}
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return float64(n) / float64(len(a))
}

// NotesHaveEntry reports whether the conversation's notes hold an entry
// under heading (AppendNotes writes "## heading — date").
func NotesHaveEntry(key notes.ConversationKey, heading string) bool {
	return strings.Contains(loadNotes(key), "\n## "+heading+" — ")
}

// ReadNotes returns the notes' tail (at most notesReadMax bytes) and their
// size, or "" when there are none.
func ReadNotes(key notes.ConversationKey) (string, int) {
	s := loadNotes(key)
	size := len(s)
	if size > notesReadMax {
		s = "…(earlier notes omitted)\n" + s[size-notesReadMax:]
	}
	return s, size
}

func Notes(input json.RawMessage) (string, error) {
	var in NotesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	if strings.TrimSpace(in.Append) == "" && !in.Read && strings.TrimSpace(in.Rule) == "" {
		return "", fmt.Errorf("notes: give append (text to add), rule (to keep for later) or read: true")
	}
	if strings.TrimSpace(in.Rule) != "" {
		if err := ruleBook.Add(in.RulesFile, in.Rule); err != nil {
			return "", fmt.Errorf("notes: the rule was not kept: %w", err)
		}
		if strings.TrimSpace(in.Append) == "" && !in.Read {
			return fmt.Sprintf("kept in %s — read at every turn from now on, in this conversation and every later one", in.RulesFile), nil
		}
	}
	if notesRepo == nil {
		return "", fmt.Errorf("notes: no notes store is set up in this gateway")
	}
	key, err := notes.ParseConversationKey(in.Conversation)
	if err != nil {
		return "", fmt.Errorf("notes: %w", err)
	}
	if strings.TrimSpace(in.Append) != "" {
		if err := AppendNotes(key, "note", in.Append); err != nil {
			return "", fmt.Errorf("notes: %w", err)
		}
		if !in.Read {
			_, n := ReadNotes(key)
			return fmt.Sprintf("noted (%d bytes of notes in this conversation)", n), nil
		}
	}
	s, n := ReadNotes(key)
	if n == 0 {
		return "no notes yet in this conversation — append what you work out as you go", nil
	}
	if !in.All && n > notesJudgeMin {
		if judged, ok := judgedNotes(key, n, injectedTurnTask(input)); ok {
			return judged, nil
		}
	}
	return fmt.Sprintf("notes (%d bytes):\n%s", n, s), nil
}

// Judged read. An agent reads its notes at the start of turns, and one
// folder's notes grow across every job done in it: 84 KB on 2026-09-26, read
// 27 times in one evening for 641 KB of tool output, each copy resent on
// every later call. When the turn
// has a brief and a decision model answers, each section is judged against
// the brief and only the ones that help come back, with the latest few
// always kept. Measured on that file with the sourdough edit's brief: 17 of
// 132 sections kept (12 KB of 83 KB), 16 of the 18 about that job among
// them; the two missed were older duplicates of kept delivery notes.
const (
	notesJudgeMin    = 12 << 10 // below this the whole tail is returned
	notesKeepRecent  = 3        // the latest sections are always kept
	notesJudgeClip   = 1500     // characters of each section the judge sees
	notesJudgeBudget = 20 * time.Second
)

// judgedNotes returns the sections of the notes that help with the turn's
// brief, or false to fall back to the unjudged tail.
func judgedNotes(key notes.ConversationKey, size int, task string) (string, bool) {
	brief := strings.TrimSpace(task)
	svc := decisionService()
	if brief == "" || svc == nil {
		return "", false
	}
	secs := splitNoteSections(loadNotes(key))
	if len(secs) <= notesKeepRecent {
		return "", false
	}
	items := make([]string, len(secs))
	for i, sec := range secs {
		if r := []rune(sec); len(r) > notesJudgeClip {
			sec = string(r[:notesJudgeClip]) + "…"
		}
		items[i] = sec
	}
	ctx, cancel := context.WithTimeout(context.Background(), notesJudgeBudget)
	defer cancel()
	ps, err := judgeRelevance(ctx, svc, relevanceSpec{
		Task:     brief,
		Framing:  "Each question shows one section of the agent's own notes from earlier in this conversation. Judge whether re-reading it helps with the task.",
		Question: "Does this note help with the task?",
		True:     "It is about this job, its source, or a rule or finding that applies to it.",
		False:    "It is about another job or is no longer useful.",
		Purpose:  "notes-read",
	}, items)
	if err != nil {
		return "", false
	}
	var kept []string
	for i, sec := range secs {
		if ps[i] >= jevKeep || i >= len(secs)-notesKeepRecent {
			kept = append(kept, sec)
		}
	}
	out := strings.Join(kept, "\n\n")
	if len(out) > notesReadMax {
		out = "…(earlier kept notes omitted)\n" + out[len(out)-notesReadMax:]
	}
	return fmt.Sprintf("notes (%d bytes) — the %d of %d sections that help with this turn's task, the latest %d always included. "+
		"{\"read\": true, \"all\": true} returns the last 48 KB unfiltered.\n%s",
		size, len(kept), len(secs), notesKeepRecent, out), true
}

// splitNoteSections splits the notes at their "## " headings, keeping each
// heading with its body. Text before the first heading is its own section.
func splitNoteSections(s string) []string {
	var secs []string
	var cur strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		if strings.HasPrefix(line, "## ") && strings.TrimSpace(cur.String()) != "" {
			secs = append(secs, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
		cur.WriteString(line)
	}
	if strings.TrimSpace(cur.String()) != "" {
		secs = append(secs, strings.TrimSpace(cur.String()))
	}
	return secs
}
