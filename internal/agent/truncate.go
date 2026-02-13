package agent

import "strings"

const (
	// DefaultContextWindowTokens is the default context window size for token estimation.
	DefaultContextWindowTokens = 128_000
	// MaxToolResultContextShare caps a single tool result at 30% of context window.
	MaxToolResultContextShare = 0.3
	// HardMaxToolResultChars is the absolute max chars for any tool result.
	HardMaxToolResultChars = 400_000
	// MinKeepChars is the minimum prefix to keep when truncating.
	MinKeepChars = 2_000
)

const truncationSuffix = "\n\n⚠️ [Content truncated — original was too large for the model's context window. " +
	"The content above is a partial view. If you need more, request specific sections or use " +
	"offset/limit parameters to read smaller chunks.]"

// CalculateMaxToolResultChars returns the max allowed characters for a tool result
// based on context window size. Uses ~4 chars per token heuristic.
func CalculateMaxToolResultChars(contextWindowTokens int) int {
	if contextWindowTokens <= 0 {
		contextWindowTokens = DefaultContextWindowTokens
	}
	maxTokens := int(float64(contextWindowTokens) * MaxToolResultContextShare)
	maxChars := maxTokens * 4
	if maxChars > HardMaxToolResultChars {
		return HardMaxToolResultChars
	}
	return maxChars
}

// TruncateToolResult truncates text to maxChars, preserving the beginning.
// Tries to break at a newline boundary.
func TruncateToolResult(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	keepChars := maxChars - len(truncationSuffix)
	if keepChars < MinKeepChars {
		keepChars = MinKeepChars
	}
	cutPoint := keepChars
	if lastNewline := strings.LastIndex(text[:keepChars], "\n"); lastNewline > keepChars*80/100 {
		cutPoint = lastNewline
	}
	return text[:cutPoint] + truncationSuffix
}
