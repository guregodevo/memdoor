package providers

import (
	"strings"
	"testing"
)

// A 402 becomes one line with what to do, without the account's URLs; the
// balance it can afford is read so the request can be retried within it.
func TestProviderRefusalIsOneHumanLine(t *testing.T) {
	body := []byte(`{"error":{"message":"This request requires more credits, or fewer max_tokens. You requested up to 16384 tokens, but can only afford 7268. To increase, visit https://openrouter.ai/workspaces/default/keys/abc and adjust the key's daily limit","code":402,"metadata":{"previous_errors":[{"code":402,"message":"again"},{"code":402,"message":"again"}]}}}`)
	err := providerRefusal(OpenRouterBase, 402, body).Error()
	if !strings.Contains(err, "out of credit") || !strings.Contains(err, "openrouter.ai/settings/credits") {
		t.Fatalf("must say what happened and what to do: %s", err)
	}
	if strings.Contains(err, "keys/abc") || strings.Contains(err, "previous_errors") || len(err) > 400 {
		t.Fatalf("no account URLs, no repeated errors, one line: %s", err)
	}
	if got := affordableTokens(402, body); got != 7268 {
		t.Fatalf("affordable tokens = %d, want 7268", got)
	}
	if affordableTokens(429, body) != 0 || affordableTokens(402, []byte(`{"error":{"message":"no credit"}}`)) != 0 {
		t.Fatal("only a 402 that names an amount is retried")
	}
	if !strings.Contains(providerRefusal(OpenRouterBase, 500, []byte("upstream exploded")).Error(), "HTTP 500") {
		t.Fatal("other statuses keep their code")
	}
}

// A context-length refusal keeps the provider's count of the prompt: the turn
// reads it to recalibrate, shed and ask again (live 2026-09-29, the count was
// cut off with the second sentence and the turn died on the refusal).
func TestAContextRefusalKeepsTheCount(t *testing.T) {
	body := []byte(`{"error":{"message":"This endpoint's maximum context length is 32768 tokens. However, you requested about 37623 tokens (21239 of text input, 16384 in the output). Please reduce the length of either one, or use the context-compression plugin to compress your prompt automatically.","code":400}}`)
	got := providerRefusal(OpenRouterBase, 400, body).Error()
	want := "the model provider refused the request (HTTP 400): This endpoint's maximum context length is 32768 tokens. However, you requested about 37623 tokens (21239 of text input, 16384 in the output)"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// A refusal from any other provider names that provider, not OpenRouter: an
// LLM Gateway account with no credit was sent to openrouter.ai (2026-11-09).
func TestARefusalNamesTheHostThatRefused(t *testing.T) {
	body := []byte(`{"error":{"message":"No API key set for provider and organization has insufficient credits","code":"billing_error"}}`)
	got := providerRefusal(llmGatewayDefaultBase, 402, body).Error()
	if !strings.Contains(got, "llmgateway.io refused the request: the account is out of credit") || !strings.Contains(got, "Add credit at llmgateway.io") || strings.Contains(got, "OpenRouter") || strings.Contains(got, "openrouter") {
		t.Fatalf("LLM Gateway's 402: %s", got)
	}
	got = providerRefusal(euRouterDefaultBase, 401, []byte(`{"error":{"message":"invalid key"}}`)).Error()
	if got != "eurouter.ai refused the key (HTTP 401): invalid key. Check the key" {
		t.Fatalf("EUrouter's 401: %s", got)
	}
}

// Vercel's card refusal (403 customer_verification_required, live
// 2026-11-09) reads as Vercel's, not OpenRouter's.
func TestVercelsCardRefusalNamesVercel(t *testing.T) {
	card := []byte(`{"error":{"message":"AI Gateway requires a valid credit card on file to service requests. Please visit https://vercel.com/d?to=x to add a card.","type":"customer_verification_required"}}`)
	got := providerRefusal(vercelDefaultBase, 403, card).Error()
	want := "ai-gateway.vercel.sh refused the key (HTTP 403): AI Gateway requires a valid credit card on file to service requests. Check the key"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
