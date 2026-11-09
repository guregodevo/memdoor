package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/providers"
)

// A vendor's list, EUrouter's shape: four priced chat models with tools and
// one with no price.
const pickList = `{"data":[
 {"id":"c-dear","name":"C","context_length":131072,"supported_parameters":["tools"],"supported_api_endpoints":["chat.completions"],"pricing":{"prompt":"0.000001","completion":"0.000002","currency":"USD"}},
 {"id":"a-cheapest","name":"A","context_length":131072,"supported_parameters":["tools"],"supported_api_endpoints":["chat.completions"],"pricing":{"prompt":"0.0000001","completion":"0.0000002","currency":"USD"}},
 {"id":"d-unpriced","name":"D","context_length":131072,"supported_parameters":["tools"],"supported_api_endpoints":["chat.completions"],"pricing":{"prompt":"0","completion":"0","currency":"USD"}},
 {"id":"b-middle","name":"B","context_length":131072,"supported_parameters":["tools"],"supported_api_endpoints":["chat.completions"],"pricing":{"prompt":"0.0000003","completion":"0.0000006","currency":"USD"}},
 {"id":"e-dearest","name":"E","context_length":131072,"supported_parameters":["tools"],"supported_api_endpoints":["chat.completions"],"pricing":{"prompt":"0.000002","completion":"0.000004","currency":"USD"}}]}`

