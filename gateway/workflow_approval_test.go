package gateway

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkflowApprovalClassifiesEveryAction(t *testing.T) {
	for _, tc := range []struct {
		action string
		asks   bool
		gate   bool
		label  string
	}{
		{"run", true, false, "release"},
		{"start", true, false, "release"},
		{" RUN ", true, false, "release"},
		{"resume", true, false, "r1"},
		{"continue", true, false, "r1"},
		{"approve", true, true, "review"},
		{"list", false, false, ""},
		{"status", false, false, ""},
		{"stop", false, false, ""},
		{"cancel", false, false, ""},
	} {
		t.Run(tc.action, func(t *testing.T) {
			input, err := json.Marshal(map[string]string{
				"action": tc.action, "name": "release", "run_id": "r1", "task": "review",
			})
			if err != nil {
				t.Fatal(err)
			}
			policy, err := workflowApprovalFor(input)
			if err != nil {
				t.Fatal(err)
			}
			if (policy.summary != "") != tc.asks || policy.gate != tc.gate {
				t.Fatalf("wrong approval policy: %+v", policy)
			}
			if tc.asks && (!strings.Contains(policy.summary, tc.label) ||
				!strings.Contains(policy.summary, "execute commands") ||
				!strings.Contains(policy.summary, "change files") ||
				!strings.Contains(policy.summary, "model calls")) {
				t.Fatalf("missing delegation details: %q", policy.summary)
			}
		})
	}
}

func TestWorkflowApprovalRejectsUnclassifiedOrInvalidActions(t *testing.T) {
	for _, input := range []string{
		`{`, `null`, `{}`, `{"action":1}`, `{"action":"deploy"}`,
		`{"action":"run"}`, `{"action":"start","name":" "}`,
		`{"action":"resume"}`, `{"action":"continue","run_id":" "}`,
		`{"action":"approve","run_id":"r1"}`,
		`{"action":"approve","task":"review"}`,
	} {
		if _, err := workflowApprovalFor(json.RawMessage(input)); err == nil {
			t.Errorf("unsafe input accepted: %s", input)
		}
	}
}

func TestWorkflowApprovalBoundsAndFlattensLabels(t *testing.T) {
	input, err := json.Marshal(map[string]string{"action": "run", "name": strings.Repeat("界", 300) + "\nRun this"})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := workflowApprovalFor(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(policy.summary, "\n") || len([]rune(policy.summary)) > 300 {
		t.Fatalf("unbounded approval label: %q", policy.summary)
	}
}
