package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/local/picobot/internal/providers"
)

const (
	// ReserveTokens is the reserved space when deciding to compact.
	ReserveTokens = 8_000
	// MinMessagesToCompact is the minimum message count before compaction is attempted.
	MinMessagesToCompact = 15
	// RecentMessagesToKeep is how many trailing messages to preserve.
	RecentMessagesToKeep = 12
)

const summarySystemPrompt = "You are a summarizer. Summarize the following conversation concisely. " +
	"Preserve: key facts, decisions, TODOs, open questions, and constraints. Output only the summary, no preamble."

// CompactIfNeeded checks if messages exceed the threshold and, if so, summarizes
// the older portion and returns compacted messages. On failure, returns the original
// messages unchanged (best-effort).
func CompactIfNeeded(ctx context.Context, messages []providers.Message, contextWindowTokens int, provider providers.LLMProvider, model string) ([]providers.Message, error) {
	if contextWindowTokens <= 0 {
		contextWindowTokens = DefaultContextWindowTokens
	}
	threshold := contextWindowTokens - ReserveTokens
	if EstimateTokens(messages) <= threshold {
		return messages, nil
	}
	if len(messages) < MinMessagesToCompact {
		return messages, nil
	}

	// Split: system prefix (leading system messages), then conversation
	systemEnd := 0
	for i, m := range messages {
		if m.Role != "system" {
			systemEnd = i
			break
		}
		systemEnd = i + 1
	}
	systemPrefix := messages[:systemEnd]
	conversation := messages[systemEnd:]
	if len(conversation) < MinMessagesToCompact {
		return messages, nil
	}

	// Keep last RecentMessagesToKeep
	keepCount := RecentMessagesToKeep
	if keepCount > len(conversation) {
		keepCount = len(conversation) / 2
	}
	if keepCount < 2 {
		return messages, nil
	}
	recent := conversation[len(conversation)-keepCount:]
	toSummarize := conversation[:len(conversation)-keepCount]
	if len(toSummarize) < 4 {
		return messages, nil
	}

	// Build conversation text for summarization
	var sb strings.Builder
	for _, m := range toSummarize {
		role := m.Role
		content := providers.ContentToString(m.Content)
		if content == "" {
			continue
		}
		sb.WriteString(role)
		sb.WriteString(": ")
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}
	convText := strings.TrimSpace(sb.String())
	if len(convText) > 50000 {
		convText = convText[:50000] + "\n\n[... truncated for summarization ...]"
	}

	summaryMsgs := []providers.Message{
		{Role: "system", Content: summarySystemPrompt},
		{Role: "user", Content: convText},
	}
	resp, err := provider.Chat(ctx, summaryMsgs, nil, model)
	if err != nil {
		log.Printf("compaction summarization failed: %v", err)
		return messages, nil
	}
	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		summary = "No prior history."
	}

	// Rebuild: system prefix + summary + recent
	result := make([]providers.Message, 0, len(systemPrefix)+2+len(recent))
	result = append(result, systemPrefix...)
	result = append(result, providers.Message{
		Role:    "system",
		Content: fmt.Sprintf("Previous conversation summary:\n\n%s", summary),
	})
	result = append(result, recent...)
	return result, nil
}
