package providers

import "testing"

// A task that names its hosts gets their own reply ceiling: the smallest
// among the named hosts that list one. Qwen3.8-27B is listed model-wide at
// 65,536 (its top host's), the hosts a bench orders at 235,929; held to the
// model's figure, a 128K plan call was cut mid-thought (2026-10-29).
func TestNamedHostsGiveTheirOwnReplyCeiling(t *testing.T) {
	old := fetchEndpoints
	calls := 0
	fetchEndpoints = func(key, id string) ([]Endpoint, error) {
		calls++
		return []Endpoint{
			{Tag: "modal", MaxCompletion: 235929},
			{Tag: "parasail/fp8", MaxCompletion: 235929},
			{Tag: "chutes/fp8", MaxCompletion: 65536},
			{Tag: "wafer"},
		}, nil
	}
	t.Cleanup(func() { fetchEndpoints = old })
	m := "qwen/qwen3.8-27b-test"
	if got := HostsOutputCeiling("k", m, []string{"modal", " Parasail", "wafer"}); got != 235929 {
		t.Errorf("modal, parasail, wafer (unlisted): %d", got)
	}
	if got := HostsOutputCeiling("k", m, []string{"modal", "chutes"}); got != 65536 {
		t.Errorf("a named host with a smaller ceiling sets it: %d", got)
	}
	if got := HostsOutputCeiling("k", m, []string{"nobody"}); got != 0 {
		t.Errorf("no named host listed: %d", got)
	}
	if got := HostsOutputCeiling("", m, []string{"modal"}); got != 0 {
		t.Errorf("no key: %d", got)
	}
	if calls != 1 {
		t.Errorf("the listing was read %d times, want once (cached)", calls)
	}
}
