package streaming

// FenceSpan represents a code fence block (```...```)
// Pattern: OpenClaw src/markdown/fences.ts
type FenceSpan struct {
	Start     int    // Start index in text
	End       int    // End index in text
	Indent    string // Leading whitespace
	Marker    string // Fence marker (``` or ~~~)
	OpenLine  string // Full opening line (e.g., "```python")
	CloseLine string // Full closing line (e.g., "```")
	Language  string // Language identifier (e.g., "python")
}
