package context

import "testing"

// The estimate follows the server: a refusal lifts the factor at once to
// the server's count, an accepted answer moves it halfway, and it never
// drops below one. The mutation check: return the bare chars/4 from
// scaledEstimate and the lifted estimate stays put.
func TestEstimatesLearnTheServersCount(t *testing.T) {
	setTokenScale(tokenScaleStart)
	t.Cleanup(func() { setTokenScale(tokenScaleStart) })
	tc := NewTokenCounter("m", false)
	text := make([]byte, 4000)
	for i := range text {
		text[i] = 'a'
	}
	if got := tc.EstimateTokens(string(text)); got != 1000 {
		t.Fatalf("chars/4 to begin with, got %d", got)
	}
	CalibrateTokensHard(61444, 50000) // the server refused: 1.23×
	if got := tc.EstimateTokens(string(text)); got < 1225 || got > 1230 {
		t.Fatalf("after the refusal the estimate carries the server's factor, got %d", got)
	}
	CalibrateTokensHard(40000, 50000) // a smaller ratio never lowers a hard floor
	if got := tc.EstimateTokens(string(text)); got < 1225 {
		t.Fatalf("a hard calibration never drops the factor, got %d", got)
	}
	setTokenScale(tokenScaleStart)
	CalibrateTokens(60000, 50000) // accepted: halfway from 1.0 to 1.2
	if s := TokenScale(); s < 1.09 || s > 1.11 {
		t.Fatalf("an accepted answer moves the factor halfway, got %.3f", s)
	}
	CalibrateTokens(10000, 50000) // never below one
	if s := TokenScale(); s < 1.0 {
		t.Fatalf("the factor never drops below one, got %.3f", s)
	}
}

func TestParseContextLengthError(t *testing.T) {
	msg := "remote engine brain: This model's maximum context length is 65536 tokens. However, you requested 4093 output tokens and your prompt contains at least 61444 input tokens, for a total of at least 65537 tokens. Please reduce the length"
	w, o, in, ok := ParseContextLengthError(msg)
	if !ok || w != 65536 || o != 4093 || in != 61444 {
		t.Fatalf("got %d %d %d %v", w, o, in, ok)
	}
	// OpenRouter's wording, as the provider refusal reports it (live 2026-09-29).
	or := "the model provider refused the request (HTTP 400): This endpoint's maximum context length is 32768 tokens. However, you requested about 37623 tokens (21239 of text input, 16384 in the output)"
	if w, o, in, ok := ParseContextLengthError(or); !ok || w != 32768 || o != 16384 || in != 21239 {
		t.Fatalf("OpenRouter: got %d %d %d %v", w, o, in, ok)
	}
	if _, _, _, ok := ParseContextLengthError("connection reset by peer"); ok {
		t.Fatal("another error is not a context refusal")
	}
}
