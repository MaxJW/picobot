package agent

import "github.com/local/picobot/internal/providers"

// CharsPerToken is the heuristic for token estimation (~4 chars per token for English).
const CharsPerToken = 4

// EstimateTokens returns an approximate token count for a slice of messages.
// Uses ~4 chars per token heuristic; content from tool_calls adds overhead.
func EstimateTokens(messages []providers.Message) int {
	var total int
	for _, m := range messages {
		total += messageTokens(m)
	}
	return total
}

func messageTokens(m providers.Message) int {
	n := len(providers.ContentToString(m.Content)) / CharsPerToken
	// Add overhead for tool_calls (schemas, IDs, args)
	if len(m.ToolCalls) > 0 {
		n += 100 * len(m.ToolCalls)
	}
	return n
}
