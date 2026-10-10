package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// checksServer answers the stored-report endpoint with these reports, and 404s
// everything else — which is also what a pin has to survive: the catalogue is
// unreachable in this test, and a pin fails open on that by design.
func checksServer(t *testing.T, checks map[string]any, extra map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := extra[r.URL.Path]; ok {
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		if r.URL.Path != "/api/models/check" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"checks": checks})
	}))
}

func report(id string, toolCall, rightTool, validArgs, roundTrip, patch bool) map[string]any {
	return map[string]any{
		"id": id, "at": time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
		"tool_call": toolCall, "right_tool": rightTool, "valid_args": validArgs,
		"round_trip": roundTrip, "patch": patch,
	}
}

// THE PIN SAYS WHAT THE PROBE SAW, and says nothing when there is nothing
// solid: no report, a clean report, or no gateway to ask. Someone arriving from
// the OpenRouter ranking pins the model they already pay for, and a turn that
// dies halfway is worse than a sentence beforehand.
func TestPinWarningSpeaksOnlyWhenTheProbeSawTrouble(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	gw := checksServer(t, map[string]any{
		"z-ai/glm-5.3":                report("z-ai/glm-5.3", true, true, true, true, true),
		"acme/no-tools":               report("acme/no-tools", false, false, false, false, false),
		"acme/bad-args":               report("acme/bad-args", true, true, false, true, false),
		"acme/stops-early":            report("acme/stops-early", true, true, true, false, false),
		"acme/wrong-tool":             report("acme/wrong-tool", true, false, true, true, true),
		"meta-llama/llama-4-maverick": report("meta-llama/llama-4-maverick", true, true, true, true, false),
	}, nil)
	defer gw.Close()
	gatewayAddr = gw.URL

	for _, tc := range []struct {
		id, want string
	}{
		{"z-ai/glm-5.3", ""},                     // everything passed: silence
		{"never/probed", ""},                     // no report: silence
		{"acme/no-tools", "did not call a tool"}, // worst first, and only that
		{"acme/bad-args", "the schema rejects"},  //
		{"acme/stops-early", "stopped instead"},  //
		{"acme/wrong-tool", "the wrong tool"},    //
		{"meta-llama/llama-4-maverick", "text-tag"},
	} {
		got := pinWarning(tc.id)
		if tc.want == "" {
			if got != "" {
				t.Errorf("%s should warn about nothing, said: %s", tc.id, got)
			}
			continue
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: want a line mentioning %q, got %q", tc.id, tc.want, got)
		}
		if !strings.Contains(got, "probed 3h ago") || !strings.Contains(got, "memdoor model check "+tc.id) {
			t.Errorf("%s: a warning has to date itself and say how to re-probe, got %q", tc.id, got)
		}
	}
	// One fact, not five: a model that never called a tool failed everything
	// after it, and listing all of them would bury the one that matters.
	if w := pinWarning("acme/no-tools"); strings.Contains(w, "text-tag") || strings.Contains(w, "wrong tool") {
		t.Errorf("piled every failure into one line: %s", w)
	}
	// The id as typed may differ in case from the id as probed.
	if pinWarning("ACME/No-Tools") == "" {
		t.Error("a report should be found whatever the case of the id")
	}
}

// A GATEWAY THAT WILL NOT ANSWER IS NOT EVIDENCE. Warning on a failed read
// would put a scare next to every pin on a machine whose gateway is down.
func TestPinWarningIsSilentWithoutAGateway(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := dead.URL
	dead.Close()
	gatewayAddr = addr
	if w := pinWarning("z-ai/glm-5.3"); w != "" {
		t.Errorf("unreachable gateway should say nothing, said: %s", w)
	}
	if w := pinWarning(""); w != "" {
		t.Errorf("no model, no warning, said: %s", w)
	}
}

