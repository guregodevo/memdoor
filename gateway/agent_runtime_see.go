package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memdoor/gateway/providers"
	"memdoor/pkg/llm"
	"memdoor/tools"
)

const (
	seeDescribe = "Describe this picture for a programmer who cannot see it: what it shows, its layout, " +
		"every piece of text in it word for word, and anything that looks wrong (errors, overlaps, cut-off or misaligned parts)."
	seeAnswer = "Look at this picture and answer, for a programmer who cannot see it: "
)

// runSee shows the picture a see call names to the turn's model in a one-off
// request and returns what it saw. It runs on the turn's context, so the look
// takes the turn's pin, its cancellation and its meter.
func (ar *AgentRuntime) runSee(ctx context.Context, input json.RawMessage) (string, error) {
	var in tools.SeeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("see: %w", err)
	}
	pic, err := tools.ReadPicture(in.Path, turnWorkdir(ctx))
	if err != nil {
		return "", err
	}
	if ar.clientFactory == nil {
		return "", fmt.Errorf("see: no model client")
	}
	agent, _ := ctx.Value("buddy_agent_name").(string)
	client, err := ar.clientFactory.GetClientFor(ctx, agent)
	if err != nil {
		return "", fmt.Errorf("see: %w", err)
	}
	model, _ := ar.answeringModel(ctx)
	ask := seeDescribe
	if q := strings.TrimSpace(in.Question); q != "" {
		ask = seeAnswer + q
	}
	ctx, cancel := context.WithTimeout(llm.WithUtilityCall(ctx), 3*time.Minute)
	defer cancel()
	resp, err := client.Messages().New(ctx, llm.MessageNewParams{
		Model: llm.Model(providers.ModelName),
		Messages: []llm.MessageParam{llm.NewUserMessage(
			llm.ContentBlockParamUnion{OfImage: &llm.ImageBlockParam{Type: "image", MediaType: pic.MediaType, Data: pic.Data}},
			llm.NewTextBlock(ask))},
		MaxTokens: maxOutputTokens(model),
		Agent:     agent,
	})
	if err != nil {
		return "", fmt.Errorf("see: %w", err)
	}
	var b strings.Builder
	for _, c := range resp.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "", fmt.Errorf("see: the model said nothing about %s; it may not read pictures", pic.Path)
	}
	return text, nil
}
