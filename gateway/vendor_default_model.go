package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"memdoor/gateway/logs"
	"memdoor/gateway/providers"
)

// A VENDOR KEY WITH NO MODEL NAMED (#206, the maintainer 2026-11-09: "No need for
// MEMDOOR_MODEL"). The gateway picks the model on that provider: the cheapest,
// by the provider's own list price, that passed `memdoor model check`. With
// no passing report (a first start), it checks the cheapest models it has no
// report for until one passes, at most vendorPickChecks of them, and keeps
// the reports, so the next start reads them and spends nothing.

// vendorPickChecks bounds the calls a first start spends: each check is four
// or five calls on the key.
const vendorPickChecks = 3

// pickedReason is what `memdoor model` and the footer say about the model.
const pickedReason = "picked: the cheapest that passed model check"

// checkForPick runs one check; a test replaces it.
var checkForPick = func(s *Server, ctx context.Context, id string) modelCheck { return s.checkModel(ctx, id) }

// pickVendorModel is the model for providerID's key, or "" and why there is
// none.
func (s *Server) pickVendorModel(ctx context.Context, providerID string) (model, why string) {
	p, ok := providers.FindProvider(providerID)
	if !ok || !p.Connected() {
		return "", providerID + " is not connected: no model to pick"
	}
	list, err := providers.ModelsOf(ctx, p)
	if err != nil {
		return "", fmt.Sprintf("the model list of %s could not be read (%v): name a model with MEMDOOR_MODEL", providerID, err)
	}
	priced := cheapestFirst(list)
	if len(priced) == 0 {
		return "", fmt.Sprintf("%s states no price for its models, so none is the cheapest: name one with MEMDOOR_MODEL", providerID)
	}
	reports := loadModelChecks()
	for _, m := range priced {
		if c, ok := reports[providerID+":"+m.ID]; ok && c.Runs() {
			return m.ID, ""
		}
	}
	checked := 0
	for _, m := range priced {
		if checked == vendorPickChecks {
			break
		}
		id := providerID + ":" + m.ID
		if _, ok := reports[id]; ok {
			continue // it failed before
		}
		checked++
		logs.New("Brain").Info("picking a model for the "+providerID+" key: model check on its cheapest", slog.String("model", m.ID))
		c := checkForPick(s, ctx, id)
		saveModelCheck(c)
		if c.Runs() {
			return m.ID, ""
		}
	}
	return "", fmt.Sprintf("no model on %s passed model check (%d checked, cheapest first): name one with MEMDOOR_MODEL, or `memdoor model check %s:<id>`", providerID, checked, providerID)
}

// cheapestFirst is the models with a stated price, input plus output per
// million, cheapest first. A model with no price cannot be ranked and is
// left out. Prices are compared as the provider states them.
func cheapestFirst(list []providers.Model) []providers.Model {
	out := make([]providers.Model, 0, len(list))
	for _, m := range list {
		if m.InPerM+m.OutPerM > 0 {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].InPerM+out[i].OutPerM, out[j].InPerM+out[j].OutPerM
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}
