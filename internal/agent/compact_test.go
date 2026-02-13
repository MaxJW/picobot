package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/local/picobot/internal/providers"
)

func TestEstimateTokens(t *testing.T) {
	msgs := []providers.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: strings.Repeat("x", 400)},
	}
	got := EstimateTokens(msgs)
	if got < 50 || got > 200 {
		t.Errorf("EstimateTokens = %d, expected rough 50-200", got)
	}
}

func TestCompactIfNeeded_NoOpWhenUnderThreshold(t *testing.T) {
	ctx := context.Background()
	msgs := []providers.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	provider := providers.NewStubProvider()
	got, err := CompactIfNeeded(ctx, msgs, 128_000, provider, "stub")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(msgs) {
		t.Errorf("expected no compaction (same len), got %d vs %d", len(got), len(msgs))
	}
}

func TestCompactIfNeeded_NoOpWhenTooFewMessages(t *testing.T) {
	ctx := context.Background()
	msgs := make([]providers.Message, 10)
	for i := range msgs {
		msgs[i] = providers.Message{Role: "user", Content: "msg"}
	}
	provider := providers.NewStubProvider()
	got, err := CompactIfNeeded(ctx, msgs, 100, provider, "stub")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(msgs) {
		t.Errorf("expected no compaction (too few), got %d vs %d", len(got), len(msgs))
	}
}