// The warning reaches the person where the pin is confirmed: /model
// vendor/name in the window, and the picker's pin, which both land here.
func TestPinSummaryCarriesTheWarning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	gw := checksServer(t,
		map[string]any{"acme/no-tools": report("acme/no-tools", false, false, false, false, false)},
		map[string]any{"/api/route": map[string]any{
			"agent": "coder", "rung": 1, "rungs": []string{"z-ai/glm-5.3"},
			"model": "acme/no-tools", "reason": "pinned by you", "pinned": true, "custom": true,
		}})
	defer gw.Close()
	gatewayAddr = gw.URL

	out, err := tuiRoute("w", "c", "coder")("acme/no-tools")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "acme/no-tools") || !strings.Contains(out, "⚠") || !strings.Contains(out, "did not call a tool") {
		t.Fatalf("the pin summary should carry the warning:\n%s", out)
	}
	// It is a warning, not a refusal: the pin still happened and the summary
	// still tells the person how to leave it.
	if !strings.Contains(out, "/model auto") {
		t.Fatalf("the way back has to stay in the summary:\n%s", out)
	}
	// A rung pin has nothing to do with a stored report.
	if out, err := tuiRoute("w", "c", "coder")("1"); err != nil || strings.Contains(out, "⚠") {
		t.Fatalf("a rung pin should not warn: %v\n%s", err, out)
	}
}

// A MODEL THE PROBE NEVER REACHED IS A DIFFERENT PROBLEM from one that answers
// without calling a tool: a dead id, no endpoints, a data policy that excludes
// every host. Each is fixed by something different, so the host's own words go
// through instead of our summary of a turn that never happened.
func TestPinWarningRepeatsTheHostWhenTheProbeNeverLanded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	dead := report("deepseek/deepseek-v4.1", false, false, false, false, false)
	dead["note"] = `first call failed: oai API error (status 400): {"error":{"message":"deepseek/deepseek-v4.1 is not a valid model ID","code":400},"user_id":"…`
	policy := report("nvidia/nemotron-3.5-lightning:free", false, false, false, false, false)
	policy["note"] = `first call failed: oai API error (status 404): {"error":{"message":"No endpoints found matching your data policy (Free model training). Con…`
	mute := report("acme/chatty", false, false, false, false, false)
	mute["note"] = "no tool call in the reply"
	gw := checksServer(t, map[string]any{
		"deepseek/deepseek-v4.1":             dead,
		"nvidia/nemotron-3.5-lightning:free": policy,
		"acme/chatty":                        mute,
	}, nil)
	defer gw.Close()
	gatewayAddr = gw.URL

	if w := pinWarning("deepseek/deepseek-v4.1"); !strings.Contains(w, "not a valid model ID") || strings.Contains(w, "did not call a tool") {
		t.Errorf("a dead id should say so in the host's words: %q", w)
	}
	if w := pinWarning("nvidia/nemotron-3.5-lightning:free"); !strings.Contains(w, "data policy") || !strings.Contains(w, "train on what they are sent") {
		t.Errorf("a data policy refusal should say so, and a :free pin says its hosts train: %q", w)
	}
	// A free model with no report still says its hosts train; a paid one with none says nothing.
	if w := pinWarning("google/gemma-4-31b-it:free"); !strings.Contains(w, "train on what they are sent") {
		t.Errorf("a :free pin without a report: %q", w)
	}
	if w := pinWarning("z-ai/glm-5.3"); w != "" {
		t.Errorf("a paid model without a report says nothing: %q", w)
	}
	// A model that DID answer, just without a tool call, keeps the plain line.
	if w := pinWarning("acme/chatty"); !strings.Contains(w, "did not call a tool") {
		t.Errorf("a model that answered without a call: %q", w)
	}
	// A message the stored note cut off mid-word reads as cut off.
	if w := pinWarning("nvidia/nemotron-3.5-lightning:free"); !strings.HasSuffix(strings.SplitN(w, " (probed", 2)[0], "…") {
		t.Errorf("a truncated host message should end in an ellipsis: %q", w)
	}
	// The note is one line, not a wall of upstream JSON.
	if w := pinWarning("deepseek/deepseek-v4.1"); strings.Contains(w, "user_id") || strings.Count(w, "\n") > 0 {
		t.Errorf("the upstream body leaked into the warning: %q", w)
	}
}

