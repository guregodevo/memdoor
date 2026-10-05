package providers

import (
	"strings"
	"testing"
)

// A 402 becomes one line with what to do, without the account's URLs; the
// balance it can afford is read so the request can be retried within it.
func TestProviderRefusalIsOneHumanLine(t *testing.T) {
	body := []byte(`{"error":{"message":"This request requires more credits, or fewer max_tokens. You requested up to 16384 tokens, but can only afford 7268. To increase, visit https://openrouter.ai/workspaces/default/keys/abc and adjust the key's daily limit","code":402,"metadata":{"previous_errors":[{"code":402,"message":"again"},{"code":402,"message":"again"}]}}}`)
	err := providerRefusal(402, body).Error()
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
	if !strings.Contains(providerRefusal(500, []byte("upstream exploded")).Error(), "HTTP 500") {
		t.Fatal("other statuses keep their code")
	}
}

// A context-length refusal keeps the provider's count of the prompt: the turn
// reads it to recalibrate, shed and ask again (live 2026-09-29, the count was
// cut off with the second sentence and the turn died on the refusal).
func TestAContextRefusalKeepsTheCount(t *testing.T) {
	body := []byte(`{"error":{"message":"This endpoint's maximum context length is 32768 tokens. However, you requested about 37623 tokens (21239 of text input, 16384 in the output). Please reduce the length of either one, or use the context-compression plugin to compress your prompt automatically.","code":400}}`)
	got := providerRefusal(400, body).Error()
	want := "the model provider refused the request (HTTP 400): This endpoint's maximum context length is 32768 tokens. However, you requested about 37623 tokens (21239 of text input, 16384 in the output)"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
