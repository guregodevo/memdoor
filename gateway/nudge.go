package gateway

import "memdoor/pkg/llm"

// nudged is conv with a retry's nudge at its end: a text block on the last
// user message (the tool results, or the request), or a user message of its
// own after an assistant one. The system prompt stays as it was, so the
// prompt the model already read is a prefix of this one: a nudge written
// into the system prompt changed its first tokens, and an engine with a
// prefix cache read the whole conversation twice, for the nudged call and
// for the next (janis, Qwen3.5-9B, 2026-11-24: 8,676 tokens with 0 reused,
// then 9,099 with 113).
func nudged(conv []llm.MessageParam, nudge string) []llm.MessageParam {
	if nudge == "" {
		return conv
	}
	n := len(conv)
	if n == 0 || conv[n-1].Role != llm.MessageParamRoleUser {
		return append(conv, llm.NewUserMessage(llm.NewTextBlock(nudge)))
	}
	last := conv[n-1]
	last.Content = append(append([]llm.ContentBlockParamUnion(nil), last.Content...), llm.NewTextBlock(nudge))
	out := append([]llm.MessageParam(nil), conv[:n-1]...)
	return append(out, last)
}
