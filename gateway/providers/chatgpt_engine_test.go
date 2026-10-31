package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/pkg/chatgpt"
	"memdoor/pkg/shared"
)

// planAndKey is a signed-in plan and an OPENAI_API_KEY that list the same
// id, served by one fake: the plan answers its bearer with planModels (an
// empty list, or a 500 when planDown).
func planAndKey(t *testing.T, planModels []string, planDown bool) {
	t.Helper()
	clearProviderEnv(t)
	_ = logs.InitGlobalLogger(t.TempDir(), false)
	if err := chatgpt.NewStore(shared.MemdoorHome()).Save(&chatgpt.Credential{ClientID: "oaiapp_1", AccessToken: "plan-token", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer plan-token") {
			if planDown {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var list []map[string]string
			for _, id := range planModels {
				list = append(list, map[string]string{"slug": id, "visibility": "list"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"models": list})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "gpt-6-astra"}, {"id": "gpt-4.1-mini"}}})
	}))
	t.Cleanup(srv.Close)
	oldBase := chatgptBase
	chatgptBase = srv.URL + "/v1"
	t.Cleanup(func() { chatgptBase = oldBase })
	t.Setenv("OPENAI_API_KEY", "sk-key")
	t.Setenv("OPENAI_BASE_URL", srv.URL+"/v1")
}

// A signed-in plan is an engine on the first model it lists (the list is
// OpenAI's: the mutation check, list[len-1], picks gpt-6-astra).
func TestASignedInPlanIsAnEngineOnItsFirstModel(t *testing.T) {
	planAndKey(t, []string{"gpt-6.1-sol", "gpt-6-astra"}, false)
	re, ok := ChatGPTPlanEngine(context.Background())
	if !ok || !re.IsChatGPTPlan() || re.Model != "gpt-6.1-sol" {
		t.Fatalf("the plan's engine: %v %+v", ok, re)
	}
	if err := SetRemoteEngine(re); err != nil {
		t.Fatal(err)
	}
	if ActiveProviderID() != ChatGPTID {
		t.Fatalf("the plan answers: active %q", ActiveProviderID())
	}
}

// No engine from a plan that is not signed in, lists nothing or does not
// answer: the next engine in line answers then.
func TestNoPlanEngineWithoutAListedModel(t *testing.T) {
	planAndKey(t, nil, false)
	if _, ok := ChatGPTPlanEngine(context.Background()); ok {
		t.Fatal("a plan that lists no model is no engine")
	}
	planAndKey(t, []string{"gpt-6.1-sol"}, true)
	if _, ok := ChatGPTPlanEngine(context.Background()); ok {
		t.Fatal("a plan that does not answer is no engine")
	}
	clearProviderEnv(t)
	if _, ok := ChatGPTPlanEngine(context.Background()); ok {
		t.Fatal("no sign-in, no engine")
	}
}

// The order a choice is offered in: the answering provider first, then a
// signed-in plan before the OpenAI key, then the registry's (the mutation
// check: drop the plan's rank and openai comes first, as it did live).
func TestAnswerOrderPutsThePlanBeforeTheKey(t *testing.T) {
	planAndKey(t, []string{"gpt-6-astra"}, false)
	ids := func() []string {
		var out []string
		for _, p := range AnswerOrder() {
			out = append(out, p.ID)
		}
		return out
	}
	before := func(list []string, a, b string) bool {
		ia, ib := -1, -1
		for i, id := range list {
			if id == a {
				ia = i
			}
			if id == b {
				ib = i
			}
		}
		return ia >= 0 && ib >= 0 && ia < ib
	}
	if got := ids(); !before(got, ChatGPTID, "openai") {
		t.Fatalf("the plan before the key: %v", got)
	}
	if err := SetRemoteEngine(RemoteEngine{Vendor: VendorOpenAI, Endpoint: "https://api.openai.com/v1", APIKey: "sk-key", Model: "gpt-4.1-mini"}); err != nil {
		t.Fatal(err)
	}
	if got := ids(); got[0] != "openai" {
		t.Fatalf("the answering provider first: %v", got)
	}
	if len(AnswerOrder()) != len(Providers()) {
		t.Fatal("the order keeps every provider")
	}
}
