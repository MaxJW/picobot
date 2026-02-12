package agent

import (
	"strings"
	"testing"

	"github.com/local/picobot/internal/agent/memory"
	"github.com/local/picobot/internal/providers"
)

func TestBuildMessagesIncludesMemories(t *testing.T) {
	cb := NewContextBuilder(".", memory.NewSimpleRanker(), 5)
	history := []string{"user: hi"}
	mems := []memory.MemoryItem{{Kind: "short", Text: "remember this"}, {Kind: "long", Text: "big fact"}}
	memCtx := "Long-term memory: important fact"
	msgs := cb.BuildMessages(history, "hello", nil, "telegram", "123", memCtx, mems)

	// Expect at least system prompt + some system messages + user history + current
	if len(msgs) < 4 {
		t.Fatalf("expected at least 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("expected first message to be system prompt, got %s", msgs[0].Role)
	}
	// find a system message containing the memory context
	foundMemCtx := false
		foundSummary := false
		for _, m := range msgs {
			c := providers.ContentToString(m.Content)
			if m.Role == "system" && strings.Contains(c, "Long-term memory: important fact") {
				foundMemCtx = true
			}
			if m.Role == "system" && strings.Contains(c, "remember this") && strings.Contains(c, "big fact") {
				foundSummary = true
			}
		}
	if !foundMemCtx {
		t.Fatalf("expected memory context system message to be present in messages: %v", msgs)
	}
	if !foundSummary {
		t.Fatalf("expected memory summary to be present in messages: %v", msgs)
	}
}
