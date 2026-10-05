package tools

import "memdoor/pkg/llm"

// BuildToolUnionParams converts the runtime tool definitions into the
// Anthropic-SDK ToolUnionParam shape that gateway/providers consumes.
// The agent runtime calls this on the hot path (runInference) and the
// gateway's KV-cache warmup calls it to produce byte-identical tool
// schemas — drift between the two would silently make the warmup miss
// the cache for the tool-schema portion of the prompt prefix.
//
// Single transformation, two callers, one source of truth.
func BuildToolUnionParams(defs []ToolDefinition) []llm.ToolUnionParam {
	out := make([]llm.ToolUnionParam, 0, len(defs))
	for _, def := range defs {
		desc, schema := applyPatchParams(def)
		toolParam := llm.ToolParam{
			Name:        def.Name,
			Description: llm.String(desc),
			InputSchema: schema,
		}
		out = append(out, llm.ToolUnionParam{OfTool: &toolParam})
	}
	return out
}
