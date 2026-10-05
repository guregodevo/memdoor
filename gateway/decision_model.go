package gateway

import (
	"memdoor/gateway/providers"
	"memdoor/pkg/decision"
	"memdoor/pkg/decision/systemone"
)

// autoDecisionModel picks the decision provider; there is no setting.
//
// THE PERSON'S OWN KEY FIRST (Greg, 2026-10-03: "Jev at user key by default",
// "jev decision model is not pro"). A decision key, or the OpenRouter key they
// code with, judges on their own account. Never through the broker: no seat
// supplies a key (Greg, 2026-10-04: "Pro seat never offer key. It's always
// with user key"). On a company's vendor
// key nothing but the vendor may be contacted (providers/vendor.go), so there
// only a decision key the gateway was given explicitly judges.
//
// "" when none: decisions stay off and every consumer behaves as it did before
// the feature existed.
func autoDecisionModel(direct decision.Provider, codingKey bool) string {
	vendor := providers.VendorMode()
	switch {
	case direct != nil && direct.Ready() && !(codingKey && vendor):
		return systemone.ProviderID
	}
	return ""
}
