package gateway

import (
	"strings"
	"testing"
)

func TestByokBandThresholds(t *testing.T) {
	// The argument is OpenRouter's per-token price, as the API gives it.
	for _, tc := range []struct {
		perToken float64
		want     string
	}{
		{0, "free"},
		{0.04 / 1e6, "$"},
		{0.38 / 1e6, "$$"},
		{3 / 1e6, "$$$"},
	} {
		if got := byokBand(tc.perToken); got != tc.want {
			t.Errorf("byokBand(%g) = %s, want %s", tc.perToken, got, tc.want)
		}
	}
}

// Without an OpenRouter key the catalogue is not asked for — on a company's
// vendor key nothing may be (providers/vendor.go).
func TestCatalogueIsNotFetchedWithoutAKey(t *testing.T) {
	var c byokCatalog
	if _, err := c.models(""); err == nil || !strings.Contains(err.Error(), "no OpenRouter key") {
		t.Fatalf("empty key must not fetch: %v", err)
	}
}
