package context

import (
	"math"
	"regexp"
	"strconv"
	"sync/atomic"
)

// The estimator is chars/4, and the serving model's tokenizer is not.
//
// French prose and JSON tool output run nearer three characters a token
// than four, so an estimate of 57,000 was 61,444 on the server, past a
// 65,536 window: the shed ladder never fired because by our count the
// turn fit, and the turn died on the server's count instead (2026-09-18
// 14:36, the Mélenchon session). The server tells us the real count on
// every answer (usage.input_tokens) and, when it refuses, in the refusal.
// tokenScale is that ratio, learned as we go; every estimate is
// multiplied by it, so the ladder sees what the server will see.

// tokenScaleBits holds a float64: the factor real/estimate, never below 1.
var tokenScaleBits = atomicFloat(tokenScaleStart)

const (
	// tokenScaleStart is the factor before the first answer teaches one.
	// The first turn after a start read 23% low, and a start of 1.2 was
	// tried; the cause was the measurement, not the tokenizer — the coder's
	// system prompt was left out of every count. Counted in, chars/4 came
	// within 1% of the provider's count on GLM 5.3 Flash (2026-09-29).
	tokenScaleStart = 1.0
	tokenScaleMin   = 1.0
	tokenScaleMax   = 2.5
	// tokenScaleEMA is how much a new observation moves the factor: half,
	// so one odd request neither sticks nor is ignored.
	tokenScaleEMA = 0.5
)

func atomicFloat(f float64) *uint64 {
	b := math.Float64bits(f)
	return &b
}

// TokenScale is the current factor applied to every chars/4 estimate.
func TokenScale() float64 {
	return math.Float64frombits(atomic.LoadUint64(tokenScaleBits))
}

func setTokenScale(f float64) {
	if f < tokenScaleMin {
		f = tokenScaleMin
	}
	if f > tokenScaleMax {
		f = tokenScaleMax
	}
	atomic.StoreUint64(tokenScaleBits, math.Float64bits(f))
}

// CalibrateTokens learns from a request the server accepted: real is the
// server's input_tokens, estimate ours BEFORE scaling for the same request.
func CalibrateTokens(real, estimate int) {
	if real <= 0 || estimate <= 0 {
		return
	}
	r := float64(real) / float64(estimate)
	setTokenScale(TokenScale()*(1-tokenScaleEMA) + r*tokenScaleEMA)
}

// CalibrateTokensHard learns from a refusal: the server's count is the
// floor of the factor from now on, at once — the next attempt must fit.
func CalibrateTokensHard(real, estimate int) {
	if real <= 0 || estimate <= 0 {
		return
	}
	if r := float64(real) / float64(estimate); r > TokenScale() {
		setTokenScale(r)
	}
}

// scaledEstimate is chars/4 corrected by what the server has taught us.
func scaledEstimate(chars int) int {
	if chars <= 0 {
		return 0
	}
	return int(math.Ceil(float64((chars+3)/4) * TokenScale()))
}

// contextLengthErr reads a vLLM/OpenAI context refusal: "This model's
// maximum context length is 65536 tokens. However, you requested 4093
// output tokens and your prompt contains at least 61444 input tokens".
var contextLengthErr = regexp.MustCompile(`maximum context length is (\d+) tokens.*?requested (\d+) output tokens.*?at least (\d+) input tokens`)

// openRouterContextLengthErr is OpenRouter's wording of the same refusal:
// "This endpoint's maximum context length is 32768 tokens. However, you
// requested about 37623 tokens (21239 of text input, 16384 in the output)".
var openRouterContextLengthErr = regexp.MustCompile(`maximum context length is (\d+) tokens.*?\((\d+) of text input, (\d+) in the output\)`)

// ParseContextLengthError returns the window, the output asked for and the
// server's count of the prompt, when the error is that refusal.
func ParseContextLengthError(msg string) (window, output, input int, ok bool) {
	if m := contextLengthErr.FindStringSubmatch(msg); m != nil {
		window, _ = strconv.Atoi(m[1])
		output, _ = strconv.Atoi(m[2])
		input, _ = strconv.Atoi(m[3])
		return window, output, input, true
	}
	if m := openRouterContextLengthErr.FindStringSubmatch(msg); m != nil {
		window, _ = strconv.Atoi(m[1])
		input, _ = strconv.Atoi(m[2])
		output, _ = strconv.Atoi(m[3])
		return window, output, input, true
	}
	return 0, 0, 0, false
}
