package cmd

import (
	"strings"
	"testing"

	"memdoor/pkg/price"
)

// memdoor model search prints each price in its own currency: EUrouter
// states some models in euros, next to OpenRouter's dollars.
func TestModelSearchPrintsEachPriceInItsCurrency(t *testing.T) {
	out := renderModelCatalog("mistral-small", []catalogModel{
		{ID: "mistralai/mistral-small-3.2-24b-instruct", Provider: "openrouter", Context: 256000, InPerM: 0.07, OutPerM: 0.2},
		{ID: "mistral-small-3.2-24b", Provider: "eurouter", Context: 131072, InPerM: 0.09, OutPerM: 0.28, Currency: price.EUR},
	})
	for _, want := range []string{"$0.07 / $0.20", "€0.09 / €0.28"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%q missing:\n%s", want, out)
		}
	}
	if strings.Contains(out, "$0.09") {
		t.Fatalf("a euro price printed as dollars:\n%s", out)
	}
}
