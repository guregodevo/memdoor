package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// A PROVIDER'S REFUSAL IN ONE LINE (2026-09-28, TUI session). An account out
// of credit came back as the raw JSON body — the same message repeated ten
// times through previous_errors, with the account's key-management URL in it
// — printed into the conversation. The person needs what happened and what to
// do, once.

// providerRefusal turns a non-200 answer into the error the turn reports.
func providerRefusal(status int, body []byte) error {
	msg := refusalMessage(body)
	switch {
	case status == http.StatusPaymentRequired:
		return fmt.Errorf("OpenRouter refused the request: the account is out of credit for it (%s). "+
			"Add credit at openrouter.ai/settings/credits, or raise the key's limit", shortReason(msg))
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("OpenRouter refused the key (HTTP %d): %s. Check OPEN_ROUTER_API_KEY", status, shortReason(msg))
	default:
		return fmt.Errorf("the model provider refused the request (HTTP %d): %s", status, shortReason(msg))
	}
}

// refusalMessage is error.message from an OpenAI-style body, or the body.
func refusalMessage(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && strings.TrimSpace(e.Error.Message) != "" {
		return e.Error.Message
	}
	return strings.TrimSpace(string(body))
}

var urlInText = regexp.MustCompile(`https?://\S+`)

// shortReason keeps the sentence that says why, without links (they point at
// account pages), cut to a line. A context-length refusal keeps its second
// sentence too: it holds the provider's count of the prompt, which the turn
// learns from before it sheds and asks again (ctxmgmt.ParseContextLengthError).
func shortReason(msg string) string {
	msg = strings.TrimSpace(urlInText.ReplaceAllString(msg, ""))
	msg = strings.TrimRight(strings.TrimSuffix(strings.TrimSpace(msg), "To increase, visit"), " .,")
	if i := strings.Index(msg, " in the output)"); i > 0 && strings.Contains(msg[:i], "maximum context length") {
		return msg[:i+len(" in the output)")]
	}
	if i := strings.Index(msg, ". "); i > 0 && i < 200 {
		msg = msg[:i]
	}
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}

var affordRe = regexp.MustCompile(`can only afford (\d+)`)

// affordableTokens is how many output tokens a 402 says the balance covers,
// or 0 when the refusal is not about the reservation or leaves too little to
// answer with.
func affordableTokens(status int, body []byte) int {
	if status != http.StatusPaymentRequired {
		return 0
	}
	m := affordRe.FindSubmatch(body)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(string(m[1]))
	if n < 1024 {
		return 0
	}
	return n
}
