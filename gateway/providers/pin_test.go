package providers

import (
	"strings"
	"testing"
)

// A pin on a provider kept by an env var's NAME, the var not set where the
// gateway started (an /update restarted it without the person's shell):
// the pin is unreachable, and the reason names the variable.
func TestAPinOnAProviderWithNoKeyIsUnreachable(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("CORP_PIN_TEST_KEY", "")
	if err := SaveProvider(ProviderSpec{ID: "corp", API: APIChat, Base: "http://127.0.0.1:1", Key: "CORP_PIN_TEST_KEY", Models: []Model{{ID: "m1"}}}); err != nil {
		t.Fatal(err)
	}
	why := UnreachablePin("corp:deepseek/deepseek-v3.2")
	if !strings.HasPrefix(why, "corp is not connected") || !strings.Contains(why, "CORP_PIN_TEST_KEY"+KeyNameNotSet) {
		t.Fatalf("why = %q", why)
	}
}

// A pin on a connected provider, a pin with no provider prefix and a prefix
// that is not a provider's id are tried as before.
func TestAPinThatCanAnswerIsNotUnreachable(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("CORP_PIN_TEST_KEY", "k")
	if err := SaveProvider(ProviderSpec{ID: "corp", API: APIChat, Base: "http://127.0.0.1:1", Key: "CORP_PIN_TEST_KEY", Models: []Model{{ID: "m1"}}}); err != nil {
		t.Fatal(err)
	}
	for _, pin := range []string{"corp:m1", "z-ai/glm-5.3-flash", "nosuchprovider:m1"} {
		if why := UnreachablePin(pin); why != "" {
			t.Errorf("%q: unreachable %q", pin, why)
		}
	}
}

// A built-in with no key (its key source is cleared) still says why.
func TestAPinOnABuiltInWithNoKeySaysWhy(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("AIMLAPI_API_KEY", "")
	if why := UnreachablePin("aimlapi:deepseek/deepseek-thinking-v3.2-exp"); why != "aimlapi is not connected: this gateway has no key for it" {
		t.Fatalf("why = %q", why)
	}
}
