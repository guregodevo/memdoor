package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"memdoor/gateway/logs"
	sharedctx "memdoor/pkg/shared/context"
	"net"
	"net/http"
	"strings"
	"time"

	"memdoor/pkg/llm"
)

// oaiClient is the shared OpenAI-compatible HTTP client used by every
// OAI-shape provider in this package (OpenRouter, DeepSeek, Groq, a
// company's gateway, any provider added with memdoor connect). Per-provider differences
// live in their own files — base URL, factory, and any request-body
// hooks that overlay provider-specific fields onto the common request
// shape.
type oaiClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	// model is what this client serves; the request's model when the
	// caller names none.
	model string
	// provider is OpenRouter's routing policy for this turn, set only on the
	// BYOK path (providers/byok.go).
	provider map[string]any
	// attribute names this app to OpenRouter (byok.go): true when this client
	// talks to OpenRouter.
	attribute bool
	// vendor: a company's key (vendor.go) — the company's attribution
	// headers travel on every request, nothing OpenRouter-specific does.
	vendor bool
}

type oaiMessages struct {
	client *oaiClient
}

func (c *oaiClient) Messages() MessageService {
	return &oaiMessages{client: c}
}

func (m *oaiMessages) New(ctx context.Context, params llm.MessageNewParams) (*llm.Message, error) {
	messages := convertAnthropicToOAI(params)
	tools := convertToolsToOAI(params)

	// Inject system prompt as the first message (OpenAI format).
	// The Anthropic SDK puts it in params.System, not in messages.
	if len(params.System) > 0 {
		var systemText string
		for _, block := range params.System {
			if block.Text != "" {
				systemText += block.Text
			}
		}
		if systemText != "" {
			messages = append([]chatMessage{{Role: "system", Content: systemText}}, messages...)
		}
	}

	// Always stream — gives us time-to-first-token in the response
	// log AND lets opt-in callers receive per-delta callbacks via
	// streamCallbackFromContext for incremental UI updates. The
	// accumulator reconstructs the same chatResponse shape for
	// downstream code that hasn't been adapted to streaming.
	model := string(params.Model)
	if m.client.model != "" {
		model = m.client.model
	}
	reqBody := chatRequest{
		Model:         model,
		Messages:      messages,
		MaxTokens:     int(params.MaxTokens),
		Temperature:   params.Temperature,
		Tools:         tools,
		Stream:        true,
		StreamOptions: &chatStreamOptions{IncludeUsage: true},
	}
	if len(tools) > 0 {
		reqBody.ToolChoice = "auto"
		// A retry of a reply that announced work and did none must open on a
		// call (llm.WithForcedToolCall); the prefill remote.go uses for it
		// does not exist on this path, and "auto" let GLM 5.3 Flash announce
		// twice and end the turn (2026-09-29, "update docs").
		if llm.ForcedToolCallFromContext(ctx) {
			reqBody.ToolChoice = "required"
		}
	}
	reqBody.Provider = m.client.provider
	if m.client.attribute {
		reqBody.Provider = withStickyHost(m.client.provider, model) // byok_hosts.go
		reqBody.Reasoning = reasoningFor(ctx, model)                // reasoning.go
	}

	if m.client.vendor && strings.Contains(m.client.baseURL, "api.openai.com") {
		// OpenAI's own host: the newer name from the start.
		reqBody.MaxCompletionTokens, reqBody.MaxTokens = reqBody.MaxTokens, 0
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the model actually serving this call. This client is only built
	// for a named remote model (model_providers.go), so that is the model
	// asked for — not the placeholder label.
	modelLabel := reqBody.Model

	// Per-request body-shape log so we can see on-the-wire size.
	// Helps diagnose "is the system prompt actually being sent at
	// full length" without packet capture.
	logLLMReqShape(llmReqShape{
		Caller:          "agent",
		Model:           modelLabel,
		BodyBytes:       len(jsonData),
		MessageCount:    len(messages),
		ToolCount:       len(tools),
		SystemPromptLen: chatSystemPromptLen(messages),
	})

	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Function.Name
	}
	// Central LLM log. logLLMRequest emits both a grep-friendly message
	// string and structured slog attrs, so `memdoor logs query --regex LLM`
	// finds every call in one search.
	logLLMRequest(llmReqInfo{
		Caller:         "agent",
		Agent:          params.Agent,
		Model:          modelLabel,
		MessageCount:   len(messages),
		ToolCount:      len(tools),
		ToolNames:      toolNames,
		SystemHead:     chatMessageHead(messages, "system", 1200),
		UserHead:       chatMessageHead(messages, "user", 600),
		SystemPromptLn: chatSystemPromptLen(messages),
		UserMessageLn:  chatLastUserMessageLen(messages),
	})

	// THE PROVIDER'S FAILURE IS NOT AN ANSWER. A 429/5xx, or a stream the
	// upstream ends with an error object (finish_reason "error"), is tried
	// again after a pause while nothing has streamed to the person; once
	// text has streamed, or the retries are spent, it is an error the turn
	// reports. Before this, a mid-stream error came back as an empty
	// finished reply and the turn ended as "succeeded" with nothing said
	// (2026-09-26, DeepSeek at Fireworks, 63 tokens, cost 0).
	var (
		acc          *streamAccumulator
		consumeErr   error
		requestStart time.Time
	)
	cb := llm.StreamCallbackFromContext(ctx)
	shrunk := false
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST",
			m.client.baseURL+"/chat/completions",
			bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+m.client.apiKey)
		SetAppHeaders(req)
		if m.client.attribute {
			SetOpenRouterAttribution(req.Header)
		}
		if m.client.vendor {
			setVendorHeaders(req.Header)
			setAttributionHeaders(ctx, req.Header)
		}

		requestStart = time.Now()
		resp, err := m.client.httpClient.Do(req)
		if err != nil {
			if providerSilent(err) && ctx.Err() == nil {
				if attempt < len(oaiRetryAfter) {
					logs.New("Remote").Warn("provider sent no answer; asking again", slog.Duration("silent_for", oaiStallAfter),
						slog.Int("attempt", attempt+1), slog.String("model", modelLabel))
					llm.Notify(ctx, fmt.Sprintf("The model's provider sent nothing for %s — asking again.", oaiStallAfter))
					time.Sleep(oaiRetryAfter[attempt])
					continue
				}
				return nil, errProviderSilent()
			}
			return nil, fmt.Errorf("oai request failed: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if attempt < len(oaiRetryAfter) && ctx.Err() == nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
				logs.New("Remote").Warn("provider refused the request; retrying", slog.Int("status", resp.StatusCode),
					slog.Int("attempt", attempt+1), slog.String("model", modelLabel), slog.String("body", truncate(string(body), 200)))
				wait := retryAfter(resp.Header, oaiRetryAfter[attempt])
				llm.Notify(ctx, fmt.Sprintf("The model's provider refused the request (HTTP %d) — asking again in %s.", resp.StatusCode, wait.Round(time.Second)))
				time.Sleep(wait)
				continue
			}
			// A host that wants the cap under its newer name says so in a 400;
			// the request is sent once more with max_completion_tokens.
			if resp.StatusCode == http.StatusBadRequest && reqBody.MaxTokens > 0 && strings.Contains(string(body), "max_tokens") && strings.Contains(string(body), "max_completion_tokens") {
				reqBody.MaxCompletionTokens, reqBody.MaxTokens = reqBody.MaxTokens, 0
				if jsonData, err = json.Marshal(reqBody); err == nil {
					logs.New("Remote").Info("the host wants max_completion_tokens; asking again with it", slog.String("model", modelLabel))
					continue
				}
			}
			// A low balance refuses the RESERVATION, not the reply: OpenRouter
			// holds credit for the whole max_tokens, most replies are a few
			// hundred tokens, and the refusal says what the balance covers.
			// Ask once more within it rather than failing the turn.
			if afford := affordableTokens(resp.StatusCode, body); afford > 0 && !shrunk && afford < reqBody.MaxTokens {
				shrunk = true
				reqBody.MaxTokens = afford * 9 / 10
				if jsonData, err = json.Marshal(reqBody); err != nil {
					return nil, err
				}
				logs.New("Remote").Warn("balance covers fewer output tokens; asking again within it",
					slog.Int("max_tokens", reqBody.MaxTokens), slog.String("model", modelLabel))
				continue
			}
			return nil, providerRefusal(resp.StatusCode, body)
		}

		// Stream the response body via SSE. The accumulator reconstructs
		// the non-streaming chatResponse shape so the rest of this function
		// stays unchanged. A per-delta callback (if planted on ctx via
		// WithStreamCallback) fires for every text fragment — that's the
		// hook the agent runtime uses for AssistantStreamer broadcasts.
		acc = &streamAccumulator{}
		body := newIdleTimeoutReader(resp.Body, oaiStallAfter)
		consumeErr = acc.consume(body, cb, llm.StreamGuardFromContext(ctx))
		body.Close()
		if errors.Is(consumeErr, errStreamIdle) && ctx.Err() == nil {
			if acc.content.Len() == 0 && len(acc.toolCalls) == 0 && attempt < len(oaiRetryAfter) {
				logs.New("Remote").Warn("provider went silent before any text; asking again", slog.Duration("silent_for", oaiStallAfter),
					slog.Int("attempt", attempt+1), slog.String("model", modelLabel))
				llm.Notify(ctx, fmt.Sprintf("The model's provider went silent for %s — asking again.", oaiStallAfter))
				time.Sleep(oaiRetryAfter[attempt])
				continue
			}
			consumeErr = errProviderSilent()
		}
		// The connection dropped mid-stream (a reset, an unexpected EOF) with
		// nothing shown yet: ask again, as for silence and provider errors. A
		// half-received tool call is not shown and cannot run. Live
		// 2026-09-30: "read: connection reset by peer" ended a coder turn.
		if consumeErr != nil && !errors.Is(consumeErr, errStreamIdle) && acc.content.Len() == 0 &&
			attempt < len(oaiRetryAfter) && ctx.Err() == nil {
			logs.New("Remote").Warn("connection dropped before any text; asking again",
				slog.Int("attempt", attempt+1), slog.String("model", modelLabel), slog.String("error", truncate(consumeErr.Error(), 200)))
			llm.Notify(ctx, "The connection to the model's provider dropped — asking again.")
			time.Sleep(oaiRetryAfter[attempt])
			continue
		}
		if acc.failed() {
			if acc.content.Len() == 0 && attempt < len(oaiRetryAfter) && ctx.Err() == nil {
				logs.New("Remote").Warn("provider failed mid-stream before any text; retrying",
					slog.Int("attempt", attempt+1), slog.String("model", modelLabel), slog.String("error", truncate(acc.errMsg, 200)))
				llm.Notify(ctx, "The model's provider failed before answering — asking again.")
				time.Sleep(oaiRetryAfter[attempt])
				continue
			}
			why := acc.errMsg
			if why == "" {
				why = "the provider ended the answer with an error"
			}
			return nil, fmt.Errorf("the model's provider failed mid-answer: %s (%d characters had streamed)", why, acc.content.Len())
		}
		break
	}
	if consumeErr != nil {
		// Salvage path: when the ctx fires (or the connection drops)
		// mid-stream, we still have everything the accumulator has
		// already written. If that's pure text — no pending tool_calls
		// the caller would have to dispatch — finalize it as a normal
		// (truncated) response instead of returning an error. The user
		// sees what the model actually said up to the cut-off; the
		// alternative is the "AI service is experiencing high load"
		// template covering up real text the model already produced.
		//
		// Tool_calls are explicitly excluded because they're not
		// resumable — half-written JSON arguments can't be executed,
		// and pretending the call completed would dispatch a malformed
		// tool invocation. Same reasoning for the usage block: it only
		// arrives on the FINAL chunk, so a truncated response carries
		// zero tokens; callers that watch CompletionTokens will see
		// the truncation that way too.
		if acc.content.Len() == 0 || len(acc.toolCalls) > 0 {
			return nil, fmt.Errorf("stream consume: %w", consumeErr)
		}
		if acc.finishReason == "" {
			acc.finishReason = "stream_canceled"
		}
		if log := llmLog(); log != nil {
			log.Warn(fmt.Sprintf("LLM stream truncated by ctx caller=agent model=%s kept_chars=%d err=%s",
				reqBody.Model, acc.content.Len(), truncate(consumeErr.Error(), 200)))
		}
	}
	oaiResp := acc.build()
	// Time-to-first-token, measured at the OAI client layer because
	// that's where the SSE chunks actually arrive. Zero when the
	// stream produced no content (tool-call-only round, for example).
	var firstTokenMs float64
	if !acc.firstTokenAt.IsZero() {
		firstTokenMs = float64(acc.firstTokenAt.Sub(requestStart).Milliseconds())
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in oai response")
	}

	choice := oaiResp.Choices[0]
	// Reasoning models (DeepSeek R1/V4) sometimes leave content empty
	// and put the final answer in reasoning_content. Fall back to it so
	// the agent has something to post.
	if choice.Message.Content == "" && choice.Message.ReasoningContent != "" {
		choice.Message.Content = choice.Message.ReasoningContent
	}
	// Normalize cache-hit tokens across provider shapes: DeepSeek
	// flat field vs OpenAI nested under prompt_tokens_details.
	cacheHit := oaiResp.Usage.PromptCacheHitTokens
	if cacheHit == 0 && oaiResp.Usage.PromptTokensDetails != nil {
		cacheHit = oaiResp.Usage.PromptTokensDetails.CachedTokens
	}
	// The model that ANSWERED, not the one asked for: a provider may route
	// to another (found 2026-09-26: Opus turns metered as DeepSeek after a
	// pin was released).
	served := answeringModel(oaiResp.Model, modelLabel)
	info := llmRespInfo{
		Caller:          "agent",
		Agent:           params.Agent,
		Model:           served,
		FinishReason:    choice.FinishReason,
		InputTokens:     oaiResp.Usage.PromptTokens,
		OutputTokens:    oaiResp.Usage.CompletionTokens,
		TotalTokens:     oaiResp.Usage.TotalTokens,
		CacheHitTokens:  cacheHit,
		CacheMissTokens: oaiResp.Usage.PromptCacheMissTokens,
		ContentLen:      len(choice.Message.Content),
		ContentHead:     truncate(choice.Message.Content, 600),
		ToolCalls:       len(choice.Message.ToolCalls),
	}
	info.FirstTokenMs = firstTokenMs
	info.Upstream = oaiResp.Provider
	logLLMResponse(info)
	if m.client.attribute {
		go noteServedHost(m.client.apiKey, model, oaiResp.Provider)
	}
	// Token usage into the same ledger as every brain call (memdoor llm meter).
	RecordMeter(MeterEntry{
		TS:           time.Now(),
		Engine:       m.client.baseURL,
		Model:        served,
		InputTokens:  int64(oaiResp.Usage.PromptTokens),
		CachedTokens: int64(cacheHit),
		OutputTokens: int64(oaiResp.Usage.CompletionTokens),
		DurationMS:   time.Since(requestStart).Milliseconds(),
		User:         ctxString(ctx, sharedctx.ActorIDKey),
		Agent:        ctxString(ctx, sharedctx.AgentIDKey),
		Session:      ctxString(ctx, sharedctx.SessionIDKey),
		CostUSD:      oaiResp.Usage.Cost,
		GenID:        oaiResp.ID,
	})

	return convertOAIToAnthropicWithTools(choice, oaiResp.Usage, served, tools), nil
}

// messageHead / systemPromptLen / lastUserMessageLen are tiny helpers
// the request-side prompt logging uses to extract the system + user
// heads from the OAI-shape message slice. Defined here so the OAI
// client is self-contained.
func chatMessageHead(messages []chatMessage, role string, maxLen int) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == role {
			return truncate(messages[i].Content, maxLen)
		}
	}
	return ""
}

func chatSystemPromptLen(messages []chatMessage) int {
	for _, m := range messages {
		if m.Role == "system" {
			return len(m.Content)
		}
	}
	return 0
}

func chatLastUserMessageLen(messages []chatMessage) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return len(messages[i].Content)
		}
	}
	return 0
}

// oaiRetryAfter is the pause before each retry of a request the provider
// refused (429/5xx) or failed mid-stream before any text; tests shorten it.
var oaiRetryAfter = []time.Duration{2 * time.Second, 5 * time.Second}

// providerSilent is a request that got no headers within oaiStallAfter.
func providerSilent(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// errProviderSilent is an answer the provider never sent.
func errProviderSilent() error {
	return fmt.Errorf("the model's provider sent nothing for %s", oaiStallAfter)
}
