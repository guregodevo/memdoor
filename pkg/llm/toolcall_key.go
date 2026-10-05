package llm

import "encoding/json"

// ToolCallKey is what makes two calls in one reply "the same": the tool and
// its input by CONTENT, not by bytes. A reply cut at max_tokens and resumed
// can repeat a call with its fields in another order or other spacing;
// compared byte for byte, the repeat ran again (roadmap: a repeat overwrote
// a 4.4 MB output with a 48-byte file). Input that is not JSON is compared
// as it is. Compaction uses it too: a call made again supersedes the
// earlier one's result.
func ToolCallKey(name string, input []byte) string {
	var v any
	if json.Unmarshal(input, &v) == nil {
		if b, err := json.Marshal(v); err == nil {
			return name + "\x00" + string(b)
		}
	}
	return name + "\x00" + string(input)
}
