package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Minimal OpenRouter chat-completions client with tool calling and usage.
// Reads OPEN_ROUTER_API_KEY, like the gateway's BYOK path.

const openrouterURL = "https://openrouter.ai/api/v1/chat/completions"

type usage struct {
	Input, Output int
}

func chat(model string, turns []map[string]any, tools []toolSpec) (reply []map[string]any, u usage, err error) {
	payload := map[string]any{
		"model":    model,
		"messages": turns,
		"tools":    tools,
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", openrouterURL, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OPEN_ROUTER_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, u, err
	}
	defer resp.Body.Close()
	var body struct {
		Choices []struct {
			Message map[string]any `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct {
			Message  string         `json:"message"`
			Metadata map[string]any `json:"metadata"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, u, fmt.Errorf("decode: %w", err)
	}
	if body.Error != nil {
		return nil, u, fmt.Errorf("api: %s (%v)", body.Error.Message, body.Error.Metadata)
	}
	if len(body.Choices) == 0 {
		return nil, u, fmt.Errorf("no choices")
	}
	u.Input, u.Output = body.Usage.PromptTokens, body.Usage.CompletionTokens
	m := body.Choices[0].Message
	turns2 := map[string]any{"role": "assistant"}
	if c, ok := m["content"].(string); ok && c != "" {
		turns2["content"] = c
	}
	if tcs, ok := m["tool_calls"].([]any); ok {
		turns2["tool_calls"] = tcs
	}
	return []map[string]any{turns2}, u, nil
}

type toolCall struct {
	ID        string
	Name      string
	Arguments string
}

func lastAssistant(msgs []map[string]any) map[string]any {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["role"] == "assistant" {
			return msgs[i]
		}
	}
	return nil
}

func extractToolCalls(m map[string]any) []toolCall {
	if m == nil {
		return nil
	}
	raw, ok := m["tool_calls"].([]any)
	if !ok {
		return nil
	}
	var out []toolCall
	for _, r := range raw {
		tc, ok := r.(map[string]any)
		if !ok {
			continue
		}
		fn, _ := tc["function"].(map[string]any)
		name, _ := fn["name"].(string)
		args, _ := fn["arguments"].(string)
		id, _ := tc["id"].(string)
		if name != "" {
			out = append(out, toolCall{ID: id, Name: name, Arguments: args})
		}
	}
	return out
}
