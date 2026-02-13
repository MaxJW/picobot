package agent

import (
	"strings"
	"testing"

	"github.com/local/picobot/internal/agent/memory"
	"github.com/local/picobot/internal/providers"
)

func TestParseHistoryItem(t *testing.T) {
	tests := []struct {
		in       string
		wantRole string
		wantCnt string
	}{
		{"user: hello", "user", "hello"},
		{"assistant: hi there", "assistant", "hi there"},
		{"user: ", "user", ""},
		{"assistant: multi\nline\ncontent", "assistant", "multi\nline\ncontent"},
		{"no-colon", "user", "no-colon"},
		{"", "user", ""},
	}
	for _, tt := range tests {
		role, content := parseHistoryItem(tt.in)
		if role != tt.wantRole || content != tt.wantCnt {
			t.Errorf("parseHistoryItem(%q) = (%q, %q), want (%q, %q)", tt.in, role, content, tt.wantRole, tt.wantCnt)
		}
	}
}

func TestBuildMessagesParsesHistoryRoles(t *testing.T) {
	cb := NewContextBuilder(".", nil, 5)
	history := []string{
		"user: what is 2+2?",
		"assistant: 2+2 equals 4",
		"user: thanks",
	}
	msgs := cb.BuildMessages(history, "bye", nil, "discord", "123", "", nil)

	// Find the history messages (after system block, before current user)
	var historyMsgs []providers.Message
	for i, m := range msgs {
		c := providers.ContentToString(m.Content)
		if c == "what is 2+2?" {
			if m.Role != "user" {
				t.Errorf("message %d: expected role user, got %s", i, m.Role)
			}
			historyMsgs = append(historyMsgs, m)
		} else if c == "2+2 equals 4" {
			if m.Role != "assistant" {
				t.Errorf("message %d: expected role assistant, got %s", i, m.Role)
			}
			historyMsgs = append(historyMsgs, m)
		} else if c == "thanks" {
			if m.Role != "user" {
				t.Errorf("message %d: expected role user, got %s", i, m.Role)
			}
			historyMsgs = append(historyMsgs, m)
		}
	}
	if len(historyMsgs) != 3 {
		t.Errorf("expected 3 history messages with correct roles, got %d", len(historyMsgs))
	}
}

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
