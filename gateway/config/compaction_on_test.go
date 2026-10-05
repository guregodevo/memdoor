package config

import (
	"encoding/json"
	"testing"
)

// compaction.enabled: unset is on; only an explicit false turns it off.
func TestCompactionIsOnUnlessTurnedOff(t *testing.T) {
	var none *CompactionConfig
	if !none.On() || !(&CompactionConfig{}).On() {
		t.Fatal("no config, or no enabled key: on")
	}
	var off CompactionConfig
	if err := json.Unmarshal([]byte(`{"enabled": false}`), &off); err != nil {
		t.Fatal(err)
	}
	if off.On() {
		t.Fatal("enabled: false is off")
	}
}
