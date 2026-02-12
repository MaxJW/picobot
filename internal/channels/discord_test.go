package channels

import "testing"

func TestSplitContent(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   []string
	}{
		{"empty", "", 2000, []string{""}},
		{"short", "hello", 2000, []string{"hello"}},
		{"exact", string(make([]rune, 2000)), 2000, []string{string(make([]rune, 2000))}},
		{"split two", string(make([]rune, 2500)), 2000, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitContent(tt.s, tt.maxLen)
			if tt.want == nil {
				if len(got) != 2 {
					t.Errorf("splitContent(2500 runes) = %d chunks, want 2", len(got))
				}
				if len([]rune(got[0])) != 2000 || len([]rune(got[1])) != 500 {
					t.Errorf("chunk lengths wrong: %d, %d", len([]rune(got[0])), len([]rune(got[1])))
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("splitContent() = %d chunks, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("chunk[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
