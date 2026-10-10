package cmd

import (
	"fmt"
	"strings"
	"time"

	"memdoor/gateway/providers"
)

// WARN AT THE MOMENT OF PINNING (roadmap SHOULD.md, "Work on the OpenRouter
// leaderboard's top models": "warn at the moment of pinning … so `/model` warns
// before the turn rather than failing during it").
//
// `memdoor model check` already asks a model for what a coder turn needs — a
// native tool call, arguments our schema accepts, an answer after a tool result,
// the right tool of several, an apply_patch hunk our parser parses — and keeps
// the newest report per model in ~/.memdoor/model-checks.json. Someone arriving
// from the OpenRouter ranking pins whatever they already pay for, and the report
// for that model is often already sitting there. Reading it costs nothing and
// turns a turn that dies halfway into a sentence beforehand.
//
// A WARNING, NEVER A REFUSAL. The probe is a few hundred tokens of evidence
// about one moment: a vendor ships a fix the next day, the adapter
// (gateway/providers/model_adapter.go) repairs some of these on the fly, and the
// text-tag fallback covers a patch a model cannot emit as a call. So the pin
// always goes through, and silence is the answer whenever there is nothing
// solid to say — no report, a probe that passed, or a gateway that will not
// answer.

// freeModelNotice is the line after a pin of a `:free` model: the one case
// where a request leaves with data_collection allow.
const freeModelNotice = "⚠ a free model: its hosts train on what they are sent and serve it as they choose (below fp8 too); Memdoor sends it with data_collection allow and no quantization floor. Fine for a trial, not for private code."

// pinWarning is the one line to add after a pin, or "" when there is nothing to
// warn about.
func pinWarning(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	// A free model's hosts train on what they are sent; the request says
	// allow for it (gateway/providers/byok.go). Said once, at the pin, after
	// whatever the probe found.
	free := ""
	if providers.IsFreeModel(id) {
		free = freeModelNotice
	}
	var res struct {
		Checks map[string]modelCheckView `json:"checks"`
	}
	if err := NewClient().GetJSON("/api/models/check", &res); err != nil {
		return free // never probed here, or no gateway to ask: say nothing more
	}
	c, ok := res.Checks[id]
	if !ok {
		for k, v := range res.Checks {
			if strings.EqualFold(k, id) {
				c, ok = v, true
				break
			}
		}
	}
	if !ok {
		return free
	}
	trouble := pinTrouble(c)
	if trouble == "" {
		return free
	}
	return strings.TrimSpace(fmt.Sprintf("⚠ %s %s. `memdoor model check %s` probes it again. %s", trouble, probedWhen(c.At), id, free))
}

// pinTrouble is what the report says in the person's terms, worst first and one
// thing only: a model that never called a tool also failed everything after it,
// and a list of five would bury the one fact that matters.
func pinTrouble(c modelCheckView) string {
	switch {
	case !c.ToolCall:
		// "Did not call a tool" would be a lie about a model the probe never
		// reached — a dead id, no endpoints, a data policy that excludes every
		// host. Those three were the only failures in 32 reports on
		// 2026-09-27, and each is fixed by something different, so the host's
		// own words go through.
		if why, ok := upstreamReason(c.Note); ok {
			return "the probe never got an answer out of this model: " + why
		}
		return "this model did not call a tool at all when probed, so a coder turn cannot run on it"
	case !c.ValidArgs:
		return "this model called a tool with arguments the schema rejects, so a coder turn may stall on it"
	case !c.RoundTrip:
		return "this model stopped instead of carrying on after a tool result, so a turn may end early on it"
	case !c.RightTool:
		return "this model picked the wrong tool of several when probed, so it may read the wrong thing first"
	case !c.Patch:
		return "this model wrote no patch our parser accepts, so edits fall back to the text-tag path"
	case c.Followed == "no":
		return "this model skipped an instruction to load the workflow skill first when probed, so a workflow ask may run inline on it"
	}
	return ""
}

// probedWhen dates the report, because a model is a moving target and a person
// deciding whether to trust this needs to know how old it is.
func probedWhen(at time.Time) string {
	if at.IsZero() {
		return "(probed at an unknown time)"
	}
	switch d := time.Since(at); {
	case d < time.Hour:
		return "(probed just now)"
	case d < 24*time.Hour:
		return fmt.Sprintf("(probed %dh ago)", int(d.Hours()))
	case d < 48*time.Hour:
		return "(probed yesterday)"
	default:
		return fmt.Sprintf("(probed %d days ago)", int(d.Hours()/24))
	}
}

// upstreamReason is what the host said when the probe's first call never landed,
// or false when the model did answer and the trouble is the answer itself.
func upstreamReason(note string) (string, bool) {
	if !strings.HasPrefix(strings.TrimSpace(note), "first call failed") {
		return "", false
	}
	msg, cut := note, false
	if i := strings.Index(note, `"message":"`); i >= 0 {
		msg = note[i+len(`"message":"`):]
		if j := strings.Index(msg, `"`); j > 0 {
			msg = msg[:j]
		} else {
			cut = true // the stored note was truncated inside the message
		}
	}
	msg = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(msg), "…"))
	if msg == "" {
		return "", false
	}
	if len(msg) > 90 {
		msg, cut = strings.TrimSpace(msg[:90]), true
	}
	if cut {
		// An ellipsis, so a message ending mid-word reads as cut short rather
		// than as something the host actually said.
		msg += "…"
	}
	return msg, true
}
