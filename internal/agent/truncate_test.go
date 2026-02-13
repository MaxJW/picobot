package agent

import (
	"strings"
	"testing"
)

func TestCalculateMaxToolResultChars(t *testing.T) {
	tests := []struct {
		ctxTokens int
		want      int
	}{
		{128_000, 153600},  // 128k * 0.3 * 4 = 153600
		{200_000, 240000},  // 200k * 0.3 * 4 = 240000
		{2_000_000, 400_000}, // capped at HardMax
		{0, 153600},        // default 128k
	}
	for _, tt := range tests {
		got := CalculateMaxToolResultChars(tt.ctxTokens)
		if got != tt.want {
			t.Errorf("CalculateMaxToolResultChars(%d) = %d, want %d", tt.ctxTokens, got, tt.want)
		}
	}
}

func TestTruncateToolResult(t *testing.T) {
	suffix := truncationSuffix
	tests := []struct {
		text      string
		maxChars  int
		hasSuffix bool
	}{
		{"short", 100, false},
		{strings.Repeat("x", 5000), 1000, true},
		{strings.Repeat("a\n", 1000), 500, true},
	}
	for _, tt := range tests {
		got := TruncateToolResult(tt.text, tt.maxChars)
		if tt.hasSuffix && !strings.HasSuffix(got, suffix) {
			tail := got
			if len(got) > 80 {
				tail = got[len(got)-80:]
			}
			t.Errorf("TruncateToolResult expected to have suffix, got tail: %q", tail)
		}
		if !tt.hasSuffix && strings.HasSuffix(got, suffix) {
			t.Errorf("TruncateToolResult should not have suffix for short input")
		}
	}
}
