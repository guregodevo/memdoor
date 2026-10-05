package gateway

import (
	"encoding/json"
	"fmt"
	"strings"

	"memdoor/tools"
)

// toolFailureLedger counts, per tool and per turn, the failures that have not
// been redeemed by a success.
//
// Retirement exists because some tools cannot work in a given turn at all — a
// tool with no workspace answers "no workspace available" identically however
// many times it is asked, and the production traces put the cost of that retry
// amplification at roughly 4x compute. After enough failures the tool is
// refused unrun for the rest of the turn.
//
// What the plain counter got wrong is the evidence a SUCCESS carries. "No
// workspace available" does not become false mid-turn — but a tool that just
// returned a real result has demonstrated it is not in that category at all,
// and counting its earlier failures against it retires a working tool.
// Measured live 2026-08-30: todo_write failed, failed, SUCCEEDED, failed, and
// was retired on the last one — while the model's arguments were valid every
// single time (the real fault was an envelope the parser did not unwrap).
//
// So a success clears the tool's count. A permanently broken tool never gets
// one and is still retired on schedule.
//
// The ledger also keeps the last REASON. A refusal that says only "do not
// call it again" tells the model nothing it can act on, and what came back
// live (2026-09-16) was the same call four more times. The reason is what
// the next decision needs: a file name that does not exist is fixable, a
// missing workspace is not.
// failureKey is what the ledger counts against: the tool, and for apply_patch
// the files the patch names. A wall on one file is not a wall on the others:
// live 2026-09-30, three refusals on tools/apply_patch.go retired apply_patch
// for README.md too, and the model went on editing through bash scripts that
// no check covers (one of them cut docs/reference/CLI.md by 2,246 lines).
func failureKey(tool string, input []byte) string {
	if tool != tools.ApplyPatchDefinition.Name {
		return tool
	}
	files := tools.PatchTargets(input)
	if len(files) == 0 {
		return tool
	}
	return tool + " on " + strings.Join(files, ", ")
}

type toolFailures struct {
	n   int
	why string
}

type toolFailureLedger map[string]toolFailures

// THE SAME WALL THREE TIMES, NOT THREE DIFFERENT WALLS.
//
// Retirement is for a tool that cannot work in this turn at all, and the
// evidence for that is the SAME answer coming back. Counting any three
// failures retired a tool that was making progress: live 2026-09-17,
// a tool said "missing input for part 1 — make it first", it was made, it
// said the same of part 2, that was made too — and the third call, which
// would have worked, was refused. Each failure
// had already been fixed by the time the next one arrived.
//
// So a DIFFERENT reason resets the count. It is the same argument a
// success makes, one step earlier: this tool is not in the category the
// retirement exists for.
func (l toolFailureLedger) failed(name, why string) {
	f := l[name]
	if why != "" && f.why != "" && why != f.why {
		f.n = 0
	}
	f.n++
	if why != "" {
		f.why = why
	}
	l[name] = f
}

// succeeded clears the tool's failures: it has just proved it can work.
func (l toolFailureLedger) succeeded(name string) { delete(l, name) }

func (l toolFailureLedger) retired(name string, max int) bool { return l[name].n >= max }

func (l toolFailureLedger) count(name string) int     { return l[name].n }
func (l toolFailureLedger) reason(name string) string { return l[name].why }

// nearSameAsks counts the earlier inputs that say what input says.
//
// Two inputs are the same ask when they are equal, or when — read as the
// JSON objects tool inputs are — they have the same keys and every value
// matches: a long text by its words (nearSameOverlap of each in the
// other), anything short EXACTLY. The short rule is what keeps four
// fetches of four URLs, or three candidates calls on three sources, from
// counting as one ask: the words they share (url, height, topic) are the
// shape of the call, the one word that differs is the whole point (live
// 2026-09-18 12:04: the rebuild turn was ended for exactly that).
func nearSameAsks(earlier []string, input string) int {
	n := 0
	for _, e := range earlier {
		if nearSameInput(e, input) {
			n++
		}
	}
	return n
}

func nearSameInput(a, b string) bool {
	if a == b {
		return true
	}
	var oa, ob map[string]any
	if json.Unmarshal([]byte(a), &oa) != nil || json.Unmarshal([]byte(b), &ob) != nil {
		// Not objects: the whole text by its words, long texts only.
		wa, wb := toolWordSet(a), toolWordSet(b)
		return len(wa) >= nearSameMinWords && len(wb) >= nearSameMinWords &&
			toolWordOverlap(wa, wb) >= nearSameOverlap && toolWordOverlap(wb, wa) >= nearSameOverlap
	}
	if len(oa) != len(ob) {
		return false
	}
	for k, va := range oa {
		vb, ok := ob[k]
		if !ok {
			return false
		}
		sa, aStr := va.(string)
		sb, bStr := vb.(string)
		if aStr && bStr {
			wa, wb := toolWordSet(sa), toolWordSet(sb)
			if len(wa) < nearSameMinWords || len(wb) < nearSameMinWords {
				if strings.TrimSpace(sa) != strings.TrimSpace(sb) {
					return false
				}
				continue
			}
			if toolWordOverlap(wa, wb) < nearSameOverlap || toolWordOverlap(wb, wa) < nearSameOverlap {
				return false
			}
			continue
		}
		ja, _ := json.Marshal(va)
		jb, _ := json.Marshal(vb)
		if string(ja) != string(jb) {
			return false
		}
	}
	return true
}

const (
	nearSameOverlap  = 0.8
	nearSameMinWords = 8
)

func toolWordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x80)
	}) {
		out[w] = true
	}
	return out
}

func toolWordOverlap(a, b map[string]bool) float64 {
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

func ordinal(n int) string {
	switch n {
	case 1:
		return "first"
	case 2:
		return "second"
	case 3:
		return "third"
	case 4:
		return "fourth"
	case 5:
		return "fifth"
	case 6:
		return "sixth"
	case 7:
		return "seventh"
	}
	return fmt.Sprintf("%dth", n)
}

// lastToolFailure is " — tool: reason" for the last failed call of a turn,
// the reason cut to a line; "" when nothing failed.
func lastToolFailure(executed []ToolExecutionInfo) string {
	for i := len(executed) - 1; i >= 0; i-- {
		if executed[i].Error == "" {
			continue
		}
		why := strings.TrimSpace(executed[i].Error)
		if nl := strings.IndexByte(why, '\n'); nl > 0 {
			why = why[:nl]
		}
		if len(why) > 220 {
			why = why[:220] + "…"
		}
		return " — " + executed[i].Name + ": " + why
	}
	return ""
}