// vendorKeyGateway is a HOME with a kept OpenRouter key and EUROUTER_API_KEY
// in the environment, no MEMDOOR_MODEL, the EUrouter list served by the test
// and every check answered by pass (the ids it was asked for are returned).
func vendorKeyGateway(t *testing.T, pass func(id string) bool) *[]string {
	t.Helper()
	keptKeyGateway(t, false, true, freeCatalog, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, pickList)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("EUROUTER_API_KEY", "eur_test")
	t.Setenv("EUROUTER_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("MEMDOOR_MODEL_CONTEXT", "")
	providers.ForgetModels(providers.VendorEURouter)
	t.Cleanup(func() { providers.ForgetModels(providers.VendorEURouter) })
	var asked []string
	saved := checkForPick
	checkForPick = func(_ *Server, _ context.Context, id string) modelCheck {
		asked = append(asked, id)
		ok := pass(id)
		return modelCheck{ID: id, At: time.Now(), ToolCall: ok, ValidArgs: ok, RoundTrip: ok}
	}
	t.Cleanup(func() { checkForPick = saved })
	return &asked
}

func passes(ids ...string) func(string) bool {
	return func(id string) bool {
		for _, p := range ids {
			if id == p {
				return true
			}
		}
		return false
	}
}

// With reports kept, the cheapest model that passed serves the vendor key,
// nothing is checked, and the vendor key wins over the kept OpenRouter key.
func TestAVendorKeyRunsOnTheCheapestModelThatPassed(t *testing.T) {
	asked := vendorKeyGateway(t, passes())
	saveModelCheck(modelCheck{ID: "eurouter:c-dear", At: time.Now(), ToolCall: true, ValidArgs: true, RoundTrip: true})
	saveModelCheck(modelCheck{ID: "eurouter:b-middle", At: time.Now(), ToolCall: true, ValidArgs: true, RoundTrip: true})
	saveModelCheck(modelCheck{ID: "eurouter:a-cheapest", At: time.Now(), ToolCall: true})
	(&Server{}).selectModelEngine()
	re := providers.ActiveRemoteEngine()
	if re == nil || re.Vendor != providers.VendorEURouter || re.Model != "b-middle" || re.Picked != pickedReason {
		t.Fatalf("the cheapest that passed, on the vendor key: %+v", re)
	}
	if len(*asked) != 0 {
		t.Fatalf("a kept report spends nothing: %v", *asked)
	}
}

// A first start has no report: the cheapest models are checked in order
// until one passes, and the next start reads the kept reports.
func TestAFirstStartChecksTheCheapestUntilOnePasses(t *testing.T) {
	asked := vendorKeyGateway(t, passes("eurouter:b-middle"))
	(&Server{}).selectModelEngine()
	re := providers.ActiveRemoteEngine()
	if re == nil || re.Model != "b-middle" || re.Picked != pickedReason {
		t.Fatalf("picked after its check: %+v", re)
	}
	if strings.Join(*asked, ",") != "eurouter:a-cheapest,eurouter:b-middle" {
		t.Fatalf("cheapest first, stopping at the first pass: %v", *asked)
	}
	_ = providers.ClearRemoteEngine()
	(&Server{}).selectModelEngine()
	if len(*asked) != 2 || providers.ActiveRemoteEngine().Model != "b-middle" {
		t.Fatalf("the next start reads the reports: %v %+v", *asked, providers.ActiveRemoteEngine())
	}
}

// At most vendorPickChecks models are checked. When none passes, the gateway
// runs as before: the warning names MEMDOOR_MODEL, and the OpenRouter key
// answers. A model with no price is never checked.
func TestAPickChecksAtMostThreeModels(t *testing.T) {
	asked := vendorKeyGateway(t, passes())
	(&Server{}).selectModelEngine()
	if strings.Join(*asked, ",") != "eurouter:a-cheapest,eurouter:b-middle,eurouter:c-dear" {
		t.Fatalf("three checks, cheapest first, none unpriced: %v", *asked)
	}
	re := providers.ActiveRemoteEngine()
	if re == nil || re.Vendor != "" || !re.Byok {
		t.Fatalf("no pick: the OpenRouter key answers as before: %+v", re)
	}
	(&Server{}).selectModelEngine()
	if len(*asked) != 4 || (*asked)[3] != "eurouter:e-dearest" {
		t.Fatalf("a model that failed is not checked again: %v", *asked)
	}
}

// MEMDOOR_MODEL still names the model: nothing is picked or checked.
func TestMEMDOORMODELStillNamesTheModel(t *testing.T) {
	asked := vendorKeyGateway(t, passes())
	t.Setenv("MEMDOOR_MODEL", "c-dear")
	(&Server{}).selectModelEngine()
	re := providers.ActiveRemoteEngine()
	if re == nil || re.Vendor != providers.VendorEURouter || re.Model != "c-dear" || re.Picked != "" || len(*asked) != 0 {
		t.Fatalf("the named model, not picked: %+v %v", re, *asked)
	}
}

// A model with no price cannot be the cheapest; a list that prices none
// gives no candidate.
func TestCheapestFirstRanksPricedModelsOnly(t *testing.T) {
	got := cheapestFirst([]providers.Model{{ID: "z", InPerM: 1, OutPerM: 1}, {ID: "free"}, {ID: "y", InPerM: 0.5, OutPerM: 1.5}, {ID: "a", InPerM: 0.1, OutPerM: 0.2}})
	ids := make([]string, 0, len(got))
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "a,y,z" {
		t.Fatalf("priced, cheapest first, ties by id: %v", ids)
	}
	if len(cheapestFirst([]providers.Model{{ID: "free"}})) != 0 {
		t.Fatal("no price, no candidate")
	}
}

// `memdoor model` and the footer say the model was picked.
func TestTheRouteSaysTheModelWasPicked(t *testing.T) {
	v := viewRoute(&providers.RemoteEngine{Model: "b-middle", Vendor: providers.VendorEURouter, Picked: pickedReason}, nil, "coder")
	if v.Model != "b-middle" || v.Reason != pickedReason {
		t.Fatalf("the reason is the pick: %+v", v)
	}
	if v := viewRoute(&providers.RemoteEngine{Model: "m"}, nil, "coder"); v.Reason != "first rung · the default" {
		t.Fatalf("an engine with no pick reads as before: %+v", v)
	}
}