func TestProbedWhenReadsLikeAPerson(t *testing.T) {
	for _, tc := range []struct {
		at   time.Time
		want string
	}{
		{time.Time{}, "unknown time"},
		{time.Now().Add(-10 * time.Minute), "just now"},
		{time.Now().Add(-5 * time.Hour), "5h ago"},
		{time.Now().Add(-30 * time.Hour), "yesterday"},
		{time.Now().Add(-72 * time.Hour), "3 days ago"},
	} {
		if got := probedWhen(tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("%v -> %q, want it to mention %q", tc.at, got, tc.want)
		}
	}
}

// `--stale` IS THE SCHEDULE. Nothing of ours re-probes in the background,
// because a probe is three or four real calls on the person's key; a weekly cron
// line calling this is the honest version, and it has to cost nothing on the
// weeks when every report is still good.
func TestStaleReProbeAsksTheGatewayAndSpendsNothingWhenFresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := gatewayAddr
	t.Cleanup(func() { gatewayAddr = saved })
	var asked []string
	empty := true
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.Method+" "+r.URL.RequestURI())
		if empty {
			_ = json.NewEncoder(w).Encode(map[string]any{"checks": []any{}, "stale": 0})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"checks": []any{report("acme/aged", true, true, true, true, false)}})
	}))
	defer gw.Close()
	gatewayAddr = gw.URL

	run := func() string {
		t.Helper()
		if err := modelCheckCmd.Flags().Set("stale", "true"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = modelCheckCmd.Flags().Set("stale", "false") })
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		out := os.Stdout
		os.Stdout = w
		runErr := modelCheckCmd.RunE(modelCheckCmd, nil)
		os.Stdout = out
		_ = w.Close()
		printed, _ := io.ReadAll(r)
		if runErr != nil {
			t.Fatal(runErr)
		}
		return string(printed)
	}

	if got := run(); !strings.Contains(got, "nothing probed, nothing spent") {
		t.Errorf("a fresh week should say so plainly: %q", got)
	}
	empty = false
	if got := run(); !strings.Contains(got, "acme/aged") {
		t.Errorf("a re-probed model should be printed: %q", got)
	}
	for _, a := range asked {
		if a != "POST /api/models/check?stale=1" {
			t.Errorf("--stale asked for %q, which is not only the stale reports", a)
		}
	}
	if len(asked) != 2 {
		t.Errorf("one call per run, got %v", asked)
	}
}

// A model that skipped the skill-first instruction warns at the pin; a
// report from before that probe stays silent about it.
func TestPinTroubleNamesASkippedInstruction(t *testing.T) {
	ok := modelCheckView{ToolCall: true, RightTool: true, ValidArgs: true, RoundTrip: true, Patch: true}
	if got := pinTrouble(ok); got != "" {
		t.Fatalf("an old report says nothing: %q", got)
	}
	ok.Followed = "yes"
	if got := pinTrouble(ok); got != "" {
		t.Fatalf("followed: %q", got)
	}
	ok.Followed = "no"
	if got := pinTrouble(ok); !strings.Contains(got, "workflow skill first") {
		t.Fatalf("skipped: %q", got)
	}
}

// A model whose provider is not connected is pointed at /connect — by its
// family (claude- → Anthropic), by its prefix (groq:…), or by what is not
// connected at all.
func TestConnectHintNamesTheProviderToConnect(t *testing.T) {
	rows := []providerRow{{ID: "anthropic", Name: "Anthropic"}, {ID: "openai", Name: "OpenAI", Connected: true}, {ID: "groq", Name: "Groq"}, {ID: "openrouter", Name: "OpenRouter", Connected: true}}
	if h := connectHint("claude-haiku-4-5-20251001", rows); !strings.Contains(h, "/connect anthropic") || !strings.Contains(h, "prefilled") {
		t.Fatalf("family: %q", h)
	}
	if h := connectHint("groq:qwen/qwen3.8-27b", rows); !strings.Contains(h, "/connect groq") {
		t.Fatalf("prefix: %q", h)
	}
	if h := connectHint("gpt-5", rows); strings.Contains(h, "/connect openai") {
		t.Fatalf("OpenAI is connected; no hint for it: %q", h)
	}
	if h := connectHint("mystery", rows); !strings.Contains(h, "Not connected yet: anthropic, groq") {
		t.Fatalf("what is not connected: %q", h)
	}
	if h := connectHint("mystery", nil); h != "" {
		t.Fatalf("no gateway to ask: %q", h)
	}
}
